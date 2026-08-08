package packets

import "fmt"

//go:generate stringer -type ImageFormatType -trimprefix ImageFormatType
type ImageFormatType uint32

const (
	ImageFormatTypeRawGreyscale ImageFormatType = iota
	ImageFormatTypeRawBGR
	ImageFormatTypeJPG
	ImageFormatTypePNG
)

type Image struct {
	ID                                       uint32
	Format                                   ImageFormatType
	Width, Height                            uint32
	LeftCrop, TopCrop, RightCrop, BottomCrop float32
	Size                                     uint32
	Data                                     []byte
}

// String deliberately reports the payload length rather than dumping the bytes;
// a single video frame is easily megabytes and printing it is never useful.
func (c Image) String() string {
	return fmt.Sprintf("[id: %v format: %v width: %v height: %v left: %v top: %v right: %v bottom: %v size: %v data: %d bytes]",
		c.ID, c.Format, c.Width, c.Height,
		c.LeftCrop, c.TopCrop, c.RightCrop, c.BottomCrop,
		c.Size, len(c.Data),
	)
}

type ComponentImage struct {
	Images []Image
}

func (c ComponentImage) String() string { return fmt.Sprintf("%v", c.Images) }

// imageHeaderBytes is id, format, width, height, four crop floats and size.
const imageHeaderBytes = 36

// UnmarshalBinary decodes camera images.
//
// Two defects are fixed here. Width was read from a zero-length slice
// (data[pos+8:pos+8]), which panicked on every image frame, and Data was
// allocated with make([]byte, 0, size) before copy, so copy had nowhere to
// write and every image came back empty.
func (c *ComponentImage) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	imageCount := cur.Uint32()
	if !cur.checkCount(imageCount, imageHeaderBytes, "image") {
		return cur.Err()
	}

	c.Images = make([]Image, 0, imageCount)
	for i := uint32(0); i < imageCount; i++ {
		img := Image{
			ID:         cur.Uint32(),
			Format:     ImageFormatType(cur.Uint32()),
			Width:      cur.Uint32(),
			Height:     cur.Uint32(),
			LeftCrop:   cur.Float32(),
			TopCrop:    cur.Float32(),
			RightCrop:  cur.Float32(),
			BottomCrop: cur.Float32(),
		}
		img.Size = cur.Uint32()
		if cur.Err() != nil {
			return cur.Err()
		}
		if uint64(img.Size) > uint64(cur.Remaining()) {
			return fmt.Errorf("%w: image %d claims %d bytes, %d remaining",
				ErrShortPacket, img.ID, img.Size, cur.Remaining())
		}
		img.Data = cur.Bytes(int(img.Size))
		c.Images = append(c.Images, img)
	}
	return cur.Err()
}
