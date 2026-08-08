package qualisys

import (
	"fmt"
	"os"
)

//go:generate stringer -type FileType -trimprefix FileType
type FileType uint32

const (
	FileTypeC3D FileType = 5
	FileTypeQTM FileType = 8
)

// FilePacket carries a captured measurement returned by GetCaptureC3D or
// GetCaptureQTM.
type FilePacket struct {
	// Size is the length of File in bytes.
	Size uint32
	// Type distinguishes a C3D transfer from a QTM transfer.
	Type FileType
	// File is the raw file content.
	File []byte
}

// UnmarshalBinary decodes a file transfer payload.
//
// Two bugs are fixed here. The payload of a C3D/QTM file packet is the file
// content itself, starting immediately after the 8 byte packet header -- the
// previous code treated the first eight bytes of the file as a size and type
// field and discarded them, corrupting every transfer. And File was allocated
// with make([]byte, 0, size) before copy, so copy had zero length to write into
// and the result was always empty.
func (d *FilePacket) UnmarshalBinary(data []byte) error {
	d.Size = uint32(len(data))
	d.File = make([]byte, len(data))
	copy(d.File, data)
	return nil
}

// WriteFile writes the transferred file to path.
func (d *FilePacket) WriteFile(path string) error {
	if len(d.File) == 0 {
		return fmt.Errorf("filepacket: no file content received")
	}
	if err := os.WriteFile(path, d.File, 0o644); err != nil {
		return fmt.Errorf("filepacket: write %s: %w", path, err)
	}
	return nil
}
