package packets

import "fmt"

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

func (e EyeTracker) String() string {
	return fmt.Sprintf("[samplenumber: %v samples: %v]", e.SampleNumber, e.Samples)
}

type ComponentEyeTracker struct {
	EyeTrackers []EyeTracker
}

func (c ComponentEyeTracker) String() string {
	return fmt.Sprintf("Eyetrackers: %v", c.EyeTrackers)
}

// UnmarshalBinary decodes eye tracker pupil diameters. As with gaze vectors, a
// device reporting zero samples omits the sample number field.
func (c *ComponentEyeTracker) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	deviceCount := cur.Uint32()
	if !cur.checkCount(deviceCount, 4, "eye tracker") {
		return cur.Err()
	}

	c.EyeTrackers = make([]EyeTracker, 0, deviceCount)
	for i := uint32(0); i < deviceCount; i++ {
		sampleCount := cur.Uint32()
		if sampleCount == 0 {
			c.EyeTrackers = append(c.EyeTrackers, EyeTracker{})
			continue
		}
		et := EyeTracker{SampleNumber: cur.Uint32()}
		if !cur.checkCount(sampleCount, 8, "eye tracker sample") {
			return cur.Err()
		}
		et.Samples = make([]EyeTrackerSample, sampleCount)
		for s := uint32(0); s < sampleCount; s++ {
			et.Samples[s] = EyeTrackerSample{
				LeftPupilDiameter:  cur.Float32(),
				RightPupilDiameter: cur.Float32(),
			}
		}
		c.EyeTrackers = append(c.EyeTrackers, et)
	}
	return cur.Err()
}
