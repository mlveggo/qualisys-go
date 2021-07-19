package packets

import (
	"encoding/binary"
	"fmt"
)

type Rotation struct {
	X, Y, Z, W float32
}

type Segment struct {
	ID       uint32
	Position Point
	Rotation Rotation
}

func (s Segment) String() string {
	return fmt.Sprintf(
		"[id: %v x: %v y: %v z: %v rx:%v ry:%v rz:%v rw:%v]",
		s.ID, s.Position.X, s.Position.Y, s.Position.Z,
		s.Rotation.X, s.Rotation.Y, s.Rotation.Z, s.Rotation.W,
	)
}

type Skeleton struct {
	Segments []Segment
}

type ComponentSkeleton struct {
	Skeletons []Skeleton
}

func (c ComponentSkeleton) String() string {
	return fmt.Sprintf("Skeletons: %v", c.Skeletons)
}

func (c *ComponentSkeleton) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfSkeletons := binary.LittleEndian.Uint32(data[0:4])
	pos := 4
	c.Skeletons = make([]Skeleton, 0, numberOfSkeletons)
	for m := uint32(0); m < numberOfSkeletons; m++ {
		segments := binary.LittleEndian.Uint32(data[pos : pos+4])
		s := make([]Segment, segments)
		for i := uint32(0); i < segments; i++ {
			s[i].ID = binary.LittleEndian.Uint32(data[pos+4 : pos+8])
			s[i].Position.X = Float32frombytes(data[pos+8 : pos+12])
			s[i].Position.Y = Float32frombytes(data[pos+12 : pos+16])
			s[i].Position.Z = Float32frombytes(data[pos+16 : pos+20])
			s[i].Rotation.X = Float32frombytes(data[pos+20 : pos+24])
			s[i].Rotation.Y = Float32frombytes(data[pos+24 : pos+28])
			s[i].Rotation.Z = Float32frombytes(data[pos+28 : pos+32])
			s[i].Rotation.W = Float32frombytes(data[pos+32 : pos+36])
			pos += 32
		}
		c.Skeletons = append(c.Skeletons, Skeleton{Segments: s})
		pos += 4
	}
	return nil
}
