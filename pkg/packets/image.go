package packets

import (
	"encoding/binary"
	"fmt"
)

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

func (c Image) String() string {
	return fmt.Sprintf("[id: %v format: %v width: %v height: %v left: %v top: %v right: %v bottom: %v size: %v data: %v",
		c.ID,
		c.Format,
		c.Width, c.Height,
		c.LeftCrop, c.TopCrop, c.RightCrop, c.BottomCrop,
		c.Size,
		c.Data,
	)
}

type ComponentImage struct {
	Images []Image
}

func (c ComponentImage) String() string {
	return fmt.Sprintf("%v", c.Images)
}

func (c *ComponentImage) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	imageCount := binary.LittleEndian.Uint32(data[0:4])
	pos := uint32(4)
	c.Images = make([]Image, 0, imageCount)
	for i := uint32(0); i < imageCount; i++ {
		image := Image{
			ID:         binary.LittleEndian.Uint32(data[pos : pos+4]),
			Format:     ImageFormatType(binary.LittleEndian.Uint32(data[pos+4 : pos+8])),
			Width:      binary.LittleEndian.Uint32(data[pos+8 : pos+8]),
			Height:     binary.LittleEndian.Uint32(data[pos+12 : pos+16]),
			LeftCrop:   Float32frombytes(data[pos+16 : pos+20]),
			TopCrop:    Float32frombytes(data[pos+20 : pos+24]),
			RightCrop:  Float32frombytes(data[pos+24 : pos+28]),
			BottomCrop: Float32frombytes(data[pos+28 : pos+32]),
			Size:       binary.LittleEndian.Uint32(data[pos+32 : pos+36]),
		}
		image.Data = make([]byte, 0, image.Size)
		copy(image.Data, data[pos+36:pos+36+image.Size])
		pos += 36 + image.Size
		c.Images = append(c.Images, image)
	}
	return nil
}
