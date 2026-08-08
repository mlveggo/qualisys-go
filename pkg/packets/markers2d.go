package packets

import "fmt"

// Marker2D is a marker as seen by a single camera. Positions and diameters are
// in camera sensor subpixel units.
type Marker2D struct {
	X         uint32
	Y         uint32
	DiameterX uint16
	DiameterY uint16
}

func (m Marker2D) String() string {
	return fmt.Sprintf("[x:%v y:%v dx:%v dy:%v]", m.X, m.Y, m.DiameterX, m.DiameterY)
}

// Camera holds the 2D markers seen by one camera.
type Camera struct {
	Markers []Marker2D
	Status  uint8
}

func (c Camera) String() string {
	// The previous implementation passed Markers where Status was formatted and
	// vice versa, so every 2D dump printed the two fields swapped.
	return fmt.Sprintf("Status: %v Markers2D: %v", c.Status, c.Markers)
}

type Component2D struct {
	Droprate      uint16
	OutOfSyncRate uint16
	Cameras       []Camera
}

func (c Component2D) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Cameras: %v", c.Droprate, c.OutOfSyncRate, c.Cameras)
}

func unmarshal2D(c *Component2D, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	cameraCount := cur.Uint32()
	c.Droprate = cur.Uint16()
	c.OutOfSyncRate = cur.Uint16()

	// Each camera block is 4 bytes marker count + 1 byte status flags, then
	// 12 bytes per marker.
	const cameraHeader = 5
	if !cur.checkCount(cameraCount, cameraHeader, "2d camera") {
		return cur.Err()
	}

	c.Cameras = make([]Camera, 0, cameraCount)
	for i := uint32(0); i < cameraCount; i++ {
		markerCount := cur.Uint32()
		cam := Camera{Status: cur.Uint8()}
		if !cur.checkCount(markerCount, 12, "2d marker") {
			return cur.Err()
		}
		cam.Markers = make([]Marker2D, 0, markerCount)
		for m := uint32(0); m < markerCount; m++ {
			cam.Markers = append(cam.Markers, Marker2D{
				X:         cur.Uint32(),
				Y:         cur.Uint32(),
				DiameterX: cur.Uint16(),
				DiameterY: cur.Uint16(),
			})
		}
		c.Cameras = append(c.Cameras, cam)
	}
	return cur.Err()
}

func (c *Component2D) UnmarshalBinary(data []byte) error {
	return unmarshal2D(c, data)
}

type Component2DLinearized Component2D

func (c Component2DLinearized) String() string { return Component2D(c).String() }

func (c *Component2DLinearized) UnmarshalBinary(data []byte) error {
	return unmarshal2D((*Component2D)(c), data)
}
