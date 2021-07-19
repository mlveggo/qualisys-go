package packets

import (
	"encoding/binary"
	"fmt"
)

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
		s += fmt.Sprintf("%v", fp.String())
	}
	return s
}

func (c *ComponentForce) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfForcePlates := binary.LittleEndian.Uint32(data[0:4])
	pos := 4
	c.ForcePlates = make([]ForcePlate, 0, numberOfForcePlates)
	for m := uint32(0); m < numberOfForcePlates; m++ {
		fp := ForcePlate{}
		fp.ID = binary.LittleEndian.Uint32(data[pos : pos+4])
		samples := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		fp.Number = binary.LittleEndian.Uint32(data[pos+8 : pos+12])
		if samples > 0 {
			fp.Samples = make([]ForceSample, samples)
			for i := uint32(0); i < samples; i++ {
				fp.Samples[i].Force.X = Float32frombytes(data[pos+12 : pos+16])
				fp.Samples[i].Force.Y = Float32frombytes(data[pos+16 : pos+20])
				fp.Samples[i].Force.Z = Float32frombytes(data[pos+20 : pos+24])
				fp.Samples[i].Moment.X = Float32frombytes(data[pos+24 : pos+28])
				fp.Samples[i].Moment.Y = Float32frombytes(data[pos+28 : pos+32])
				fp.Samples[i].Moment.Z = Float32frombytes(data[pos+32 : pos+36])
				fp.Samples[i].CenterOfPressure.X = Float32frombytes(data[pos+36 : pos+40])
				fp.Samples[i].CenterOfPressure.Y = Float32frombytes(data[pos+40 : pos+44])
				fp.Samples[i].CenterOfPressure.Z = Float32frombytes(data[pos+44 : pos+48])
				pos += 36
			}
		}
		c.ForcePlates = append(c.ForcePlates, fp)
		pos += 12
	}
	return nil
}

type ComponentForceSingle ComponentForce

func (c ComponentForceSingle) String() string {
	return ComponentForce(c).String()
}

func (c *ComponentForceSingle) UnmarshalBinary(data []byte) error {
	numberOfForcePlates := binary.LittleEndian.Uint32(data[0:4])
	pos := 4
	c.ForcePlates = make([]ForcePlate, 0, numberOfForcePlates)
	for m := uint32(0); m < numberOfForcePlates; m++ {
		fp := ForcePlate{}
		fp.ID = binary.LittleEndian.Uint32(data[pos : pos+4])
		fp.Samples = make([]ForceSample, 1)
		fp.Samples[0].Force.X = Float32frombytes(data[pos+4 : pos+8])
		fp.Samples[0].Force.Y = Float32frombytes(data[pos+8 : pos+12])
		fp.Samples[0].Force.Z = Float32frombytes(data[pos+12 : pos+16])
		fp.Samples[0].Moment.X = Float32frombytes(data[pos+16 : pos+20])
		fp.Samples[0].Moment.Y = Float32frombytes(data[pos+20 : pos+24])
		fp.Samples[0].Moment.Z = Float32frombytes(data[pos+24 : pos+28])
		fp.Samples[0].CenterOfPressure.X = Float32frombytes(data[pos+28 : pos+32])
		fp.Samples[0].CenterOfPressure.Y = Float32frombytes(data[pos+32 : pos+36])
		fp.Samples[0].CenterOfPressure.Z = Float32frombytes(data[pos+36 : pos+40])
		pos += 36
		c.ForcePlates = append(c.ForcePlates, fp)
		pos += 4
	}
	return nil
}
