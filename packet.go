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

type DataPacket struct {
	Timestamp  uint64
	Frame      uint32
	Components []IDataObject
}

type RtPacket struct {
	Type            PacketType
	ErrorResponse   string
	CommandResponse string
	XMLResponse     string
	Event           EventType
	Size            int
	Data            DataPacket
	File            FilePacket
}

func (p *RtPacket) Error() bool {
	return p.Type == PacketTypeError
}

func (p *RtPacket) IsPacketData() bool {
	return p.Type == PacketTypeData
}

func (p *RtPacket) EndOfData() bool {
	return p.Type == PacketTypeNoMoreData
}

type IDataObject interface {
	UnmarshalBinary([]byte) error
}

func (d DataPacket) getComponentObject(c ComponentType) IDataObject {
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

func (d *DataPacket) UnmarshalBinary(data []byte) error {
	d.Timestamp = binary.LittleEndian.Uint64(data[0:8])
	d.Frame = binary.LittleEndian.Uint32(data[8:12])
	numberOfComponents := binary.LittleEndian.Uint32(data[12:16])
	pos := uint32(16)
	for i := uint32(0); i < numberOfComponents; i++ {
		csize := binary.LittleEndian.Uint32(data[pos : pos+4])
		ctype := ComponentType(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		iobj := d.getComponentObject(ctype)
		if iobj == nil {
			return fmt.Errorf("datapacket unknown data object")
		}
		if err := iobj.UnmarshalBinary(data[pos+8:]); err != nil {
			return fmt.Errorf("datapacket unmarshalbinary: %w", err)
		}
		d.Components = append(d.Components, iobj)
		pos += csize
	}
	return nil
}

func trimStringResponse(data []byte) string {
	return string(bytes.Trim(data, "\x00"))
}

func (p *RtPacket) UnmarshalBinary(data []byte) error {
	p.Size = int(binary.LittleEndian.Uint32(data[0:4]))
	p.Type = PacketType(binary.LittleEndian.Uint32(data[4:8]))
	switch p.Type {
	case PacketTypeError:
		p.ErrorResponse = trimStringResponse(data[8:])
	case PacketTypeCommand:
		p.CommandResponse = trimStringResponse(data[8:])
	case PacketTypeXML:
		p.XMLResponse = trimStringResponse(data[8:])
	case PacketTypeData:
		return p.Data.UnmarshalBinary(data[8:])
	case PacketTypeNoMoreData:
		return nil
	case PacketTypeNone:
		return nil
	case PacketTypeC3DFile:
		return p.File.UnmarshalBinary(data[8:])
	case PacketTypeQTMFile:
		return p.File.UnmarshalBinary(data[8:])
	case PacketTypeEvent:
		p.Event = EventType(data[8])
	case PacketTypeDiscover:
		return nil
	default:
	}
	return nil
}
