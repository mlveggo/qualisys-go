package packets

import "fmt"

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
	i.Year = 0x7F & high
	i.Day = 0x1FF & (high >> 7)
	i.Hour = 0x1F & low
	i.Minute = 0x3F & (low >> 5)
	i.Second = 0x3F & (low >> 11)
	i.Tenth = 0xF & (low >> 17)
}

func (i *IrigTime) String() string {
	return fmt.Sprintf("%02d:%03d:%02d:%02d:%02d.%d", i.Year, i.Day, i.Hour, i.Minute, i.Second, i.Tenth)
}

// SmpteTime is an SMPTE timecode. SubFrame counts camera frames within a single
// timecode frame and is what makes the timecode usable above the timecode
// frequency; it was previously dropped on the floor during decoding.
type SmpteTime struct {
	Hour, Minute, Second, Frame, SubFrame uint32
}

func (i *SmpteTime) Convert(_, low uint32) {
	i.Hour = 0x1F & low
	i.Minute = 0x3F & (low >> 5)
	i.Second = 0x3F & (low >> 11)
	i.Frame = 0x1F & (low >> 17)
	i.SubFrame = 0x1FF & (low >> 22)
}

func (i *SmpteTime) String() string {
	return fmt.Sprintf("%02d:%02d:%02d:%02d", i.Hour, i.Minute, i.Second, i.Frame)
}

// NormalizedSubFrame expresses SubFrame as a fraction of one timecode frame,
// given the camera capture frequency and the timecode frequency. It mirrors
// CRTProtocol::SMPTENormalizedSubFrame in the C++ SDK.
func (i *SmpteTime) NormalizedSubFrame(captureFrequency, timestampFrequency uint32) float64 {
	if captureFrequency == 0 || timestampFrequency == 0 || captureFrequency < timestampFrequency {
		return 0
	}
	subFramesPerFrame := captureFrequency / timestampFrequency
	return float64(i.SubFrame) / float64(subFramesPerFrame)
}

type CameraTime uint64

func (i *CameraTime) Convert(high, low uint32) {
	*i = CameraTime((uint64(high) << 32) | uint64(low))
}

func (i *CameraTime) String() string {
	const ticksPerSecond = 10000000
	seconds := *i / ticksPerSecond
	nanoseconds := (*i % ticksPerSecond) * (1000000000 / ticksPerSecond)
	return fmt.Sprintf("%v.%09v", seconds, nanoseconds)
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

// timecodeEntryBytes is type + high word + low word.
const timecodeEntryBytes = 12

// UnmarshalBinary decodes the timecode component.
//
// The previous implementation read every entry from the same three fixed
// offsets, so a stream carrying more than one timecode reported the first one
// repeated N times. Each entry is a fixed 12 bytes and the cursor advances
// through them.
func (c *ComponentTimecode) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	count := cur.Uint32()
	if !cur.checkCount(count, timecodeEntryBytes, "timecode") {
		return cur.Err()
	}

	c.Timecodes = make([]Timecode, count)
	for i := uint32(0); i < count; i++ {
		tc := &c.Timecodes[i]
		tc.Type = TimecodeType(cur.Uint32())
		high := cur.Uint32()
		low := cur.Uint32()
		switch tc.Type {
		case TimecodeTypeSMPTE:
			tc.Smpte.Convert(high, low)
		case TimecodeTypeIRIG:
			tc.Irig.Convert(high, low)
		case TimecodeTypeCameraTime:
			tc.CameraTime.Convert(high, low)
		}
	}
	return cur.Err()
}
