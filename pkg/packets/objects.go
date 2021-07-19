package packets

import (
	"encoding/binary"
	"fmt"
)

type BodyMatrix struct {
	Point    Point
	Residual float32
	Rotation [9]float32
}

func (b BodyMatrix) String() string {
	return fmt.Sprintf(
		"x:%v y:%v z:%v r:%v [[%v %v %v][%v %v %v][%v %v %v]]",
		b.Point.X, b.Point.Y, b.Point.Z, b.Residual,
		b.Rotation[0], b.Rotation[1], b.Rotation[2],
		b.Rotation[3], b.Rotation[4], b.Rotation[5],
		b.Rotation[6], b.Rotation[7], b.Rotation[8],
	)
}

type Component6D struct {
	Droprate      uint16
	OutOfSyncRate uint16
	Bodies        []BodyMatrix
}

func (c Component6D) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Bodies: %v\n", c.Droprate, c.OutOfSyncRate, c.Bodies)
}

func (c *Component6D) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfBodies := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Bodies = make([]BodyMatrix, 0, numberOfBodies)
	for m := uint32(0); m < numberOfBodies; m++ {
		body := BodyMatrix{
			Point: Point{
				X: Float32frombytes(data[pos : pos+4]),
				Y: Float32frombytes(data[pos+4 : pos+8]),
				Z: Float32frombytes(data[pos+8 : pos+12]),
			},
		}
		for i := 0; i < 9; i++ {
			body.Rotation[i] = Float32frombytes(data[pos+12+i*4 : pos+16+i*4])
		}
		c.Bodies = append(c.Bodies, body)
		pos += 48
	}
	return nil
}

type Component6DResidual Component6D

func (c Component6DResidual) String() string {
	return Component6D(c).String()
}

func (c *Component6DResidual) UnmarshalBinary(data []byte) error {
	numberOfBodies := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Bodies = make([]BodyMatrix, 0, numberOfBodies)
	for m := uint32(0); m < numberOfBodies; m++ {
		body := BodyMatrix{
			Point: Point{
				X: Float32frombytes(data[pos : pos+4]),
				Y: Float32frombytes(data[pos+4 : pos+8]),
				Z: Float32frombytes(data[pos+8 : pos+12]),
			},
		}
		for i := 0; i < 9; i++ {
			body.Rotation[i] = Float32frombytes(data[pos+12+i*4 : pos+16+i*4])
		}
		body.Residual = Float32frombytes(data[pos+48 : pos+52])
		c.Bodies = append(c.Bodies, body)
		pos += 52
	}
	return nil
}

type BodyEuler struct {
	Point    Point
	Residual float32
	Angles   [3]float32
}

func (b BodyEuler) String() string {
	return fmt.Sprintf(
		"x:%v y:%v z:%v r:%v [%v %v %v]",
		b.Point.X, b.Point.Y, b.Point.Z, b.Residual,
		b.Angles[0], b.Angles[1], b.Angles[2],
	)
}

type Component6DEuler struct {
	Droprate      uint16
	OutOfSyncRate uint16
	Bodies        []BodyEuler
}

func (c Component6DEuler) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Bodies: %v\n", c.Droprate, c.OutOfSyncRate, c.Bodies)
}

func (c *Component6DEuler) UnmarshalBinary(data []byte) error {
	numberOfBodies := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Bodies = make([]BodyEuler, 0, numberOfBodies)
	for m := uint32(0); m < numberOfBodies; m++ {
		body := BodyEuler{
			Point: Point{
				X: Float32frombytes(data[pos : pos+4]),
				Y: Float32frombytes(data[pos+4 : pos+8]),
				Z: Float32frombytes(data[pos+8 : pos+12]),
			},
		}
		for i := 0; i < 3; i++ {
			body.Angles[i] = Float32frombytes(data[pos+12+i*4 : pos+16+i*4])
		}
		c.Bodies = append(c.Bodies, body)
		pos += 24
	}
	return nil
}

type Component6DEulerResidual Component6DEuler

func (c Component6DEulerResidual) String() string {
	return Component6DEuler(c).String()
}

func (c *Component6DEulerResidual) UnmarshalBinary(data []byte) error {
	numberOfBodies := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Bodies = make([]BodyEuler, 0, numberOfBodies)
	for m := uint32(0); m < numberOfBodies; m++ {
		body := BodyEuler{
			Point: Point{
				X: Float32frombytes(data[pos : pos+4]),
				Y: Float32frombytes(data[pos+4 : pos+8]),
				Z: Float32frombytes(data[pos+8 : pos+12]),
			},
		}
		for i := 0; i < 3; i++ {
			body.Angles[i] = Float32frombytes(data[pos+12+i*4 : pos+16+i*4])
		}
		body.Residual = Float32frombytes(data[pos+24 : pos+28])
		c.Bodies = append(c.Bodies, body)
		pos += 28
	}
	return nil
}
