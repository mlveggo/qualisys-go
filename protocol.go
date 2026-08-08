package qualisys

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

//go:generate stringer -type ComponentType -trimprefix ComponentType
type ComponentType int

const (
	ComponentType3D ComponentType = iota + 1
	ComponentType3DNoLabels
	ComponentTypeAnalog
	ComponentTypeForce
	ComponentType6D
	ComponentType6DEuler
	ComponentType2D
	ComponentType2DLinearized
	ComponentType3DResidual
	ComponentType3DNoLabelsResidual
	ComponentType6DResidual
	ComponentType6DEulerResidual
	ComponentTypeAnalogSingle
	ComponentTypeImage
	ComponentTypeForceSingle
	ComponentTypeGazeVector
	ComponentTypeTimecode
	ComponentTypeSkeleton
	ComponentTypeEyeTracker
)

// Protocol version constants.
//
// DefaultMajorVersion/DefaultMinorVersion track the newest RT protocol version
// this SDK knows about, matching MAJOR_VERSION/MINOR_VERSION in the C++ SDK's
// RTPacket.h. MinSupportedMinorVersion is the oldest version Connect will
// negotiate down to.
const (
	DefaultMajorVersion      = 1
	DefaultMinorVersion      = 28
	MinSupportedMinorVersion = 22
)

// Port constants. QTM listens on base+1 for little-endian clients and base+2
// for big-endian clients; the base port itself speaks protocol version 1.0,
// which this SDK does not implement.
const (
	DefaultBasePort         = 22222
	DefaultLittleEndianPort = 22223
	DefaultBigEndianPort    = 22224
)

// Default timeouts.
const (
	DefaultReadTimeout    = 1 * time.Second
	DefaultConnectTimeout = 5 * time.Second
	// DefaultCommandTimeout applies to commands that wait for a response.
	DefaultCommandTimeout = 5 * time.Second
	// DefaultCalibrationTimeout matches cWaitForCalibrationTimeout in the C++
	// SDK. A camera calibration routinely takes minutes; the previous
	// hard-coded one second read deadline made Calibrate unusable.
	DefaultCalibrationTimeout = 10 * time.Minute
	// DefaultFileTimeout is how long to wait for a C3D or QTM file transfer.
	DefaultFileTimeout = 30 * time.Second
)

// DefaultMaxPacketSize caps how large a single packet may claim to be. QTM
// image and file packets are legitimately large, but an unbounded size field
// read straight off the wire is an easy way to make a client allocate itself to
// death.
const DefaultMaxPacketSize = 512 << 20 // 512 MiB

// Errors returned by the protocol. They are sentinels so callers can use
// errors.Is rather than matching on message text.
var (
	// ErrTimeout is returned when no packet arrived within the read timeout.
	ErrTimeout = errors.New("qualisys: receive timeout")
	// ErrNotConnected is returned by any operation that needs a live socket.
	ErrNotConnected = errors.New("qualisys: not connected")
	// ErrTruncated means a packet header was read but the body never fully
	// arrived. The stream is desynchronised at this point and the connection
	// must be re-established.
	ErrTruncated = errors.New("qualisys: packet truncated")
	// ErrVersionNotSupported means QTM rejected every protocol version this
	// SDK is willing to speak.
	ErrVersionNotSupported = errors.New("qualisys: no mutually supported protocol version")
)

const packetHeaderSize = 8

// Protocol is a client for the QTM real time protocol. It is not safe for
// concurrent use by multiple goroutines.
type Protocol struct {
	conn    net.Conn
	udpConn *net.UDPConn
	buffer  []byte

	ip       string
	basePort int

	// Negotiated protocol version, valid after a successful Connect.
	majorVersion int
	minorVersion int

	// Requested version and whether to fall back to older versions.
	wantMajor        int
	wantMinor        int
	negotiateVersion bool

	bigEndian bool
	order     binary.ByteOrder

	readTimeout    time.Duration
	connectTimeout time.Duration
	maxPacketSize  int

	// lastEvent tracks the most recent event packet seen, including events
	// swallowed while waiting for a command response.
	lastEvent EventType
	state     EventType
}

// Option configures a Protocol. Options are applied in NewProtocol.
type Option func(*Protocol)

// WithVersion requests a specific protocol version instead of the SDK default.
func WithVersion(major, minor int) Option {
	return func(p *Protocol) {
		p.wantMajor = major
		p.wantMinor = minor
	}
}

// WithoutVersionNegotiation disables falling back to older protocol versions.
// Connect then fails outright if QTM will not accept the requested version.
func WithoutVersionNegotiation() Option {
	return func(p *Protocol) { p.negotiateVersion = false }
}

// WithBigEndian selects the big-endian port and byte order. Most callers should
// not need this; it exists for parity with the C++ SDK.
func WithBigEndian() Option {
	return func(p *Protocol) {
		p.bigEndian = true
		p.order = binary.BigEndian
	}
}

// WithReadTimeout sets how long Receive waits for a packet to begin arriving.
func WithReadTimeout(d time.Duration) Option {
	return func(p *Protocol) { p.readTimeout = d }
}

// WithConnectTimeout sets the TCP dial timeout.
func WithConnectTimeout(d time.Duration) Option {
	return func(p *Protocol) { p.connectTimeout = d }
}

// WithMaxPacketSize caps the accepted packet size. Set this below the default
// if the client runs somewhere memory constrained.
func WithMaxPacketSize(n int) Option {
	return func(p *Protocol) { p.maxPacketSize = n }
}

// NewProtocol creates a client for the QTM instance at ip. basePort is QTM's
// base port, normally DefaultBasePort; the correct endian-specific port is
// derived from it.
func NewProtocol(ip string, basePort int, opts ...Option) *Protocol {
	const startBufferSize = 4096
	rt := &Protocol{
		buffer:           make([]byte, startBufferSize),
		ip:               ip,
		basePort:         basePort,
		wantMajor:        DefaultMajorVersion,
		wantMinor:        DefaultMinorVersion,
		negotiateVersion: true,
		order:            binary.LittleEndian,
		readTimeout:      DefaultReadTimeout,
		connectTimeout:   DefaultConnectTimeout,
		maxPacketSize:    DefaultMaxPacketSize,
		lastEvent:        EventTypeNone,
		state:            EventTypeNone,
	}
	for _, opt := range opts {
		opt(rt)
	}
	return rt
}

// port returns the TCP port to connect to for the configured byte order.
func (rt *Protocol) port() int {
	if rt.bigEndian {
		return rt.basePort + 2
	}
	return rt.basePort + 1
}

// versionCandidates builds the ordered list of protocol versions to try.
//
// This mirrors RTVersion::VersionList in the C++ SDK: the requested version
// first, then progressively older versions, skipping any that are newer than or
// equal to what was requested. The ladder stops at MinSupportedMinorVersion so
// that connecting to a QTM older than the documented floor fails cleanly
// instead of silently negotiating something this SDK cannot parse.
func (rt *Protocol) versionCandidates() [][2]int {
	candidates := [][2]int{{rt.wantMajor, rt.wantMinor}}
	if !rt.negotiateVersion {
		return candidates
	}
	for minor := DefaultMinorVersion; minor >= MinSupportedMinorVersion; minor-- {
		if rt.wantMajor == DefaultMajorVersion && minor >= rt.wantMinor {
			continue
		}
		candidates = append(candidates, [2]int{DefaultMajorVersion, minor})
	}
	return candidates
}

// Connect opens the TCP connection, reads QTM's welcome message and negotiates
// a protocol version.
//
// On any failure the socket is closed before returning, so IsConnected reports
// false and the Protocol can be reused for a retry. The previous implementation
// left a half-open connection behind on a failed handshake, so a caller looping
// on "if !IsConnected() { Connect() }" would spin forever against an
// incompatible QTM.
func (rt *Protocol) Connect() error {
	if rt.IsConnected() {
		rt.Disconnect()
	}

	addr := net.JoinHostPort(rt.ip, strconv.Itoa(rt.port()))
	conn, err := net.DialTimeout("tcp", addr, rt.connectTimeout)
	if err != nil {
		return fmt.Errorf("connect: dial %s: %w", addr, err)
	}
	rt.conn = conn

	p, err := rt.ReceiveTimeout(rt.connectTimeout)
	if err != nil {
		rt.Disconnect()
		return fmt.Errorf("connect: welcome: %w", err)
	}
	const qtmConnectedResponse = "QTM RT Interface connected"
	if p.CommandResponse != qtmConnectedResponse {
		rt.Disconnect()
		return fmt.Errorf("connect: unexpected welcome message (%q)", p.CommandResponse)
	}

	var lastErr error
	for _, v := range rt.versionCandidates() {
		if err := rt.SetVersion(v[0], v[1]); err != nil {
			lastErr = err
			continue
		}
		// Prime the cached state the same way the C++ SDK does after a
		// successful handshake. A failure here is not fatal.
		_, _ = rt.GetState()
		return nil
	}

	rt.Disconnect()
	if lastErr == nil {
		lastErr = ErrVersionNotSupported
	}
	return fmt.Errorf("connect: %w (tried %d.%d down to %d.%d): %v",
		ErrVersionNotSupported, rt.wantMajor, rt.wantMinor,
		DefaultMajorVersion, MinSupportedMinorVersion, lastErr)
}

// IsConnected reports whether a TCP connection is currently open.
func (rt *Protocol) IsConnected() bool {
	return rt.conn != nil
}

// Disconnect closes the TCP connection and any UDP stream socket.
func (rt *Protocol) Disconnect() {
	if rt.udpConn != nil {
		rt.udpConn.Close()
		rt.udpConn = nil
	}
	if rt.conn == nil {
		return
	}
	rt.conn.Close()
	rt.conn = nil
	rt.majorVersion = 0
	rt.minorVersion = 0
}

// Version returns the negotiated protocol version. It is only meaningful after
// a successful Connect.
func (rt *Protocol) Version() (major, minor int) {
	return rt.majorVersion, rt.minorVersion
}

// ParametersElementName returns the root element name used by the negotiated
// protocol version, for example "QTM_Parameters_Ver_1.28".
//
// Callers editing settings XML should use this rather than hard-coding a
// version, since the element name changes with every protocol revision.
func (rt *Protocol) ParametersElementName() string {
	return fmt.Sprintf("QTM_Parameters_Ver_%d.%d", rt.majorVersion, rt.minorVersion)
}

// State returns the most recent event QTM reported, including events observed
// while waiting for a command response.
func (rt *Protocol) State() EventType {
	return rt.state
}

// setReadDeadline applies d, or clears the deadline when d is zero or negative.
func (rt *Protocol) setReadDeadline(d time.Duration) error {
	if d <= 0 {
		return rt.conn.SetReadDeadline(time.Time{})
	}
	return rt.conn.SetReadDeadline(time.Now().Add(d))
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// Receive reads the next packet using the configured read timeout.
//
// When no data arrives within the timeout it returns a PacketTypeNoMoreData
// packet with a nil error, which is the contract the bundled examples poll on.
// A packet is always returned non-nil, even alongside an error, so callers that
// inspect the packet before checking the error do not panic on a nil pointer.
func (rt *Protocol) Receive() (*Packet, error) {
	return rt.ReceiveTimeout(rt.readTimeout)
}

// ReceiveTimeout reads the next packet, waiting at most d for it to start
// arriving. A non-positive d blocks indefinitely.
func (rt *Protocol) ReceiveTimeout(d time.Duration) (*Packet, error) {
	if !rt.IsConnected() {
		return &Packet{Type: PacketTypeNone}, ErrNotConnected
	}

	if err := rt.setReadDeadline(d); err != nil {
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receive: set deadline: %w", err)
	}

	// Read the fixed 8 byte header. io.ReadFull matters here: TCP is free to
	// deliver fewer than 8 bytes on the first read, and the previous code
	// treated any short read as a fatal "packet too small for header" error.
	if _, err := io.ReadFull(rt.conn, rt.buffer[:packetHeaderSize]); err != nil {
		if isTimeout(err) {
			return &Packet{Type: PacketTypeNoMoreData}, nil
		}
		if errors.Is(err, io.EOF) {
			return &Packet{Type: PacketTypeNone}, fmt.Errorf("receive: connection closed: %w", err)
		}
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receive: read header: %w", err)
	}

	size := int(rt.order.Uint32(rt.buffer[0:4]))
	ptype := PacketType(rt.order.Uint32(rt.buffer[4:8]))

	if size < packetHeaderSize {
		return &Packet{Type: PacketTypeNone},
			fmt.Errorf("receive: invalid packet size %d", size)
	}
	if size > rt.maxPacketSize {
		return &Packet{Type: PacketTypeNone},
			fmt.Errorf("receive: packet size %d exceeds limit %d", size, rt.maxPacketSize)
	}

	// A header-only packet carries no payload; PacketTypeNoMoreData arrives
	// this way.
	if size == packetHeaderSize {
		return &Packet{Size: size, Type: ptype, order: rt.order}, nil
	}

	if cap(rt.buffer) < size {
		rt.buffer = make([]byte, size)
	}
	rt.buffer = rt.buffer[:size]

	// Once the header is consumed the rest of the packet must arrive. A
	// timeout here means a desynchronised stream, not "no more data" --
	// reporting it as the latter, as the old code did, left the remaining
	// bytes in the socket to be misread as the next packet header.
	if err := rt.setReadDeadline(rt.readTimeout); err != nil {
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receive: set deadline: %w", err)
	}
	if _, err := io.ReadFull(rt.conn, rt.buffer[packetHeaderSize:size]); err != nil {
		if isTimeout(err) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return &Packet{Type: PacketTypeNone},
				fmt.Errorf("receive: %w: expected %d bytes: %v", ErrTruncated, size, err)
		}
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receive: read body: %w", err)
	}

	p := &Packet{order: rt.order}
	if err := p.UnmarshalBinary(rt.buffer[:size]); err != nil {
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receive: unmarshal: %w", err)
	}

	if p.Type == PacketTypeEvent {
		rt.lastEvent = p.Event
		// Camera settings changes are notifications rather than state
		// transitions, matching the C++ SDK's handling.
		if p.Event != EventTypeCameraSettingsChanged {
			rt.state = p.Event
		}
	}

	if p.Type == PacketTypeError {
		return p, fmt.Errorf("receive: error packet returned (%s)", p.ErrorResponse)
	}
	return p, nil
}

// receiveSkippingEvents reads until a non-event packet arrives or the deadline
// passes.
//
// QTM pushes events asynchronously, so a command response can be preceded by
// any number of event packets. The previous implementation treated whatever
// packet arrived first as the response, which made commands spuriously fail
// whenever an event happened to be in flight -- for example, TakeControl issued
// right after a capture started.
func (rt *Protocol) receiveSkippingEvents(timeout time.Duration) (*Packet, error) {
	deadline := time.Now().Add(timeout)
	for {
		var wait time.Duration
		if timeout > 0 {
			wait = time.Until(deadline)
			if wait <= 0 {
				return &Packet{Type: PacketTypeNone}, ErrTimeout
			}
		}
		p, err := rt.ReceiveTimeout(wait)
		if err != nil {
			return p, err
		}
		switch p.Type {
		case PacketTypeEvent:
			continue
		case PacketTypeNoMoreData:
			if timeout > 0 && !time.Now().Before(deadline) {
				return p, ErrTimeout
			}
			continue
		default:
			return p, nil
		}
	}
}
