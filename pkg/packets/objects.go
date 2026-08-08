package packets

import "fmt"

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

// Component6D holds 6DOF bodies as position plus a row-major 3x3 rotation
// matrix. Bodies arrive in the order they appear in the 6D settings XML.
type Component6D struct {
	Droprate      uint16
	OutOfSyncRate uint16
	Bodies        []BodyMatrix
}

func (c Component6D) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Bodies: %v\n", c.Droprate, c.OutOfSyncRate, c.Bodies)
}

func unmarshal6D(c *Component6D, data []byte, withResidual bool) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	count := cur.Uint32()
	c.Droprate = cur.Uint16()
	c.OutOfSyncRate = cur.Uint16()

	stride := 48 // 3 floats position + 9 floats rotation
	if withResidual {
		stride += 4
	}
	if !cur.checkCount(count, stride, "6d body") {
		return cur.Err()
	}

	c.Bodies = make([]BodyMatrix, 0, count)
	for i := uint32(0); i < count; i++ {
		body := BodyMatrix{Point: cur.Point()}
		for r := 0; r < 9; r++ {
			body.Rotation[r] = cur.Float32()
		}
		if withResidual {
			body.Residual = cur.Float32()
		}
		c.Bodies = append(c.Bodies, body)
	}
	return cur.Err()
}

func (c *Component6D) UnmarshalBinary(data []byte) error {
	return unmarshal6D(c, data, false)
}

type Component6DResidual Component6D

func (c Component6DResidual) String() string { return Component6D(c).String() }

func (c *Component6DResidual) UnmarshalBinary(data []byte) error {
	return unmarshal6D((*Component6D)(c), data, true)
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

// Component6DEuler holds 6DOF bodies as position plus Euler angles. The angle
// convention is reported by the General settings XML (the Euler element), so it
// is not fixed by this struct.
type Component6DEuler struct {
	Droprate      uint16
	OutOfSyncRate uint16
	Bodies        []BodyEuler
}

func (c Component6DEuler) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Bodies: %v\n", c.Droprate, c.OutOfSyncRate, c.Bodies)
}

func unmarshal6DEuler(c *Component6DEuler, data []byte, withResidual bool) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	count := cur.Uint32()
	c.Droprate = cur.Uint16()
	c.OutOfSyncRate = cur.Uint16()

	stride := 24 // 3 floats position + 3 floats angles
	if withResidual {
		stride += 4
	}
	if !cur.checkCount(count, stride, "6d euler body") {
		return cur.Err()
	}

	c.Bodies = make([]BodyEuler, 0, count)
	for i := uint32(0); i < count; i++ {
		body := BodyEuler{Point: cur.Point()}
		for a := 0; a < 3; a++ {
			body.Angles[a] = cur.Float32()
		}
		if withResidual {
			body.Residual = cur.Float32()
		}
		c.Bodies = append(c.Bodies, body)
	}
	return cur.Err()
}

func (c *Component6DEuler) UnmarshalBinary(data []byte) error {
	return unmarshal6DEuler(c, data, false)
}

type Component6DEulerResidual Component6DEuler

func (c Component6DEulerResidual) String() string { return Component6DEuler(c).String() }

func (c *Component6DEulerResidual) UnmarshalBinary(data []byte) error {
	return unmarshal6DEuler((*Component6DEuler)(c), data, true)
}
