package qualisys

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mlveggo/qualisys-go/pkg/packets"
)

// dataFrame builds a PacketTypeData payload (everything after the 8 byte packet
// header) from a set of already-encoded components.
func dataFrame(timestamp uint64, frame uint32, components ...[]byte) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint64(b[0:8], timestamp)
	binary.LittleEndian.PutUint32(b[8:12], frame)
	binary.LittleEndian.PutUint32(b[12:16], uint32(len(components)))
	for _, c := range components {
		b = append(b, c...)
	}
	return b
}

// component wraps a payload in the per-component size and type header. The size
// field counts the header itself.
func component(t ComponentType, payload []byte) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], uint32(8+len(payload)))
	binary.LittleEndian.PutUint32(b[4:8], uint32(t))
	return append(b, payload...)
}

func marker3DPayload(count uint32) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], count)
	binary.LittleEndian.PutUint16(b[4:6], 1)
	binary.LittleEndian.PutUint16(b[6:8], 2)
	for i := uint32(0); i < count; i++ {
		for j := 0; j < 3; j++ {
			b = binary.LittleEndian.AppendUint32(b, 0)
		}
	}
	return b
}

func TestDataPacketDecodesMultipleComponents(t *testing.T) {
	skeleton := make([]byte, 4) // zero skeletons
	payload := dataFrame(1234, 42,
		component(ComponentType3D, marker3DPayload(3)),
		component(ComponentTypeSkeleton, skeleton),
	)

	var d DataPacket
	if err := d.UnmarshalBinary(payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.Timestamp != 1234 || d.Frame != 42 {
		t.Errorf("timestamp=%d frame=%d", d.Timestamp, d.Frame)
	}
	if len(d.Components) != 2 {
		t.Fatalf("got %d components, want 2", len(d.Components))
	}
	m := d.Markers3D()
	if m == nil {
		t.Fatal("Markers3D returned nil")
	}
	if len(m.Markers) != 3 {
		t.Errorf("got %d markers, want 3", len(m.Markers))
	}
	if d.Skeletons() == nil {
		t.Error("Skeletons returned nil")
	}
}

func TestDataPacketSkipsUnknownComponentTypes(t *testing.T) {
	// Forward compatibility: a newer QTM may stream a component this SDK has
	// never heard of. The old code returned "unknown data object" and threw the
	// whole frame away, including the components it did understand.
	const futureComponent ComponentType = 99
	payload := dataFrame(1, 1,
		component(ComponentType3D, marker3DPayload(1)),
		component(futureComponent, []byte{1, 2, 3, 4, 5, 6}),
	)

	var d DataPacket
	if err := d.UnmarshalBinary(payload); err != nil {
		t.Fatalf("an unknown component should not fail the frame: %v", err)
	}
	// Both components are kept: the one that decoded and the one that could not.
	if len(d.Components) != 2 {
		t.Errorf("got %d components, want 2", len(d.Components))
	}
	if d.Markers3D() == nil {
		t.Error("the recognised component should still decode")
	}
	unknown := d.UnknownComponentTypes()
	if len(unknown) != 1 || unknown[0] != futureComponent {
		t.Errorf("UnknownComponentTypes = %v, want [%d]", unknown, futureComponent)
	}
	// The raw bytes are preserved so a caller that does know the format can
	// decode it themselves.
	var raw *UnknownComponent
	for _, c := range d.Components {
		if u, ok := c.(*UnknownComponent); ok {
			raw = u
		}
	}
	if raw == nil {
		t.Fatal("no UnknownComponent in the frame")
	}
	if string(raw.Data) != string([]byte{1, 2, 3, 4, 5, 6}) {
		t.Errorf("preserved payload = %v, want [1 2 3 4 5 6]", raw.Data)
	}
}

func TestDataPacketRejectsZeroLengthComponent(t *testing.T) {
	// A zero size field would advance the cursor by nothing and loop forever.
	payload := dataFrame(1, 1)
	bad := make([]byte, 8)
	binary.LittleEndian.PutUint32(bad[0:4], 0)
	binary.LittleEndian.PutUint32(bad[4:8], uint32(ComponentType3D))
	binary.LittleEndian.PutUint32(payload[12:16], 1)
	payload = append(payload, bad...)

	var d DataPacket
	if err := d.UnmarshalBinary(payload); err == nil {
		t.Fatal("expected an error for a zero-length component")
	}
}

func TestDataPacketRejectsComponentRunningPastEnd(t *testing.T) {
	payload := dataFrame(1, 1)
	bad := make([]byte, 8)
	binary.LittleEndian.PutUint32(bad[0:4], 4096) // far beyond what follows
	binary.LittleEndian.PutUint32(bad[4:8], uint32(ComponentType3D))
	binary.LittleEndian.PutUint32(payload[12:16], 1)
	payload = append(payload, bad...)

	var d DataPacket
	if err := d.UnmarshalBinary(payload); err == nil {
		t.Fatal("expected an error for a component claiming more bytes than remain")
	}
}

func TestDataPacketComponentsAreBoundedToTheirOwnSlice(t *testing.T) {
	// Each parser must see only its own payload. Passing everything to the end
	// of the buffer, as the old code did, let one component's parser read into
	// the next component's bytes when a count field was wrong.
	first := marker3DPayload(1)
	binary.LittleEndian.PutUint32(first[0:4], 5) // claims 5 markers, carries 1

	payload := dataFrame(1, 1,
		component(ComponentType3D, first),
		component(ComponentType6D, make([]byte, 200)),
	)

	var d DataPacket
	if err := d.UnmarshalBinary(payload); err == nil {
		t.Fatal("expected the over-claiming component to fail rather than read into its neighbour")
	}
}

func TestPacketUnmarshalEventRequiresPayload(t *testing.T) {
	b := make([]byte, packetHeaderSize)
	binary.LittleEndian.PutUint32(b[0:4], packetHeaderSize)
	binary.LittleEndian.PutUint32(b[4:8], uint32(PacketTypeEvent))

	var p Packet
	if err := p.UnmarshalBinary(b); err == nil {
		t.Error("expected an error for an event packet with no payload")
	}
}

func TestFilePacketKeepsEveryByte(t *testing.T) {
	// The payload of a C3D or QTM file packet is the file content itself. The
	// old decoder consumed the first 8 bytes as a size and type field and then
	// copied into a zero-length slice, so transfers came back empty and
	// truncated at once.
	content := []byte("C3D\x00\x01\x02\x03\x04rest of the capture")

	var f FilePacket
	if err := f.UnmarshalBinary(content); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(f.Size) != len(content) {
		t.Errorf("Size = %d, want %d", f.Size, len(content))
	}
	if string(f.File) != string(content) {
		t.Errorf("File = %q, want %q", f.File, content)
	}
}

func TestFilePacketWriteFile(t *testing.T) {
	f := FilePacket{File: []byte("payload"), Size: 7}
	path := filepath.Join(t.TempDir(), "capture.c3d")
	if err := f.WriteFile(path); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "payload" {
		t.Errorf("got %q", got)
	}

	empty := FilePacket{}
	if err := empty.WriteFile(path); err == nil {
		t.Error("expected writing an empty transfer to fail loudly")
	}
}

func TestComponentStringNames(t *testing.T) {
	// ComponentType3DNoLabelsResidual previously rendered as
	// "3dNoLabelsResidual", which QTM does not accept, so that component
	// silently never streamed.
	got, err := componentString(ComponentOptions{},
		ComponentType3D, ComponentType3DNoLabelsResidual, ComponentType6DEulerResidual)
	if err != nil {
		t.Fatalf("componentString: %v", err)
	}
	want := "3D 3DNoLabelsRes 6DEulerRes"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComponentStringOptions(t *testing.T) {
	got, err := componentString(
		ComponentOptions{AnalogChannels: "1,3,5-8", SkeletonGlobal: true},
		ComponentTypeAnalog, ComponentTypeAnalogSingle, ComponentTypeSkeleton, ComponentType3D)
	if err != nil {
		t.Fatalf("componentString: %v", err)
	}
	want := "Analog:1,3,5-8 AnalogSingle:1,3,5-8 Skeleton:global 3D"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComponentStringRejectsUnknownComponent(t *testing.T) {
	if _, err := componentString(ComponentOptions{}, ComponentType(123)); err == nil {
		t.Error("expected an error for an unknown component type")
	}
}

func TestStreamFramesCommandFormatting(t *testing.T) {
	f := newFakeQTM(t)
	f.handler = func(cmd string) []byte {
		if strings.HasPrefix(cmd, "Version ") {
			return commandPacket("Version set to 1.28")
		}
		if cmd == "GetState" {
			return eventPacket(EventTypeConnected)
		}
		return nil
	}
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	if err := rt.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rt.Disconnect()

	if err := rt.StreamFrames(StreamRateTypeFrequency, 100, ComponentType3D); err != nil {
		t.Fatalf("streamframes: %v", err)
	}
	if err := rt.StreamFramesUDP(StreamRateTypeAllFrames, 0, 6734, "192.168.0.5",
		ComponentOptions{SkeletonGlobal: true}, ComponentTypeSkeleton); err != nil {
		t.Fatalf("streamframesudp: %v", err)
	}

	want := []string{
		"StreamFrames Frequency:100 3D",
		"StreamFrames AllFrames UDP:192.168.0.5:6734 Skeleton:global",
	}
	for _, w := range want {
		if !f.waitForCommand(w, 2*time.Second) {
			t.Errorf("missing command %q in %v", w, f.sentCommands())
		}
	}
}

func TestStreamFramesRequiresComponents(t *testing.T) {
	rt := NewProtocol("127.0.0.1", 22222)
	if err := rt.StreamFrames(StreamRateTypeAllFrames, 0); err == nil {
		t.Error("expected an error when no components are requested")
	}
}

func TestStreamFramesUDPValidatesPort(t *testing.T) {
	rt := NewProtocol("127.0.0.1", 22222)
	if err := rt.StreamFramesUDP(StreamRateTypeAllFrames, 0, 0, "", ComponentOptions{}, ComponentType3D); err == nil {
		t.Error("expected an error for UDP port 0")
	}
	if err := rt.StreamFramesUDP(StreamRateTypeAllFrames, 0, 70000, "", ComponentOptions{}, ComponentType3D); err == nil {
		t.Error("expected an error for an out-of-range UDP port")
	}
}

func TestEnableUDPStreamAllocatesPort(t *testing.T) {
	rt := NewProtocol("127.0.0.1", 22222)
	port, err := rt.EnableUDPStream(0)
	if err != nil {
		t.Fatalf("enableudpstream: %v", err)
	}
	defer rt.Disconnect()
	if port == 0 {
		t.Fatal("expected a concrete port to be allocated")
	}
	if rt.UDPServerPort() != port {
		t.Errorf("UDPServerPort = %d, want %d", rt.UDPServerPort(), port)
	}
}

func TestReceiveUDPDecodesDatagram(t *testing.T) {
	rt := NewProtocol("127.0.0.1", 22222, WithReadTimeout(2*time.Second))
	port, err := rt.EnableUDPStream(0)
	if err != nil {
		t.Fatalf("enableudpstream: %v", err)
	}
	defer rt.Disconnect()

	frame := dataFrame(7, 8, component(ComponentType3D, marker3DPayload(2)))
	pkt := encodePacket(PacketTypeData, frame)

	conn, err := netDialUDP(port)
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write(pkt); err != nil {
		t.Fatalf("write: %v", err)
	}

	p, err := rt.ReceiveUDP()
	if err != nil {
		t.Fatalf("receiveudp: %v", err)
	}
	if !p.IsPacketData() {
		t.Fatalf("got packet type %v", p.Type)
	}
	if p.Data.Frame != 8 {
		t.Errorf("frame = %d, want 8", p.Data.Frame)
	}
	m, ok := p.Data.Component(ComponentType3D).(*packets.Component3D)
	if !ok || len(m.Markers) != 2 {
		t.Errorf("3D component = %v", p.Data.Component(ComponentType3D))
	}
}

// netDialUDP is a small helper kept separate to avoid importing net into the
// main body of this file's imports twice.
func netDialUDP(port int) (netConn, error) {
	return dialUDPLoopback(port)
}
