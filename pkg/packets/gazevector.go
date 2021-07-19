package packets

import (
	"encoding/binary"
	"fmt"
)

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

type ComponentGazeVector struct {
	GazeVectors []GazeVector
}

func (c ComponentGazeVector) String() string {
	return fmt.Sprintf("GazeVectors: %v", c.GazeVectors)
}

func (c *ComponentGazeVector) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfGazeVector := binary.LittleEndian.Uint32(data[0:4])
	pos := 4
	c.GazeVectors = make([]GazeVector, 0, numberOfGazeVector)
	for m := uint32(0); m < numberOfGazeVector; m++ {
		samples := binary.LittleEndian.Uint32(data[pos : pos+4])
		if samples == 0 {
			pos += 4
			continue
		}
		sampleNumber := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		g := make([]GazeVectorSample, samples)
		for i := uint32(0); i < samples; i++ {
			g[i].X = Float32frombytes(data[pos+8 : pos+12])
			g[i].Y = Float32frombytes(data[pos+12 : pos+16])
			g[i].Z = Float32frombytes(data[pos+16 : pos+20])
			g[i].PositionX = Float32frombytes(data[pos+20 : pos+24])
			g[i].PositionY = Float32frombytes(data[pos+24 : pos+28])
			g[i].PositionZ = Float32frombytes(data[pos+28 : pos+32])
			pos += 24
		}
		c.GazeVectors = append(c.GazeVectors, GazeVector{SampleNumber: sampleNumber, Samples: g})
		pos += 8
	}
	return nil
}
