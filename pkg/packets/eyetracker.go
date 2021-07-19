package packets

import (
	"encoding/binary"
	"fmt"
)

type EyeTrackerSample struct {
	LeftPupilDiameter  float32
	RightPupilDiameter float32
}

func (m EyeTrackerSample) String() string {
	return fmt.Sprintf("[left:%v right:%v]", m.LeftPupilDiameter, m.RightPupilDiameter)
}

type EyeTracker struct {
	SampleNumber uint32
	Samples      []EyeTrackerSample
}

type ComponentEyeTracker struct {
	EyeTrackers []EyeTracker
}

func (c ComponentEyeTracker) String() string {
	return fmt.Sprintf("Eyetrackers: %v", c.EyeTrackers)
}

func (c *ComponentEyeTracker) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfEyeTrackers := binary.LittleEndian.Uint32(data[0:4])
	pos := 4
	c.EyeTrackers = make([]EyeTracker, 0, numberOfEyeTrackers)
	for m := uint32(0); m < numberOfEyeTrackers; m++ {
		samples := binary.LittleEndian.Uint32(data[pos : pos+4])
		if samples == 0 {
			pos += 4
			continue
		}
		sampleNumber := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		s := make([]EyeTrackerSample, samples)
		for i := uint32(0); i < samples; i++ {
			s[i].LeftPupilDiameter = Float32frombytes(data[pos+8 : pos+12])
			s[i].RightPupilDiameter = Float32frombytes(data[pos+12 : pos+16])
			pos += 8
		}
		c.EyeTrackers = append(c.EyeTrackers, EyeTracker{SampleNumber: sampleNumber, Samples: s})
		pos += 8
	}
	return nil
}
