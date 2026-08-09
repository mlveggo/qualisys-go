package qualisys

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeQTM is a minimal stand-in for a QTM RT server. Handler is invoked for
// each command string the client sends and returns raw packets to write back.
type fakeQTM struct {
	t        *testing.T
	listener net.Listener

	mu       sync.Mutex
	commands []string

	// welcome is sent as soon as a client connects.
	welcome []byte
	// handler produces the reply for a command. A nil reply sends nothing.
	handler func(cmd string) []byte
	// onConn, if set, takes over the connection entirely after the welcome.
	onConn func(conn net.Conn)
}

func encodePacket(t PacketType, payload []byte) []byte {
	size := packetHeaderSize + len(payload)
	b := make([]byte, size)
	binary.LittleEndian.PutUint32(b[0:4], uint32(size))
	binary.LittleEndian.PutUint32(b[4:8], uint32(t))
	copy(b[packetHeaderSize:], payload)
	return b
}

func commandPacket(s string) []byte  { return encodePacket(PacketTypeCommand, append([]byte(s), 0)) }
func errorPacket(s string) []byte    { return encodePacket(PacketTypeError, append([]byte(s), 0)) }
func xmlPacket(s string) []byte      { return encodePacket(PacketTypeXML, append([]byte(s), 0)) }
func eventPacket(e EventType) []byte { return encodePacket(PacketTypeEvent, []byte{byte(e)}) }

func newFakeQTM(t *testing.T) *fakeQTM {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeQTM{
		t:        t,
		listener: l,
		welcome:  commandPacket("QTM RT Interface connected"),
	}
	t.Cleanup(func() { l.Close() })
	return f
}

// start begins accepting connections. It is separate from newFakeQTM so a test
// can finish configuring the handler before any goroutine reads those fields.
func (f *fakeQTM) start() {
	go f.serve()
}

// basePort returns the value to pass to NewProtocol. The client connects to
// basePort+1, so the base is one below the port actually being listened on.
func (f *fakeQTM) basePort() int {
	return f.listener.Addr().(*net.TCPAddr).Port - 1
}

func (f *fakeQTM) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeQTM) handle(conn net.Conn) {
	defer conn.Close()
	if len(f.welcome) > 0 {
		if _, err := conn.Write(f.welcome); err != nil {
			return
		}
	}
	if f.onConn != nil {
		f.onConn(conn)
		return
	}
	header := make([]byte, packetHeaderSize)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		size := int(binary.LittleEndian.Uint32(header[0:4]))
		if size < packetHeaderSize {
			return
		}
		body := make([]byte, size-packetHeaderSize)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		cmd := strings.TrimRight(string(body), "\x00")

		f.mu.Lock()
		f.commands = append(f.commands, cmd)
		f.mu.Unlock()

		if f.handler == nil {
			continue
		}
		if reply := f.handler(cmd); reply != nil {
			if _, err := conn.Write(reply); err != nil {
				return
			}
		}
	}
}

func (f *fakeQTM) sentCommands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.commands))
	copy(out, f.commands)
	return out
}

// waitForCommand polls until cmd has been received or the deadline passes.
// Fire-and-forget commands such as StreamFrames send no reply, so the test has
// no response to synchronize on.
func (f *fakeQTM) waitForCommand(cmd string, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, c := range f.sentCommands() {
			if c == cmd {
				return true
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// acceptVersion builds a handler that accepts one specific protocol version and
// rejects every other, mimicking a QTM older than this SDK.
func acceptVersion(major, minor int) func(string) []byte {
	want := "Version " + itoa(major) + "." + itoa(minor)
	return func(cmd string) []byte {
		switch {
		case cmd == want:
			return commandPacket("Version set to " + itoa(major) + "." + itoa(minor))
		case strings.HasPrefix(cmd, "Version "):
			return errorPacket("Version not supported")
		case cmd == "GetState":
			return eventPacket(EventTypeConnected)
		}
		return nil
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func TestConnectNegotiatesDownToOlderVersion(t *testing.T) {
	// QTM only speaks 1.25. The SDK should walk down from its 1.28 default and
	// settle there, which is exactly what the previous hard-coded 1.22
	// handshake could not do.
	f := newFakeQTM(t)
	f.handler = acceptVersion(1, 25)
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	if err := rt.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rt.Disconnect()

	major, minor := rt.Version()
	if major != 1 || minor != 25 {
		t.Errorf("negotiated %d.%d, want 1.25", major, minor)
	}

	cmds := f.sentCommands()
	if len(cmds) == 0 || cmds[0] != "Version 1.28" {
		t.Errorf("first command = %q, want the newest version tried first", cmds)
	}
}

func TestConnectPrefersNewestAcceptedVersion(t *testing.T) {
	f := newFakeQTM(t)
	f.handler = acceptVersion(DefaultMajorVersion, DefaultMinorVersion)
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	if err := rt.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rt.Disconnect()

	major, minor := rt.Version()
	if major != DefaultMajorVersion || minor != DefaultMinorVersion {
		t.Errorf("negotiated %d.%d, want %d.%d", major, minor, DefaultMajorVersion, DefaultMinorVersion)
	}
	if n := len(f.sentCommands()); n > 2 {
		t.Errorf("sent %d commands, expected to stop after the first version was accepted", n)
	}
}

func TestConnectFailsCleanlyAgainstTooOldQTM(t *testing.T) {
	// A QTM below the supported floor rejects every version in the ladder.
	// Connect must fail and, critically, must leave the socket closed so a
	// reconnect loop does not spin forever believing it is connected.
	f := newFakeQTM(t)
	f.handler = acceptVersion(1, 15)
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	err := rt.Connect()
	if err == nil {
		t.Fatal("expected connect to fail against an unsupported QTM")
	}
	if !errors.Is(err, ErrVersionNotSupported) {
		t.Errorf("got %v, want ErrVersionNotSupported", err)
	}
	if rt.IsConnected() {
		t.Error("IsConnected is true after a failed handshake; the socket leaked")
	}
}

func TestWithoutVersionNegotiationTriesOnlyOne(t *testing.T) {
	f := newFakeQTM(t)
	f.handler = acceptVersion(1, 25)
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort(),
		WithVersion(1, 28), WithoutVersionNegotiation())
	if err := rt.Connect(); err == nil {
		t.Fatal("expected connect to fail with negotiation disabled")
	}
	if cmds := f.sentCommands(); len(cmds) != 1 {
		t.Errorf("sent %v, want exactly one version attempt", cmds)
	}
}

func TestReceiveHandlesHeaderSplitAcrossWrites(t *testing.T) {
	// TCP may deliver fewer than 8 bytes on the first read. The previous
	// implementation treated that as a fatal "packet too small for header".
	f := newFakeQTM(t)
	f.welcome = nil
	f.onConn = func(conn net.Conn) {
		pkt := commandPacket("QTM RT Interface connected")
		for _, chunk := range [][]byte{pkt[:3], pkt[3:6], pkt[6:]} {
			if _, err := conn.Write(chunk); err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(time.Second)
	}
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort(), WithReadTimeout(2*time.Second))
	conn, err := net.Dial("tcp", f.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	rt.conn = conn

	p, err := rt.Receive()
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if p.CommandResponse != "QTM RT Interface connected" {
		t.Errorf("got %q", p.CommandResponse)
	}
}

func TestReceiveReportsTruncatedPacket(t *testing.T) {
	// A packet header promising more data than ever arrives must be an error.
	// Returning NoMoreData here, as the old code did, left the partial body in
	// the socket to be misread as the next packet header.
	f := newFakeQTM(t)
	f.welcome = nil
	f.onConn = func(conn net.Conn) {
		full := commandPacket("this response never fully arrives")
		if _, err := conn.Write(full[:12]); err != nil { // header plus a few bytes only
			return
		}
		time.Sleep(2 * time.Second)
	}
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort(), WithReadTimeout(200*time.Millisecond))
	conn, err := net.Dial("tcp", f.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	rt.conn = conn

	_, err = rt.Receive()
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("got %v, want ErrTruncated", err)
	}
}

func TestReceiveRejectsAbsurdPacketSize(t *testing.T) {
	f := newFakeQTM(t)
	f.welcome = nil
	f.onConn = func(conn net.Conn) {
		b := make([]byte, packetHeaderSize)
		binary.LittleEndian.PutUint32(b[0:4], 0xFFFFFFF0)
		binary.LittleEndian.PutUint32(b[4:8], uint32(PacketTypeCommand))
		if _, err := conn.Write(b); err != nil {
			return
		}
		time.Sleep(time.Second)
	}
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort(), WithMaxPacketSize(1<<20))
	conn, err := net.Dial("tcp", f.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	rt.conn = conn

	if _, err := rt.Receive(); err == nil {
		t.Error("expected an error for a packet size beyond the configured limit")
	}
}

func TestReceiveTimeoutYieldsNoMoreData(t *testing.T) {
	f := newFakeQTM(t)
	f.welcome = nil
	f.onConn = func(_ net.Conn) { time.Sleep(time.Second) }
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort(), WithReadTimeout(50*time.Millisecond))
	conn, err := net.Dial("tcp", f.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	rt.conn = conn

	p, err := rt.Receive()
	if err != nil {
		t.Fatalf("a quiet socket should not be an error: %v", err)
	}
	if !p.EndOfData() {
		t.Errorf("got %v, want PacketTypeNoMoreData", p.Type)
	}
}

func TestCommandsSkipInterleavedEvents(t *testing.T) {
	// QTM pushes events asynchronously. The old implementation took the first
	// packet it saw as the command response, so an event arriving mid-command
	// made TakeControl and friends fail for no reason.
	f := newFakeQTM(t)
	f.handler = func(cmd string) []byte {
		switch {
		case strings.HasPrefix(cmd, "Version "):
			return commandPacket("Version set to 1.28")
		case cmd == "GetState":
			return eventPacket(EventTypeConnected)
		case cmd == "TakeControl":
			reply := eventPacket(EventTypeCaptureStarted)
			reply = append(reply, eventPacket(EventTypeWaitingForTrigger)...)
			reply = append(reply, commandPacket("You are now master")...)
			return reply
		}
		return nil
	}
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	if err := rt.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rt.Disconnect()

	if err := rt.TakeControl(""); err != nil {
		t.Fatalf("TakeControl should skip the interleaved events: %v", err)
	}
	if got := rt.State(); got != EventTypeWaitingForTrigger {
		t.Errorf("State = %v, want the last skipped event to be recorded", got)
	}
}

func TestTakeControlOmitsEmptyPassword(t *testing.T) {
	// "TakeControl " with a trailing space was previously sent when no password
	// was supplied.
	f := newFakeQTM(t)
	f.handler = func(cmd string) []byte {
		if strings.HasPrefix(cmd, "Version ") {
			return commandPacket("Version set to 1.28")
		}
		if cmd == "GetState" {
			return eventPacket(EventTypeConnected)
		}
		return commandPacket("You are now master")
	}
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	if err := rt.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rt.Disconnect()
	if err := rt.TakeControl(""); err != nil {
		t.Fatalf("takecontrol: %v", err)
	}

	if !f.waitForCommand("TakeControl", 2*time.Second) {
		t.Fatalf("commands = %v, want a bare TakeControl", f.sentCommands())
	}
	for _, c := range f.sentCommands() {
		if c == "TakeControl " {
			t.Error("sent TakeControl with a trailing space")
		}
	}
}

func TestGetParametersSkipsEvents(t *testing.T) {
	f := newFakeQTM(t)
	f.handler = func(cmd string) []byte {
		switch {
		case strings.HasPrefix(cmd, "Version "):
			return commandPacket("Version set to 1.28")
		case cmd == "GetState":
			return eventPacket(EventTypeConnected)
		case strings.HasPrefix(cmd, "GetParameters"):
			reply := eventPacket(EventTypeCameraSettingsChanged)
			reply = append(reply, xmlPacket("<QTM_Parameters_Ver_1.28><The_3D/></QTM_Parameters_Ver_1.28>")...)
			return reply
		}
		return nil
	}
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	if err := rt.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rt.Disconnect()

	xml, err := rt.GetParameters(ParameterType3D)
	if err != nil {
		t.Fatalf("getparameters: %v", err)
	}
	if !strings.Contains(xml, "The_3D") {
		t.Errorf("got %q", xml)
	}
}

func TestGetParametersSkeletonGlobalOption(t *testing.T) {
	f := newFakeQTM(t)
	f.handler = func(cmd string) []byte {
		switch {
		case strings.HasPrefix(cmd, "Version "):
			return commandPacket("Version set to 1.28")
		case cmd == "GetState":
			return eventPacket(EventTypeConnected)
		case strings.HasPrefix(cmd, "GetParameters"):
			return xmlPacket("<x/>")
		}
		return nil
	}
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	if err := rt.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rt.Disconnect()

	if _, err := rt.GetParametersWithOptions(
		ParameterOptions{SkeletonGlobal: true}, ParameterTypeSkeleton); err != nil {
		t.Fatalf("getparameters: %v", err)
	}

	if !f.waitForCommand("GetParameters Skeleton:global", 2*time.Second) {
		t.Errorf("commands = %v, want GetParameters Skeleton:global", f.sentCommands())
	}
}

func TestParametersElementNameTracksNegotiatedVersion(t *testing.T) {
	f := newFakeQTM(t)
	f.handler = acceptVersion(1, 24)
	f.start()

	rt := NewProtocol("127.0.0.1", f.basePort())
	if err := rt.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rt.Disconnect()

	if got := rt.ParametersElementName(); got != "QTM_Parameters_Ver_1.24" {
		t.Errorf("got %q, want QTM_Parameters_Ver_1.24", got)
	}

	xml := "<QTM_Parameters_Ver_1.24><The_6D/></QTM_Parameters_Ver_1.24>"
	if got := rt.StripParametersElement(xml); got != "<The_6D/>" {
		t.Errorf("strip returned %q", got)
	}
}

func TestOperationsOnClosedConnection(t *testing.T) {
	rt := NewProtocol("127.0.0.1", 22222)
	if _, err := rt.Receive(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("Receive: got %v, want ErrNotConnected", err)
	}
	if err := rt.StreamFramesAll(ComponentType3D); !errors.Is(err, ErrNotConnected) {
		t.Errorf("StreamFramesAll: got %v, want ErrNotConnected", err)
	}
	if _, err := rt.GetParameters(ParameterType3D); !errors.Is(err, ErrNotConnected) {
		t.Errorf("GetParameters: got %v, want ErrNotConnected", err)
	}
}

func TestReceiveReturnsNonNilPacketOnError(t *testing.T) {
	// The bundled examples inspect the returned packet before checking the
	// error, so a nil packet alongside an error would panic in user code.
	rt := NewProtocol("127.0.0.1", 22222)
	p, err := rt.Receive()
	if err == nil {
		t.Fatal("expected an error")
	}
	if p == nil {
		t.Fatal("Receive returned a nil packet alongside an error")
	}
	_ = p.EndOfData()
}
