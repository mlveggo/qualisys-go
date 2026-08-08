package packets

import "fmt"

type GazeVectorSample struct {
	X, Y, Z                         float32
	PositionX, PositionY, PositionZ float32
}

func (m GazeVectorSample) String() string {
	return fmt.Sprintf(
		"[x:%v y:%v z:%v px:%v py:%v pz:%v]",
		m.X, m.Y, m.Z, m.PositionX, m.PositionY, m.PositionZ,
	)
}

type GazeVector struct {
	SampleNumber uint32
	Samples      []GazeVectorSample
}

func (g GazeVector) String() string {
	return fmt.Sprintf("[samplenumber: %v samples: %v]", g.SampleNumber, g.Samples)
}

type ComponentGazeVector struct {
	GazeVectors []GazeVector
}

func (c ComponentGazeVector) String() string {
	return fmt.Sprintf("GazeVectors: %v", c.GazeVectors)
}

// UnmarshalBinary decodes gaze vectors.
//
// A device with zero samples omits the sample number field entirely, so the
// record is 4 bytes rather than 8. This matches the stride the C++ SDK uses:
// 4 + (sampleCount == 0 ? 0 : 4) + sampleCount*24.
func (c *ComponentGazeVector) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	deviceCount := cur.Uint32()
	if !cur.checkCount(deviceCount, 4, "gaze vector") {
		return cur.Err()
	}

	c.GazeVectors = make([]GazeVector, 0, deviceCount)
	for i := uint32(0); i < deviceCount; i++ {
		sampleCount := cur.Uint32()
		if sampleCount == 0 {
			c.GazeVectors = append(c.GazeVectors, GazeVector{})
			continue
		}
		gv := GazeVector{SampleNumber: cur.Uint32()}
		if !cur.checkCount(sampleCount, 24, "gaze vector sample") {
			return cur.Err()
		}
		gv.Samples = make([]GazeVectorSample, sampleCount)
		for s := uint32(0); s < sampleCount; s++ {
			gv.Samples[s] = GazeVectorSample{
				X: cur.Float32(), Y: cur.Float32(), Z: cur.Float32(),
				PositionX: cur.Float32(), PositionY: cur.Float32(), PositionZ: cur.Float32(),
			}
		}
		c.GazeVectors = append(c.GazeVectors, gv)
	}
	return cur.Err()
}
