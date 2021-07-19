package packets

import (
	"encoding/binary"
	"fmt"
)

type Marker struct {
	Point    Point
	Residual float32
	ID       uint32
}

func (m Marker) String() string {
	return fmt.Sprintf("[id: %v x:%v y:%v z:%v r:%v]", m.ID, m.Point.X, m.Point.Y, m.Point.Z, m.Residual)
}

type Component3D struct {
	Droprate      uint16
	OutOfSyncRate uint16
	Markers       []Marker
}

func (c Component3D) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Markers: %v", c.Droprate, c.OutOfSyncRate, c.Markers)
}

func (c *Component3D) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfMarkers := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Markers = make([]Marker, 0, numberOfMarkers)
	for m := uint32(0); m < numberOfMarkers; m++ {
		c.Markers = append(c.Markers, Marker{
			Point: Point{
				X: Float32frombytes(data[pos : pos+4]),
				Y: Float32frombytes(data[pos+4 : pos+8]),
				Z: Float32frombytes(data[pos+8 : pos+12]),
			},
		})
		pos += 12
	}
	return nil
}

type Component3DResidual Component3D

func (c Component3DResidual) String() string {
	return Component3D(c).String()
}

func (c *Component3DResidual) UnmarshalBinary(data []byte) error {
	numberOfMarkers := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Markers = make([]Marker, 0, numberOfMarkers)
	for m := uint32(0); m < numberOfMarkers; m++ {
		c.Markers = append(c.Markers, Marker{
			Point: Point{
				X: Float32frombytes(data[pos : pos+4]),
				Y: Float32frombytes(data[pos+4 : pos+8]),
				Z: Float32frombytes(data[pos+8 : pos+12]),
			},
			Residual: Float32frombytes(data[pos+12 : pos+16]),
		})
		pos += 16
	}
	return nil
}

type Component3DNoLabels Component3D

func (c Component3DNoLabels) String() string {
	return Component3D(c).String()
}

func (c *Component3DNoLabels) UnmarshalBinary(data []byte) error {
	numberOfMarkers := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Markers = make([]Marker, 0, numberOfMarkers)
	for m := uint32(0); m < numberOfMarkers; m++ {
		c.Markers = append(c.Markers, Marker{
			Point: Point{
				X: Float32frombytes(data[pos : pos+4]),
				Y: Float32frombytes(data[pos+4 : pos+8]),
				Z: Float32frombytes(data[pos+8 : pos+12]),
			},
			ID: binary.LittleEndian.Uint32(data[pos+12 : pos+16]),
		})
		pos += 16
	}
	return nil
}

type Component3DNoLabelsResidual Component3D

func (c Component3DNoLabelsResidual) String() string {
	return Component3D(c).String()
}

func (c *Component3DNoLabelsResidual) UnmarshalBinary(data []byte) error {
	numberOfMarkers := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Markers = make([]Marker, 0, numberOfMarkers)
	for m := uint32(0); m < numberOfMarkers; m++ {
		c.Markers = append(c.Markers, Marker{
			Point: Point{
				X: Float32frombytes(data[pos : pos+4]),
				Y: Float32frombytes(data[pos+4 : pos+8]),
				Z: Float32frombytes(data[pos+8 : pos+12]),
			},
			ID:       binary.LittleEndian.Uint32(data[pos+12 : pos+16]),
			Residual: Float32frombytes(data[pos+16 : pos+20]),
		})
		pos += 20
	}
	return nil
}
