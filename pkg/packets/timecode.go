package packets

import (
	"encoding/binary"
	"fmt"
)

//go:generate stringer -type TimecodeType -trimprefix TimecodeType
type TimecodeType uint32

const (
	TimecodeTypeSMPTE TimecodeType = iota
	TimecodeTypeIRIG
	TimecodeTypeCameraTime
)

type IrigTime struct {
	Year, Day, Hour, Minute, Second, Tenth uint32
}

func (i *IrigTime) Convert(high, low uint32) {
	i.Year = 0x7f & high
	i.Day = 0x1FF & (high >> 7)
	i.Hour = 0x1f & low
	i.Minute = 0x3F & (low >> 5)
	i.Second = 0x3F & (low >> 11)
	i.Tenth = 0xF & (low >> 17)
}

type SmpteTime struct {
	Hour, Minute, Second, Frame uint32
}

func (i *SmpteTime) Convert(high, low uint32) {
	i.Hour = 0x1f & low
	i.Minute = 0x3F & (low >> 5)
	i.Second = 0x3F & (low >> 11)
	i.Frame = 0x1F & (low >> 17)
}

type CameraTime uint64

func (i *CameraTime) Convert(high, low uint32) {
	*i = CameraTime((uint64(high) << uint64(32)) | uint64(low))
}

func (i *SmpteTime) String() string {
	return fmt.Sprintf("%02d:%02d:%02d:%02d", i.Hour, i.Minute, i.Second, i.Frame)
}

func (i *IrigTime) String() string {
	return fmt.Sprintf("%02d:%03d:%02d:%02d:%02d.%d", i.Year, i.Day, i.Hour, i.Minute, i.Second, i.Tenth)
}

func (i *CameraTime) String() string {
	const ticksPerSecond = 10000000
	seconds := (*i / ticksPerSecond)
	nanoseconds := ((*i % ticksPerSecond) * (1000000000 / ticksPerSecond))
	return fmt.Sprintf("%v.%v", seconds, nanoseconds)
}

type Timecode struct {
	Type       TimecodeType
	Irig       IrigTime
	Smpte      SmpteTime
	CameraTime CameraTime
}

func (c Timecode) String() string {
	switch c.Type {
	case TimecodeTypeIRIG:
		return c.Irig.String()
	case TimecodeTypeSMPTE:
		return c.Smpte.String()
	case TimecodeTypeCameraTime:
		return c.CameraTime.String()
	}
	return "unknown timecode"
}

type ComponentTimecode struct {
	Timecodes []Timecode
}

func (c ComponentTimecode) String() string {
	return fmt.Sprintf("Timecodes: %v", c.Timecodes)
}

func (c *ComponentTimecode) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfTimecodes := binary.LittleEndian.Uint32(data[0:4])
	c.Timecodes = make([]Timecode, numberOfTimecodes)
	for i := uint32(0); i < numberOfTimecodes; i++ {
		c.Timecodes[i].Type = TimecodeType(binary.LittleEndian.Uint32(data[4:8]))
		high := binary.LittleEndian.Uint32(data[8:12])
		low := binary.LittleEndian.Uint32(data[12:16])
		switch c.Timecodes[i].Type {
		case TimecodeTypeSMPTE:
			c.Timecodes[i].Smpte.Convert(high, low)
		case TimecodeTypeIRIG:
			c.Timecodes[i].Irig.Convert(high, low)
		case TimecodeTypeCameraTime:
			c.Timecodes[i].CameraTime.Convert(high, low)
		}
	}
	return nil
}
