package packets

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

// builder assembles little-endian component payloads for tests.
type builder struct{ b []byte }

func (w *builder) u8(v uint8) *builder { w.b = append(w.b, v); return w }

func (w *builder) u16(v uint16) *builder {
	w.b = binary.LittleEndian.AppendUint16(w.b, v)
	return w
}

func (w *builder) u32(v uint32) *builder {
	w.b = binary.LittleEndian.AppendUint32(w.b, v)
	return w
}

func (w *builder) f32(v float32) *builder {
	return w.u32(math.Float32bits(v))
}

func (w *builder) raw(p []byte) *builder { w.b = append(w.b, p...); return w }

func TestComponentImageDecodesHeaderAndPayload(t *testing.T) {
	// Regression: Width was read from data[pos+8:pos+8], a zero-length slice,
	// which panicked on every image frame. Data was allocated with length zero
	// before copy, so the payload was always dropped.
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01}
	b := (&builder{}).
		u32(1).    // image count
		u32(7).    // camera id
		u32(2).    // format: JPG
		u32(1920). // width
		u32(1080). // height
		f32(0.1).f32(0.2).f32(0.3).f32(0.4).
		u32(uint32(len(payload))).
		raw(payload)

	var c ComponentImage
	if err := c.UnmarshalBinary(b.b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(c.Images) != 1 {
		t.Fatalf("got %d images, want 1", len(c.Images))
	}
	img := c.Images[0]
	if img.ID != 7 || img.Format != ImageFormatTypeJPG {
		t.Errorf("id=%d format=%v", img.ID, img.Format)
	}
	if img.Width != 1920 {
		t.Errorf("width = %d, want 1920", img.Width)
	}
	if img.Height != 1080 {
		t.Errorf("height = %d, want 1080", img.Height)
	}
	if string(img.Data) != string(payload) {
		t.Errorf("data = %v, want %v", img.Data, payload)
	}
}

func TestComponentImageRejectsOversizedPayload(t *testing.T) {
	b := (&builder{}).
		u32(1).u32(1).u32(0).u32(4).u32(4).
		f32(0).f32(0).f32(0).f32(0).
		u32(1 << 30) // claims a gigabyte that is not there

	var c ComponentImage
	err := c.UnmarshalBinary(b.b)
	if err == nil {
		t.Fatal("expected an error for an oversized image payload")
	}
	if !errors.Is(err, ErrShortPacket) {
		t.Errorf("got %v, want ErrShortPacket", err)
	}
}

func TestComponentTimecodeAdvancesBetweenEntries(t *testing.T) {
	// Regression: every entry was decoded from the same three fixed offsets, so
	// a stream carrying N timecodes reported the first one N times.
	b := (&builder{}).
		u32(2).
		u32(uint32(TimecodeTypeSMPTE)).u32(0).u32(encodeSMPTE(1, 2, 3, 4, 5)).
		u32(uint32(TimecodeTypeSMPTE)).u32(0).u32(encodeSMPTE(6, 7, 8, 9, 10))

	var c ComponentTimecode
	if err := c.UnmarshalBinary(b.b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(c.Timecodes) != 2 {
		t.Fatalf("got %d timecodes, want 2", len(c.Timecodes))
	}
	first, second := c.Timecodes[0].Smpte, c.Timecodes[1].Smpte
	if first.Hour != 1 || first.Minute != 2 || first.Second != 3 || first.Frame != 4 || first.SubFrame != 5 {
		t.Errorf("first = %+v", first)
	}
	if second.Hour != 6 || second.Minute != 7 || second.Second != 8 || second.Frame != 9 || second.SubFrame != 10 {
		t.Errorf("second = %+v, entries were not advanced", second)
	}
}

func encodeSMPTE(h, m, s, f, sub uint32) uint32 {
	return h | m<<5 | s<<11 | f<<17 | sub<<22
}

func TestSmpteNormalizedSubFrame(t *testing.T) {
	tc := SmpteTime{SubFrame: 3}
	if got := tc.NormalizedSubFrame(120, 30); got != 0.75 {
		t.Errorf("got %v, want 0.75", got)
	}
	// Guard against divide-by-zero and inverted frequencies.
	if got := tc.NormalizedSubFrame(0, 30); got != 0 {
		t.Errorf("zero capture frequency: got %v, want 0", got)
	}
	if got := tc.NormalizedSubFrame(30, 120); got != 0 {
		t.Errorf("capture below timestamp frequency: got %v, want 0", got)
	}
}

func TestComponent3DVariantStrides(t *testing.T) {
	cases := []struct {
		name         string
		obj          interface{ UnmarshalBinary([]byte) error }
		withID       bool
		withResidual bool
	}{
		{"3D", &Component3D{}, false, false},
		{"3DRes", &Component3DResidual{}, false, true},
		{"3DNoLabels", &Component3DNoLabels{}, true, false},
		{"3DNoLabelsRes", &Component3DNoLabelsResidual{}, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := (&builder{}).u32(2).u16(11).u16(22)
			for i := 0; i < 2; i++ {
				w.f32(float32(i)).f32(float32(i) + 0.5).f32(float32(i) + 0.25)
				if tc.withID {
					w.u32(uint32(100 + i))
				}
				if tc.withResidual {
					w.f32(float32(i) * 2)
				}
			}
			if err := tc.obj.UnmarshalBinary(w.b); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			c := (*Component3D)(nil)
			switch v := tc.obj.(type) {
			case *Component3D:
				c = v
			case *Component3DResidual:
				c = (*Component3D)(v)
			case *Component3DNoLabels:
				c = (*Component3D)(v)
			case *Component3DNoLabelsResidual:
				c = (*Component3D)(v)
			}
			if c.Droprate != 11 || c.OutOfSyncRate != 22 {
				t.Errorf("droprate=%d oosrate=%d", c.Droprate, c.OutOfSyncRate)
			}
			if len(c.Markers) != 2 {
				t.Fatalf("got %d markers, want 2", len(c.Markers))
			}
			if c.Markers[1].Point.X != 1 {
				t.Errorf("second marker X = %v, want 1", c.Markers[1].Point.X)
			}
			if tc.withID && c.Markers[1].ID != 101 {
				t.Errorf("second marker ID = %d, want 101", c.Markers[1].ID)
			}
			if tc.withResidual && c.Markers[1].Residual != 2 {
				t.Errorf("second marker residual = %v, want 2", c.Markers[1].Residual)
			}
		})
	}
}

func TestComponent2DCameraStride(t *testing.T) {
	// Two cameras with different marker counts exercises the 5 byte camera
	// header plus 12 bytes per marker stride.
	w := (&builder{}).u32(2).u16(0).u16(0)
	w.u32(2).u8(0x01)
	w.u32(10).u32(20).u16(3).u16(4)
	w.u32(11).u32(21).u16(5).u16(6)
	w.u32(1).u8(0x02)
	w.u32(30).u32(40).u16(7).u16(8)

	var c Component2D
	if err := c.UnmarshalBinary(w.b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(c.Cameras) != 2 {
		t.Fatalf("got %d cameras, want 2", len(c.Cameras))
	}
	if len(c.Cameras[0].Markers) != 2 || len(c.Cameras[1].Markers) != 1 {
		t.Fatalf("marker counts = %d, %d", len(c.Cameras[0].Markers), len(c.Cameras[1].Markers))
	}
	if c.Cameras[1].Status != 0x02 {
		t.Errorf("second camera status = %#x, want 0x02", c.Cameras[1].Status)
	}
	if got := c.Cameras[1].Markers[0]; got.X != 30 || got.Y != 40 || got.DiameterX != 7 || got.DiameterY != 8 {
		t.Errorf("second camera marker = %+v", got)
	}
}

func TestComponentSkeletonSegments(t *testing.T) {
	w := (&builder{}).u32(2)
	// Skeleton 0: two segments.
	w.u32(2)
	w.u32(1).f32(1).f32(2).f32(3).f32(0).f32(0).f32(0).f32(1)
	w.u32(2).f32(4).f32(5).f32(6).f32(0).f32(0).f32(1).f32(0)
	// Skeleton 1: one segment.
	w.u32(1)
	w.u32(9).f32(7).f32(8).f32(9).f32(1).f32(0).f32(0).f32(0)

	var c ComponentSkeleton
	if err := c.UnmarshalBinary(w.b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(c.Skeletons) != 2 {
		t.Fatalf("got %d skeletons, want 2", len(c.Skeletons))
	}
	if len(c.Skeletons[0].Segments) != 2 || len(c.Skeletons[1].Segments) != 1 {
		t.Fatalf("segment counts = %d, %d", len(c.Skeletons[0].Segments), len(c.Skeletons[1].Segments))
	}
	if got := c.Skeletons[1].Segments[0]; got.ID != 9 || got.Position.X != 7 || got.Rotation.X != 1 {
		t.Errorf("second skeleton segment = %+v", got)
	}
}

func TestComponentAnalogGroupsSamplesByChannel(t *testing.T) {
	w := (&builder{}).u32(1)
	w.u32(5).u32(2).u32(3).u32(42) // device 5, 2 channels, 3 samples, sample number 42
	w.f32(1).f32(2).f32(3)         // channel 0
	w.f32(4).f32(5).f32(6)         // channel 1

	var c ComponentAnalog
	if err := c.UnmarshalBinary(w.b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	dev := c.AnalogDevices[0]
	if dev.ID != 5 || dev.SampleNumber != 42 {
		t.Errorf("id=%d sampleNumber=%d", dev.ID, dev.SampleNumber)
	}
	if len(dev.Channels) != 2 {
		t.Fatalf("got %d channels, want 2", len(dev.Channels))
	}
	if dev.Channels[1].Samples[0].Value != 4 {
		t.Errorf("channel 1 sample 0 = %v, want 4", dev.Channels[1].Samples[0].Value)
	}
}

func TestComponentForcePlateSamples(t *testing.T) {
	w := (&builder{}).u32(1)
	w.u32(3).u32(2).u32(77) // plate 3, 2 samples, force number 77
	for i := 0; i < 2; i++ {
		base := float32(i * 10)
		w.f32(base).f32(base + 1).f32(base + 2)
		w.f32(base + 3).f32(base + 4).f32(base + 5)
		w.f32(base + 6).f32(base + 7).f32(base + 8)
	}

	var c ComponentForce
	if err := c.UnmarshalBinary(w.b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fp := c.ForcePlates[0]
	if fp.ID != 3 || fp.Number != 77 || len(fp.Samples) != 2 {
		t.Fatalf("plate = %+v", fp)
	}
	if fp.Samples[1].CenterOfPressure.Z != 18 {
		t.Errorf("second sample CoP.Z = %v, want 18", fp.Samples[1].CenterOfPressure.Z)
	}
}

func TestGazeVectorZeroSampleDeviceOmitsSampleNumber(t *testing.T) {
	// A device reporting no samples writes only its 4 byte sample count, with
	// no sample number field. Getting this stride wrong desynchronises every
	// following device.
	w := (&builder{}).u32(2)
	w.u32(0) // device 0: no samples
	w.u32(1).u32(99)
	w.f32(1).f32(2).f32(3).f32(4).f32(5).f32(6)

	var c ComponentGazeVector
	if err := c.UnmarshalBinary(w.b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(c.GazeVectors) != 2 {
		t.Fatalf("got %d gaze vectors, want 2", len(c.GazeVectors))
	}
	if len(c.GazeVectors[0].Samples) != 0 {
		t.Errorf("first device should have no samples")
	}
	if c.GazeVectors[1].SampleNumber != 99 || c.GazeVectors[1].Samples[0].PositionZ != 6 {
		t.Errorf("second device = %+v", c.GazeVectors[1])
	}
}

func TestEyeTrackerZeroSampleDeviceOmitsSampleNumber(t *testing.T) {
	w := (&builder{}).u32(2)
	w.u32(0)
	w.u32(1).u32(7)
	w.f32(2.5).f32(3.5)

	var c ComponentEyeTracker
	if err := c.UnmarshalBinary(w.b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(c.EyeTrackers) != 2 {
		t.Fatalf("got %d eye trackers, want 2", len(c.EyeTrackers))
	}
	if c.EyeTrackers[1].SampleNumber != 7 || c.EyeTrackers[1].Samples[0].RightPupilDiameter != 3.5 {
		t.Errorf("second device = %+v", c.EyeTrackers[1])
	}
}

// TestTruncatedPayloadsDoNotPanic feeds every parser a progressively truncated
// version of a valid payload. Before the bounds-checked cursor these all
// panicked with an index out of range, taking down the whole client process for
// one malformed frame.
func TestTruncatedPayloadsDoNotPanic(t *testing.T) {
	valid := (&builder{}).u32(2).u16(1).u16(2).
		f32(1).f32(2).f32(3).u32(4).f32(5).
		f32(6).f32(7).f32(8).u32(9).f32(10).b

	parsers := map[string]func() interface{ UnmarshalBinary([]byte) error }{
		"3D":            func() interface{ UnmarshalBinary([]byte) error } { return &Component3D{} },
		"3DRes":         func() interface{ UnmarshalBinary([]byte) error } { return &Component3DResidual{} },
		"3DNoLabels":    func() interface{ UnmarshalBinary([]byte) error } { return &Component3DNoLabels{} },
		"3DNoLabelsRes": func() interface{ UnmarshalBinary([]byte) error } { return &Component3DNoLabelsResidual{} },
		"6D":            func() interface{ UnmarshalBinary([]byte) error } { return &Component6D{} },
		"6DRes":         func() interface{ UnmarshalBinary([]byte) error } { return &Component6DResidual{} },
		"6DEuler":       func() interface{ UnmarshalBinary([]byte) error } { return &Component6DEuler{} },
		"6DEulerRes":    func() interface{ UnmarshalBinary([]byte) error } { return &Component6DEulerResidual{} },
		"2D":            func() interface{ UnmarshalBinary([]byte) error } { return &Component2D{} },
		"2DLin":         func() interface{ UnmarshalBinary([]byte) error } { return &Component2DLinearized{} },
		"Analog":        func() interface{ UnmarshalBinary([]byte) error } { return &ComponentAnalog{} },
		"AnalogSingle":  func() interface{ UnmarshalBinary([]byte) error } { return &ComponentAnalogSingle{} },
		"Force":         func() interface{ UnmarshalBinary([]byte) error } { return &ComponentForce{} },
		"ForceSingle":   func() interface{ UnmarshalBinary([]byte) error } { return &ComponentForceSingle{} },
		"Image":         func() interface{ UnmarshalBinary([]byte) error } { return &ComponentImage{} },
		"GazeVector":    func() interface{ UnmarshalBinary([]byte) error } { return &ComponentGazeVector{} },
		"EyeTracker":    func() interface{ UnmarshalBinary([]byte) error } { return &ComponentEyeTracker{} },
		"Timecode":      func() interface{ UnmarshalBinary([]byte) error } { return &ComponentTimecode{} },
		"Skeleton":      func() interface{ UnmarshalBinary([]byte) error } { return &ComponentSkeleton{} },
	}

	for name, mk := range parsers {
		t.Run(name, func(t *testing.T) {
			for n := 0; n <= len(valid); n++ {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("panic on %d byte payload: %v", n, r)
						}
					}()
					// Errors are fine here; panics are not.
					_ = mk().UnmarshalBinary(valid[:n])
				}()
			}
		})
	}
}

func TestCursorReportsShortReads(t *testing.T) {
	c := newCursor([]byte{1, 2})
	c.Uint32()
	if !errors.Is(c.Err(), ErrShortPacket) {
		t.Fatalf("got %v, want ErrShortPacket", c.Err())
	}
	// Once failed, further reads are inert rather than panicking.
	if got := c.Uint32(); got != 0 {
		t.Errorf("read after failure = %d, want 0", got)
	}
}

func TestCursorBytesReturnsACopy(t *testing.T) {
	// The receive buffer is reused between frames, so handing out a sub-slice
	// would let the next frame silently rewrite a caller's data.
	src := []byte{1, 2, 3, 4}
	c := newCursor(src)
	got := c.Bytes(4)
	src[0] = 99
	if got[0] != 1 {
		t.Errorf("Bytes returned an alias into the source buffer")
	}
}

func TestCameraStringFieldOrder(t *testing.T) {
	// Regression: Status and Markers were passed to Sprintf in the wrong order,
	// so every 2D dump printed the two fields swapped.
	cam := Camera{Status: 3, Markers: []Marker2D{{X: 1, Y: 2}}}
	got := cam.String()
	want := "Status: 3 Markers2D: [[x:1 y:2 dx:0 dy:0]]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
