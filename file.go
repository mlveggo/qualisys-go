package qualisys

import "encoding/binary"

//go:generate stringer -type FileType -trimprefix FileType
type FileType uint32

const (
	FileTypeC3D FileType = 5
	FileTypeQTM FileType = 8
)

type FilePacket struct {
	Size uint32
	Type FileType
	File []byte
}

func (d *FilePacket) UnmarshalBinary(data []byte) error {
	d.Size = binary.LittleEndian.Uint32(data[0:4])
	d.Type = FileType(binary.LittleEndian.Uint32(data[4:8]))
	d.File = make([]byte, 0, d.Size)
	copy(d.File, data[8:])
	return nil
}
