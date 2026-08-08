package packets

import "fmt"

type Marker struct {
	Point    Point
	Residual float32
	ID       uint32
}

func (m Marker) String() string {
	return fmt.Sprintf("[id: %v x:%v y:%v z:%v r:%v]", m.ID, m.Point.X, m.Point.Y, m.Point.Z, m.Residual)
}

// Component3D holds labelled 3D markers.
//
// Markers arrive in the same order as the labels in the 3D settings XML, so
// index N here corresponds to label N from GetParameters 3D. The wire format
// carries no per-marker ID for labelled markers; ID is only populated for the
// NoLabels variants.
type Component3D struct {
	Droprate      uint16
	OutOfSyncRate uint16
	Markers       []Marker
}

func (c Component3D) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Markers: %v", c.Droprate, c.OutOfSyncRate, c.Markers)
}

// unmarshal3D is shared by all four 3D variants. bytesPerMarker documents the
// stride so the record layout is stated once instead of open-coded four times.
func unmarshal3D(c *Component3D, data []byte, withID, withResidual bool) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	count := cur.Uint32()
	c.Droprate = cur.Uint16()
	c.OutOfSyncRate = cur.Uint16()

	stride := 12
	if withID {
		stride += 4
	}
	if withResidual {
		stride += 4
	}
	if !cur.checkCount(count, stride, "3d marker") {
		return cur.Err()
	}

	c.Markers = make([]Marker, 0, count)
	for i := uint32(0); i < count; i++ {
		m := Marker{Point: cur.Point()}
		if withID {
			m.ID = cur.Uint32()
		}
		if withResidual {
			m.Residual = cur.Float32()
		}
		c.Markers = append(c.Markers, m)
	}
	return cur.Err()
}

func (c *Component3D) UnmarshalBinary(data []byte) error {
	return unmarshal3D(c, data, false, false)
}

type Component3DResidual Component3D

func (c Component3DResidual) String() string { return Component3D(c).String() }

func (c *Component3DResidual) UnmarshalBinary(data []byte) error {
	return unmarshal3D((*Component3D)(c), data, false, true)
}

type Component3DNoLabels Component3D

func (c Component3DNoLabels) String() string { return Component3D(c).String() }

func (c *Component3DNoLabels) UnmarshalBinary(data []byte) error {
	return unmarshal3D((*Component3D)(c), data, true, false)
}

type Component3DNoLabelsResidual Component3D

func (c Component3DNoLabelsResidual) String() string { return Component3D(c).String() }

func (c *Component3DNoLabelsResidual) UnmarshalBinary(data []byte) error {
	return unmarshal3D((*Component3D)(c), data, true, true)
}
