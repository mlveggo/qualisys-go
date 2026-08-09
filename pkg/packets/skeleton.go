package packets

import "fmt"

type Rotation struct {
	X, Y, Z, W float32
}

// Segment is one skeleton segment. Position and rotation are relative to the
// parent segment unless the stream was requested with the "global" option, in
// which case they are in the global coordinate system.
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

func (s Skeleton) String() string { return fmt.Sprintf("%v", s.Segments) }

type ComponentSkeleton struct {
	Skeletons []Skeleton
}

func (c ComponentSkeleton) String() string {
	return fmt.Sprintf("Skeletons: %v", c.Skeletons)
}

// segmentBytes is id + 3 position floats + 4 rotation floats.
const segmentBytes = 32

func (c *ComponentSkeleton) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	skeletonCount := cur.Uint32()
	if !cur.checkCount(skeletonCount, 4, "skeleton") {
		return cur.Err()
	}

	c.Skeletons = make([]Skeleton, 0, skeletonCount)
	for i := uint32(0); i < skeletonCount; i++ {
		segmentCount := cur.Uint32()
		if !cur.checkCount(segmentCount, segmentBytes, "skeleton segment") {
			return cur.Err()
		}
		segments := make([]Segment, segmentCount)
		for s := uint32(0); s < segmentCount; s++ {
			segments[s].ID = cur.Uint32()
			segments[s].Position = cur.Point()
			segments[s].Rotation = Rotation{
				X: cur.Float32(), Y: cur.Float32(), Z: cur.Float32(), W: cur.Float32(),
			}
		}
		c.Skeletons = append(c.Skeletons, Skeleton{Segments: segments})
	}
	return cur.Err()
}
