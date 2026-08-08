package packets

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrShortPacket is returned when a component payload ends before all the
// fields the protocol says it should contain have been read.
//
// Before this type existed every parser in this package indexed straight into
// the payload slice, so a truncated or malformed packet took the whole process
// down with an "index out of range" panic. A streaming client should never
// crash because one frame arrived short.
var ErrShortPacket = errors.New("packets: short packet")

// cursor is a bounds-checked, little-endian sequential reader.
//
// The QTM real time protocol is a dense binary format and every parser here is
// a long run of fixed offsets. Doing that arithmetic by hand is how bugs like
// reading data[pos+8:pos+8] (a guaranteed panic) and forgetting to advance
// between repeated records crept in. The cursor makes the stride explicit and
// makes running off the end an error instead of a panic.
type cursor struct {
	data []byte
	pos  int
	err  error
}

func newCursor(data []byte) *cursor {
	return &cursor{data: data}
}

// fail records the first error seen. Once a cursor is in the error state every
// subsequent read is a no-op returning the zero value, so parsers can read a
// whole record and check err once at the end rather than after every field.
func (c *cursor) fail(err error) {
	if c.err == nil {
		c.err = err
	}
}

// Err reports the first error encountered, if any.
func (c *cursor) Err() error {
	return c.err
}

// Remaining reports how many unread bytes are left.
func (c *cursor) Remaining() int {
	if c.err != nil {
		return 0
	}
	return len(c.data) - c.pos
}

// need checks that n more bytes are available and returns the slice covering
// them, advancing the position. It returns nil if the read would overrun.
func (c *cursor) need(n int) []byte {
	if c.err != nil {
		return nil
	}
	if n < 0 || c.pos+n > len(c.data) {
		c.fail(fmt.Errorf("%w: need %d bytes at offset %d, have %d",
			ErrShortPacket, n, c.pos, len(c.data)-c.pos))
		return nil
	}
	b := c.data[c.pos : c.pos+n]
	c.pos += n
	return b
}

func (c *cursor) Uint8() uint8 {
	b := c.need(1)
	if b == nil {
		return 0
	}
	return b[0]
}

func (c *cursor) Uint16() uint16 {
	b := c.need(2)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(b)
}

func (c *cursor) Uint32() uint32 {
	b := c.need(4)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

func (c *cursor) Uint64() uint64 {
	b := c.need(8)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}

func (c *cursor) Float32() float32 {
	b := c.need(4)
	if b == nil {
		return 0
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(b))
}

// Point reads three consecutive float32 as an X/Y/Z triple.
func (c *cursor) Point() Point {
	return Point{X: c.Float32(), Y: c.Float32(), Z: c.Float32()}
}

// Bytes returns a copy of the next n bytes.
//
// A copy is deliberate: the protocol reuses one receive buffer for every frame,
// so handing out a sub-slice would leave callers holding memory that the next
// frame overwrites underneath them.
func (c *cursor) Bytes(n int) []byte {
	b := c.need(n)
	if b == nil {
		return nil
	}
	out := make([]byte, n)
	copy(out, b)
	return out
}

// Skip advances past n bytes without reading them.
func (c *cursor) Skip(n int) {
	c.need(n)
}

// checkCount guards against a corrupt or hostile record count causing a huge
// allocation before any data is validated. minBytesEach is the smallest number
// of bytes one record can occupy.
func (c *cursor) checkCount(count uint32, minBytesEach int, what string) bool {
	if c.err != nil {
		return false
	}
	if minBytesEach > 0 && uint64(count)*uint64(minBytesEach) > uint64(c.Remaining()) {
		c.fail(fmt.Errorf("%w: %s count %d exceeds %d remaining bytes",
			ErrShortPacket, what, count, c.Remaining()))
		return false
	}
	return true
}

// Float32frombytes decodes a little-endian float32.
//
// Deprecated: retained for backwards compatibility with code outside this
// package. It panics on a slice shorter than four bytes; prefer the internal
// cursor for anything parsing wire data.
func Float32frombytes(b []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b))
}
