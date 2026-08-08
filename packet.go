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

	// Skipped lists component types present in the frame that this SDK does
	// not know how to decode. A newer QTM may add components; recording them
	// rather than failing keeps older clients working against newer servers.
	Skipped []ComponentType

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

// Deprecated: use the package-level getComponentObject. Retained as a method
// for source compatibility.
func (d DataPacket) getComponentObject(c ComponentType) IDataObject {
	return getComponentObject(c)
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
// one's data. And an unrecognised component type is skipped and recorded rather
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
	d.Skipped = nil

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
		if iobj := getComponentObject(ctype); iobj != nil {
			if err := iobj.UnmarshalBinary(payload); err != nil {
				return fmt.Errorf("datapacket: component %d (%v): %w", i, ctype, err)
			}
			d.Components = append(d.Components, iobj)
		} else {
			d.Skipped = append(d.Skipped, ctype)
		}
		pos += csize
	}
	return nil
}

// Component returns the first decoded component of the requested type, or nil.
//
// Iterating Components and type-switching works too, but this covers the common
// case of "give me the 3D markers from this frame" without the boilerplate.
func (d *DataPacket) Component(c ComponentType) IDataObject {
	want := getComponentObject(c)
	if want == nil {
		return nil
	}
	wantType := fmt.Sprintf("%T", want)
	for _, obj := range d.Components {
		if fmt.Sprintf("%T", obj) == wantType {
			return obj
		}
	}
	return nil
}

// Markers3D returns the labelled 3D component of the frame, if present.
func (d *DataPacket) Markers3D() *packets.Component3D {
	if c, ok := d.Component(ComponentType3D).(*packets.Component3D); ok {
		return c
	}
	return nil
}

// Bodies6D returns the 6DOF matrix component of the frame, if present.
func (d *DataPacket) Bodies6D() *packets.Component6D {
	if c, ok := d.Component(ComponentType6D).(*packets.Component6D); ok {
		return c
	}
	return nil
}

// Skeletons returns the skeleton component of the frame, if present.
func (d *DataPacket) Skeletons() *packets.ComponentSkeleton {
	if c, ok := d.Component(ComponentTypeSkeleton).(*packets.ComponentSkeleton); ok {
		return c
	}
	return nil
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
