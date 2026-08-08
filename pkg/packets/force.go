package packets

import "fmt"

type Point struct {
	X, Y, Z float32
}

type ForceSample struct {
	Force            Point
	Moment           Point
	CenterOfPressure Point
}

func (f ForceSample) String() string {
	return fmt.Sprintf("[Force X: %v Y: %v Z: %v][Moment X: %v Y: %v Z: %v][CoP X: %v Y: %v Z: %v]\n",
		f.Force.X, f.Force.Y, f.Force.Z,
		f.Moment.X, f.Moment.Y, f.Moment.Z,
		f.CenterOfPressure.X, f.CenterOfPressure.Y, f.CenterOfPressure.Z,
	)
}

type ForcePlate struct {
	ID      uint32
	Number  uint32
	Samples []ForceSample
}

func (f ForcePlate) String() string {
	return fmt.Sprintf("[id: %v nr: %v samples: %v]\n", f.ID, f.Number, f.Samples)
}

type ComponentForce struct {
	ForcePlates []ForcePlate
}

func (c ComponentForce) String() string {
	var s string
	for _, fp := range c.ForcePlates {
		s += fp.String()
	}
	return s
}

// forceSampleBytes is 9 float32: force, moment and centre of pressure.
const forceSampleBytes = 36

func (c *ComponentForce) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	plateCount := cur.Uint32()
	// 12 byte plate header: id, sample count, sample number.
	if !cur.checkCount(plateCount, 12, "force plate") {
		return cur.Err()
	}

	c.ForcePlates = make([]ForcePlate, 0, plateCount)
	for i := uint32(0); i < plateCount; i++ {
		fp := ForcePlate{ID: cur.Uint32()}
		sampleCount := cur.Uint32()
		fp.Number = cur.Uint32()
		if !cur.checkCount(sampleCount, forceSampleBytes, "force sample") {
			return cur.Err()
		}
		fp.Samples = make([]ForceSample, 0, sampleCount)
		for s := uint32(0); s < sampleCount; s++ {
			fp.Samples = append(fp.Samples, ForceSample{
				Force:            cur.Point(),
				Moment:           cur.Point(),
				CenterOfPressure: cur.Point(),
			})
		}
		c.ForcePlates = append(c.ForcePlates, fp)
	}
	return cur.Err()
}

type ComponentForceSingle ComponentForce

func (c ComponentForceSingle) String() string { return ComponentForce(c).String() }

func (c *ComponentForceSingle) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	plateCount := cur.Uint32()
	if !cur.checkCount(plateCount, 4+forceSampleBytes, "force plate") {
		return cur.Err()
	}

	c.ForcePlates = make([]ForcePlate, 0, plateCount)
	for i := uint32(0); i < plateCount; i++ {
		fp := ForcePlate{ID: cur.Uint32()}
		fp.Samples = []ForceSample{{
			Force:            cur.Point(),
			Moment:           cur.Point(),
			CenterOfPressure: cur.Point(),
		}}
		c.ForcePlates = append(c.ForcePlates, fp)
	}
	return cur.Err()
}
