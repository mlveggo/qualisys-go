package qualisys

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/mlveggo/qualisys-go/pkg/packets"
)

//go:generate stringer -type EventType -trimprefix EventType
type EventType uint8

const (
	EventTypeConnected EventType = iota + 1
	EventTypeConnectionClosed
	EventTypeCaptureStarted
	EventTypeCaptureStopped
	EventTypeCaptureFetchingFinished // Not used in version 1.10 and later
	EventTypeCalibrationStarted
	EventTypeCalibrationStopped
	EventTypeRtFromFileStarted
	EventTypeRtFromFileStopped
	EventTypeWaitingForTrigger
	EventTypeCameraSettingsChanged
	EventTypeQTMShuttingDown
	EventTypeCaptureSaved
	EventTypeReprocessingStarted
	EventTypeReprocessingStopped
	EventTypeTrigger
	EventTypeNone
)

//go:generate stringer -type PacketType -trimprefix PacketType
type PacketType uint32

const (
	PacketTypeError PacketType = iota
	PacketTypeCommand
	PacketTypeXML
	PacketTypeData
	PacketTypeNoMoreData
	PacketTypeC3DFile
	PacketTypeEvent
	PacketTypeDiscover
	PacketTypeQTMFile
	PacketTypeNone
)

// DataPacket is the payload of a PacketTypeData packet.
type DataPacket struct {
	Timestamp  uint64
	Frame      uint32
	Components []IDataObject

	order binary.ByteOrder
}

type Packet struct {
	Type            PacketType
	ErrorResponse   string
	CommandResponse string
	XMLResponse     string
	Event           EventType
	Size            int
	Data            DataPacket
	File            FilePacket

	order binary.ByteOrder
}

func (p *Packet) Error() bool {
	return p.Type == PacketTypeError
}

func (p *Packet) IsPacketData() bool {
	return p.Type == PacketTypeData
}

func (p *Packet) EndOfData() bool {
	return p.Type == PacketTypeNoMoreData
}

// byteOrder returns the configured order, defaulting to little endian for
// zero-valued Packets built by callers or tests.
func (p *Packet) byteOrder() binary.ByteOrder {
	if p.order == nil {
		return binary.LittleEndian
	}
	return p.order
}

type IDataObject interface {
	UnmarshalBinary([]byte) error
}

func getComponentObject(c ComponentType) IDataObject {
	switch c {
	case ComponentType3D:
		return new(packets.Component3D)
	case ComponentType3DResidual:
		return new(packets.Component3DResidual)
	case ComponentType3DNoLabels:
		return new(packets.Component3DNoLabels)
	case ComponentType3DNoLabelsResidual:
		return new(packets.Component3DNoLabelsResidual)
	case ComponentType6D:
		return new(packets.Component6D)
	case ComponentType6DEuler:
		return new(packets.Component6DEuler)
	case ComponentType6DResidual:
		return new(packets.Component6DResidual)
	case ComponentType6DEulerResidual:
		return new(packets.Component6DEulerResidual)
	case ComponentType2D:
		return new(packets.Component2D)
	case ComponentType2DLinearized:
		return new(packets.Component2DLinearized)
	case ComponentTypeAnalog:
		return new(packets.ComponentAnalog)
	case ComponentTypeAnalogSingle:
		return new(packets.ComponentAnalogSingle)
	case ComponentTypeForce:
		return new(packets.ComponentForce)
	case ComponentTypeForceSingle:
		return new(packets.ComponentForceSingle)
	case ComponentTypeImage:
		return new(packets.ComponentImage)
	case ComponentTypeGazeVector:
		return new(packets.ComponentGazeVector)
	case ComponentTypeTimecode:
		return new(packets.ComponentTimecode)
	case ComponentTypeSkeleton:
		return new(packets.ComponentSkeleton)
	case ComponentTypeEyeTracker:
		return new(packets.ComponentEyeTracker)
	}
	return nil
}

// dataPacketHeaderSize is timestamp (8) + frame number (4) + component count (4).
const dataPacketHeaderSize = 16

// componentHeaderSize is the per-component size and type fields. The size
// field counts these 8 bytes as well as the component payload.
const componentHeaderSize = 8

// UnmarshalBinary decodes a data frame.
//
// Three things changed from the original implementation. Every offset is now
// bounds checked, so a truncated frame is an error rather than a panic. Each
// component parser receives exactly its own slice instead of everything to the
// end of the buffer, so one component's parser cannot wander into the next
// one's data. And an unrecognized component type is skipped and recorded rather
// than aborting the whole frame, which is what lets a client built against one
// protocol version keep working when QTM starts sending a component it has
// never heard of.
func (d *DataPacket) UnmarshalBinary(data []byte) error {
	if len(data) < dataPacketHeaderSize {
		return fmt.Errorf("datapacket: need %d header bytes, have %d", dataPacketHeaderSize, len(data))
	}
	order := d.order
	if order == nil {
		order = binary.LittleEndian
	}

	d.Timestamp = order.Uint64(data[0:8])
	d.Frame = order.Uint32(data[8:12])
	componentCount := order.Uint32(data[12:16])

	d.Components = nil

	pos := dataPacketHeaderSize
	for i := uint32(0); i < componentCount; i++ {
		if pos+componentHeaderSize > len(data) {
			return fmt.Errorf("datapacket: component %d header runs past end of packet", i)
		}
		csize := int(order.Uint32(data[pos : pos+4]))
		ctype := ComponentType(order.Uint32(data[pos+4 : pos+8]))

		// A zero or undersized component length would loop forever.
		if csize < componentHeaderSize {
			return fmt.Errorf("datapacket: component %d has invalid size %d", i, csize)
		}
		if pos+csize > len(data) {
			return fmt.Errorf("datapacket: component %d claims %d bytes, only %d remain",
				i, csize, len(data)-pos)
		}

		payload := data[pos+componentHeaderSize : pos+csize]
		iobj := getComponentObject(ctype)
		if iobj == nil {
			// Preserve the raw bytes rather than discarding them, so a caller
			// that does understand a newer component can still decode it.
			iobj = &UnknownComponent{Type: ctype}
		}
		if err := iobj.UnmarshalBinary(payload); err != nil {
			return fmt.Errorf("datapacket: component %d (%v): %w", i, ctype, err)
		}
		d.Components = append(d.Components, iobj)
		pos += csize
	}
	return nil
}

// UnknownComponent holds a component this SDK cannot decode.
//
// Newer QTM releases add components. Keeping the payload instead of failing the
// frame means a client built against one protocol version keeps working, and
// leaves the door open for a caller that does know the format.
type UnknownComponent struct {
	Type ComponentType
	Data []byte
}

func (u *UnknownComponent) UnmarshalBinary(data []byte) error {
	u.Data = make([]byte, len(data))
	copy(u.Data, data)
	return nil
}

func (u UnknownComponent) String() string {
	return fmt.Sprintf("UnknownComponent{type: %d, %d bytes}", int(u.Type), len(u.Data))
}

// componentTypeOf reports the component type an already-decoded object came
// from, and whether it was recognized.
func componentTypeOf(obj IDataObject) (ComponentType, bool) {
	switch v := obj.(type) {
	case *packets.Component3D:
		return ComponentType3D, true
	case *packets.Component3DResidual:
		return ComponentType3DResidual, true
	case *packets.Component3DNoLabels:
		return ComponentType3DNoLabels, true
	case *packets.Component3DNoLabelsResidual:
		return ComponentType3DNoLabelsResidual, true
	case *packets.Component6D:
		return ComponentType6D, true
	case *packets.Component6DResidual:
		return ComponentType6DResidual, true
	case *packets.Component6DEuler:
		return ComponentType6DEuler, true
	case *packets.Component6DEulerResidual:
		return ComponentType6DEulerResidual, true
	case *packets.Component2D:
		return ComponentType2D, true
	case *packets.Component2DLinearized:
		return ComponentType2DLinearized, true
	case *packets.ComponentAnalog:
		return ComponentTypeAnalog, true
	case *packets.ComponentAnalogSingle:
		return ComponentTypeAnalogSingle, true
	case *packets.ComponentForce:
		return ComponentTypeForce, true
	case *packets.ComponentForceSingle:
		return ComponentTypeForceSingle, true
	case *packets.ComponentImage:
		return ComponentTypeImage, true
	case *packets.ComponentGazeVector:
		return ComponentTypeGazeVector, true
	case *packets.ComponentTimecode:
		return ComponentTypeTimecode, true
	case *packets.ComponentSkeleton:
		return ComponentTypeSkeleton, true
	case *packets.ComponentEyeTracker:
		return ComponentTypeEyeTracker, true
	case *UnknownComponent:
		return v.Type, false
	}
	return 0, false
}

// Component returns the first decoded component of the requested type, or nil.
//
// Iterating Components and type-switching works too, but this covers the common
// case of "give me the 3D markers from this frame" without the boilerplate.
//
// Only components this SDK could decode are reachable here. A component type it
// has never heard of is held as an UnknownComponent and is never returned by
// this method even when its type matches; use UnknownComponentData for those.
func (d *DataPacket) Component(c ComponentType) IDataObject {
	for _, obj := range d.Components {
		if ctype, known := componentTypeOf(obj); known && ctype == c {
			return obj
		}
	}
	return nil
}

// UnknownComponentData returns the raw payload of the undecodable component of
// type c, or nil if the frame carries no such component.
//
// UnknownComponentTypes says which types arrived that this SDK could not
// decode; this hands over the bytes for one of them, so a caller who does know
// a newer component's format can decode it. Component cannot serve that role:
// it deliberately only returns components this SDK decoded itself.
func (d *DataPacket) UnknownComponentData(c ComponentType) []byte {
	for _, obj := range d.Components {
		if u, ok := obj.(*UnknownComponent); ok && u.Type == c {
			return u.Data
		}
	}
	return nil
}

// UnknownComponentTypes lists component types present in the frame that this
// SDK could not decode.
func (d *DataPacket) UnknownComponentTypes() []ComponentType {
	var out []ComponentType
	for _, obj := range d.Components {
		if u, ok := obj.(*UnknownComponent); ok {
			out = append(out, u.Type)
		}
	}
	return out
}

// Markers3D returns the labeled 3D component of the frame, if present.
func (d *DataPacket) Markers3D() *packets.Component3D {
	c, _ := d.Component(ComponentType3D).(*packets.Component3D)
	return c
}

// Markers3DResidual returns the labeled 3D component with residuals.
func (d *DataPacket) Markers3DResidual() *packets.Component3DResidual {
	c, _ := d.Component(ComponentType3DResidual).(*packets.Component3DResidual)
	return c
}

// Markers3DNoLabels returns the unlabeled 3D component.
func (d *DataPacket) Markers3DNoLabels() *packets.Component3DNoLabels {
	c, _ := d.Component(ComponentType3DNoLabels).(*packets.Component3DNoLabels)
	return c
}

// Markers3DNoLabelsResidual returns the unlabeled 3D component with residuals.
func (d *DataPacket) Markers3DNoLabelsResidual() *packets.Component3DNoLabelsResidual {
	c, _ := d.Component(ComponentType3DNoLabelsResidual).(*packets.Component3DNoLabelsResidual)
	return c
}

// Bodies6D returns the 6DOF matrix component of the frame, if present.
func (d *DataPacket) Bodies6D() *packets.Component6D {
	c, _ := d.Component(ComponentType6D).(*packets.Component6D)
	return c
}

// Bodies6DResidual returns the 6DOF matrix component with residuals.
func (d *DataPacket) Bodies6DResidual() *packets.Component6DResidual {
	c, _ := d.Component(ComponentType6DResidual).(*packets.Component6DResidual)
	return c
}

// Bodies6DEuler returns the 6DOF Euler component of the frame, if present.
func (d *DataPacket) Bodies6DEuler() *packets.Component6DEuler {
	c, _ := d.Component(ComponentType6DEuler).(*packets.Component6DEuler)
	return c
}

// Bodies6DEulerResidual returns the 6DOF Euler component with residuals.
func (d *DataPacket) Bodies6DEulerResidual() *packets.Component6DEulerResidual {
	c, _ := d.Component(ComponentType6DEulerResidual).(*packets.Component6DEulerResidual)
	return c
}

// Markers2D returns the per-camera 2D component of the frame, if present.
func (d *DataPacket) Markers2D() *packets.Component2D {
	c, _ := d.Component(ComponentType2D).(*packets.Component2D)
	return c
}

// Markers2DLinearized returns the linearized per-camera 2D component.
func (d *DataPacket) Markers2DLinearized() *packets.Component2DLinearized {
	c, _ := d.Component(ComponentType2DLinearized).(*packets.Component2DLinearized)
	return c
}

// Analog returns the multi-sample analog component, if present.
func (d *DataPacket) Analog() *packets.ComponentAnalog {
	c, _ := d.Component(ComponentTypeAnalog).(*packets.ComponentAnalog)
	return c
}

// AnalogSingle returns the single-sample analog component, if present.
func (d *DataPacket) AnalogSingle() *packets.ComponentAnalogSingle {
	c, _ := d.Component(ComponentTypeAnalogSingle).(*packets.ComponentAnalogSingle)
	return c
}

// Force returns the multi-sample force plate component, if present.
func (d *DataPacket) Force() *packets.ComponentForce {
	c, _ := d.Component(ComponentTypeForce).(*packets.ComponentForce)
	return c
}

// ForceSingle returns the single-sample force plate component, if present.
func (d *DataPacket) ForceSingle() *packets.ComponentForceSingle {
	c, _ := d.Component(ComponentTypeForceSingle).(*packets.ComponentForceSingle)
	return c
}

// Images returns the camera image component, if present.
func (d *DataPacket) Images() *packets.ComponentImage {
	c, _ := d.Component(ComponentTypeImage).(*packets.ComponentImage)
	return c
}

// GazeVectors returns the gaze vector component, if present.
func (d *DataPacket) GazeVectors() *packets.ComponentGazeVector {
	c, _ := d.Component(ComponentTypeGazeVector).(*packets.ComponentGazeVector)
	return c
}

// EyeTrackers returns the eye tracker component, if present.
func (d *DataPacket) EyeTrackers() *packets.ComponentEyeTracker {
	c, _ := d.Component(ComponentTypeEyeTracker).(*packets.ComponentEyeTracker)
	return c
}

// Timecodes returns the timecode component, if present.
func (d *DataPacket) Timecodes() *packets.ComponentTimecode {
	c, _ := d.Component(ComponentTypeTimecode).(*packets.ComponentTimecode)
	return c
}

// Skeletons returns the skeleton component, if present.
func (d *DataPacket) Skeletons() *packets.ComponentSkeleton {
	c, _ := d.Component(ComponentTypeSkeleton).(*packets.ComponentSkeleton)
	return c
}

func trimStringResponse(data []byte) string {
	return string(bytes.Trim(data, "\x00"))
}

// UnmarshalBinary decodes a complete packet including its 8 byte header.
func (p *Packet) UnmarshalBinary(data []byte) error {
	if len(data) < packetHeaderSize {
		return fmt.Errorf("packet: need %d header bytes, have %d", packetHeaderSize, len(data))
	}
	order := p.byteOrder()
	p.Size = int(order.Uint32(data[0:4]))
	p.Type = PacketType(order.Uint32(data[4:8]))

	payload := data[packetHeaderSize:]

	switch p.Type {
	case PacketTypeError:
		p.ErrorResponse = trimStringResponse(payload)
	case PacketTypeCommand:
		p.CommandResponse = trimStringResponse(payload)
	case PacketTypeXML:
		p.XMLResponse = trimStringResponse(payload)
	case PacketTypeData:
		p.Data.order = order
		return p.Data.UnmarshalBinary(payload)
	case PacketTypeNoMoreData, PacketTypeNone, PacketTypeDiscover:
		return nil
	case PacketTypeC3DFile, PacketTypeQTMFile:
		p.File.Type = FileType(p.Type)
		return p.File.UnmarshalBinary(payload)
	case PacketTypeEvent:
		if len(payload) < 1 {
			return fmt.Errorf("packet: event packet has no payload")
		}
		p.Event = EventType(payload[0])
	}
	return nil
}
