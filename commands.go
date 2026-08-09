package qualisys

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type senderType func(string) error

// sendString frames and writes a null-terminated string packet.
//
// The header is written in the connection's byte order. The previous
// implementation always wrote a little-endian header, which meant a big-endian
// connection could receive but never successfully send.
func (rt *Protocol) sendString(s string, t PacketType) error {
	if !rt.IsConnected() {
		return ErrNotConnected
	}
	dataSize := len(s) + packetHeaderSize + 1
	data := make([]byte, dataSize)
	rt.order.PutUint32(data[0:4], uint32(dataSize))
	rt.order.PutUint32(data[4:8], uint32(t))
	copy(data[packetHeaderSize:], s)
	// The final byte is already zero, providing the terminator.
	if _, err := rt.conn.Write(data); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	return nil
}

func (rt *Protocol) sendCommand(cmd string) error {
	if err := rt.sendString(cmd, PacketTypeCommand); err != nil {
		return fmt.Errorf("sendcommand: %w", err)
	}
	return nil
}

func (rt *Protocol) sendXML(cmd string) error {
	if err := rt.sendString(cmd, PacketTypeXML); err != nil {
		return fmt.Errorf("sendxml: %w", err)
	}
	return nil
}

// SendCommand sends a raw command and returns QTM's response string. It is
// exposed so callers can reach protocol features this SDK has not wrapped yet.
func (rt *Protocol) SendCommand(cmd string) (string, error) {
	if err := rt.sendCommand(cmd); err != nil {
		return "", err
	}
	p, err := rt.receiveSkippingEvents(DefaultCommandTimeout)
	if err != nil {
		return "", fmt.Errorf("sendcommand %q: %w", cmd, err)
	}
	return p.CommandResponse, nil
}

// SetVersion negotiates a specific protocol version. On success the negotiated
// version is recorded and returned by Version.
func (rt *Protocol) SetVersion(major, minor int) error {
	ver := strconv.Itoa(major) + "." + strconv.Itoa(minor)
	cmd := "Version " + ver
	qtmResponses := []string{"Version set to " + ver}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("setversion %s: %w", ver, err)
	}
	rt.majorVersion = major
	rt.minorVersion = minor
	return nil
}

// GetQTMVersion returns the QTM application version string.
func (rt *Protocol) GetQTMVersion() (string, error) {
	resp, err := rt.SendCommand("QTMVersion")
	if err != nil {
		return "", fmt.Errorf("getqtmversion: %w", err)
	}
	return resp, nil
}

// GetByteOrder asks QTM which byte order the current connection uses.
func (rt *Protocol) GetByteOrder() (bigEndian bool, err error) {
	resp, err := rt.SendCommand("ByteOrder")
	if err != nil {
		return false, fmt.Errorf("getbyteorder: %w", err)
	}
	return resp == "Byte order is big endian", nil
}

// CheckLicense validates a license code against QTM.
func (rt *Protocol) CheckLicense(licenseCode string) error {
	resp, err := rt.SendCommand("CheckLicense " + licenseCode)
	if err != nil {
		return fmt.Errorf("checklicense: %w", err)
	}
	if resp != "License pass" {
		return fmt.Errorf("checklicense: rejected (%s)", resp)
	}
	return nil
}

// GetState asks QTM for its current state and returns the resulting event.
//
// The previous implementation only sent the command and left the reply for
// whoever called Receive next, so there was no way to actually read the state.
func (rt *Protocol) GetState() (EventType, error) {
	cmd := "GetState"
	if rt.majorVersion == 1 && rt.minorVersion <= 9 {
		cmd = "GetLastEvent"
	}
	if err := rt.sendCommand(cmd); err != nil {
		return EventTypeNone, fmt.Errorf("getstate: %w", err)
	}
	deadline := time.Now().Add(DefaultCommandTimeout)
	for time.Now().Before(deadline) {
		p, err := rt.ReceiveTimeout(time.Until(deadline))
		if err != nil {
			return EventTypeNone, fmt.Errorf("getstate: %w", err)
		}
		if p.Type == PacketTypeEvent {
			return p.Event, nil
		}
	}
	return EventTypeNone, fmt.Errorf("getstate: %w", ErrTimeout)
}

//go:generate stringer -type StreamRateType -trimprefix StreamRateType
type StreamRateType int

const (
	StreamRateTypeAllFrames StreamRateType = iota
	StreamRateTypeFrequency
	StreamRateTypeFrequencyDivisor
)

// ComponentOptions carries the per-component modifiers the RT protocol accepts
// after a colon. Both were previously unsupported, and were listed as known
// gaps in the project README.
type ComponentOptions struct {
	// AnalogChannels limits Analog and AnalogSingle streams to specific
	// channels, for example "1,3,5-8". Empty means all channels.
	AnalogChannels string
	// SkeletonGlobal requests skeleton segment positions and rotations in the
	// global coordinate system rather than relative to the parent segment.
	SkeletonGlobal bool
}

// componentName returns the token QTM accepts on the wire for a component, and
// whether the component is known at all.
//
// ComponentType3DNoLabelsResidual previously mapped to "3dNoLabelsResidual",
// which QTM does not recognize -- the accepted token is "3DNoLabelsRes", so
// requesting that component silently produced no data.
func componentName(c ComponentType) (string, bool) {
	switch c {
	case ComponentType3D:
		return "3D", true
	case ComponentType3DNoLabels:
		return "3DNoLabels", true
	case ComponentTypeAnalog:
		return "Analog", true
	case ComponentTypeForce:
		return "Force", true
	case ComponentType6D:
		return "6D", true
	case ComponentType6DEuler:
		return "6DEuler", true
	case ComponentType2D:
		return "2D", true
	case ComponentType2DLinearized:
		return "2DLin", true
	case ComponentType3DResidual:
		return "3DRes", true
	case ComponentType3DNoLabelsResidual:
		return "3DNoLabelsRes", true
	case ComponentType6DResidual:
		return "6DRes", true
	case ComponentType6DEulerResidual:
		return "6DEulerRes", true
	case ComponentTypeAnalogSingle:
		return "AnalogSingle", true
	case ComponentTypeImage:
		return "Image", true
	case ComponentTypeForceSingle:
		return "ForceSingle", true
	case ComponentTypeGazeVector:
		return "GazeVector", true
	case ComponentTypeTimecode:
		return "Timecode", true
	case ComponentTypeSkeleton:
		return "Skeleton", true
	case ComponentTypeEyeTracker:
		return "EyeTracker", true
	}
	return "", false
}

// componentString renders a component list with any applicable options.
func componentString(opts ComponentOptions, components ...ComponentType) (string, error) {
	parts := make([]string, 0, len(components))
	for _, c := range components {
		name, ok := componentName(c)
		if !ok {
			return "", fmt.Errorf("unknown component type %d", int(c))
		}
		switch c {
		case ComponentTypeAnalog, ComponentTypeAnalogSingle:
			if opts.AnalogChannels != "" {
				name += ":" + opts.AnalogChannels
			}
		case ComponentTypeSkeleton:
			if opts.SkeletonGlobal {
				name += ":global"
			}
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, " "), nil
}

// GetCurrentFrame requests a single frame containing the given components.
func (rt *Protocol) GetCurrentFrame(components ...ComponentType) error {
	return rt.GetCurrentFrameWithOptions(ComponentOptions{}, components...)
}

// GetCurrentFrameWithOptions requests a single frame with component options.
func (rt *Protocol) GetCurrentFrameWithOptions(opts ComponentOptions, components ...ComponentType) error {
	cs, err := componentString(opts, components...)
	if err != nil {
		return fmt.Errorf("getcurrentframe: %w", err)
	}
	if err := rt.sendCommand("GetCurrentFrame " + cs); err != nil {
		return fmt.Errorf("getcurrentframe: %w", err)
	}
	return nil
}

// StreamFramesAll streams every frame over the existing TCP connection.
func (rt *Protocol) StreamFramesAll(components ...ComponentType) error {
	return rt.StreamFrames(StreamRateTypeAllFrames, 0, components...)
}

// StreamFrames starts streaming over the existing TCP connection.
func (rt *Protocol) StreamFrames(rate StreamRateType, value int, components ...ComponentType) error {
	return rt.StreamFramesWithOptions(rate, value, ComponentOptions{}, components...)
}

// StreamFramesWithOptions starts streaming with per-component options.
func (rt *Protocol) StreamFramesWithOptions(
	rate StreamRateType,
	value int,
	opts ComponentOptions,
	components ...ComponentType,
) error {
	return rt.streamFrames(rate, value, 0, "", opts, components...)
}

// StreamFramesUDP starts streaming data frames to a UDP endpoint while keeping
// commands and responses on the TCP connection.
//
// udpAddr may be empty, in which case QTM sends to the address the TCP
// connection came from. Use EnableUDPStream to have the SDK open a receiving
// socket and then read frames with ReceiveUDP.
func (rt *Protocol) StreamFramesUDP(
	rate StreamRateType,
	value, udpPort int,
	udpAddr string,
	opts ComponentOptions,
	components ...ComponentType,
) error {
	if udpPort <= 0 || udpPort > 65535 {
		return fmt.Errorf("streamframesudp: invalid udp port %d", udpPort)
	}
	if len(udpAddr) > 64 {
		return fmt.Errorf("streamframesudp: udp address too long")
	}
	return rt.streamFrames(rate, value, udpPort, udpAddr, opts, components...)
}

func (rt *Protocol) streamFrames(
	rate StreamRateType,
	value, udpPort int,
	udpAddr string,
	opts ComponentOptions,
	components ...ComponentType,
) error {
	var b strings.Builder
	b.WriteString("StreamFrames")
	switch rate {
	case StreamRateTypeAllFrames:
		b.WriteString(" AllFrames")
	case StreamRateTypeFrequency:
		b.WriteString(" Frequency:" + strconv.Itoa(value))
	case StreamRateTypeFrequencyDivisor:
		b.WriteString(" FrequencyDivisor:" + strconv.Itoa(value))
	default:
		return fmt.Errorf("streamframes: invalid rate type %d", int(rate))
	}

	if udpPort > 0 {
		b.WriteString(" UDP")
		if udpAddr != "" {
			b.WriteString(":" + udpAddr)
		}
		b.WriteString(":" + strconv.Itoa(udpPort))
	}

	cs, err := componentString(opts, components...)
	if err != nil {
		return fmt.Errorf("streamframes: %w", err)
	}
	if cs == "" {
		return fmt.Errorf("streamframes: no components requested")
	}
	b.WriteString(" " + cs)

	if err := rt.sendCommand(b.String()); err != nil {
		return fmt.Errorf("streamframes: %w", err)
	}
	return nil
}

func (rt *Protocol) StreamFramesStop() error {
	if err := rt.sendCommand("StreamFrames Stop"); err != nil {
		return fmt.Errorf("streamframesstop: %w", err)
	}
	return nil
}

// EnableUDPStream opens a UDP socket for receiving streamed data. Pass port 0
// to let the operating system choose; the chosen port is returned and should be
// passed to StreamFramesUDP.
func (rt *Protocol) EnableUDPStream(port int) (int, error) {
	if rt.udpConn != nil {
		rt.udpConn.Close()
		rt.udpConn = nil
	}
	addr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, fmt.Errorf("enableudpstream: listen: %w", err)
	}
	rt.udpConn = conn
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		conn.Close()
		rt.udpConn = nil
		return 0, fmt.Errorf("enableudpstream: unexpected local address type")
	}
	return local.Port, nil
}

// UDPServerPort returns the local port of the UDP stream socket, or 0.
func (rt *Protocol) UDPServerPort() int {
	if rt.udpConn == nil {
		return 0
	}
	if local, ok := rt.udpConn.LocalAddr().(*net.UDPAddr); ok {
		return local.Port
	}
	return 0
}

// maxUDPDatagramSize is the largest a UDP payload can be, so a single read
// can never truncate a datagram.
const maxUDPDatagramSize = 65536

// ReceiveUDP reads one datagram from the UDP stream socket and decodes it.
//
// Each QTM UDP datagram carries exactly one complete packet, so unlike the TCP
// path there is no reassembly to do.
func (rt *Protocol) ReceiveUDP() (*Packet, error) {
	if rt.udpConn == nil {
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receiveudp: %w: call EnableUDPStream first", ErrNotConnected)
	}
	if rt.readTimeout > 0 {
		if err := rt.udpConn.SetReadDeadline(time.Now().Add(rt.readTimeout)); err != nil {
			return &Packet{Type: PacketTypeNone}, fmt.Errorf("receiveudp: set deadline: %w", err)
		}
	}
	// Reused across calls: a fresh 64 KiB per frame is real GC pressure at
	// streaming rates. Every decoder copies what it keeps -- see
	// packets.cursor.Bytes -- so overwriting this on the next call is safe.
	if rt.udpBuffer == nil {
		rt.udpBuffer = make([]byte, maxUDPDatagramSize)
	}
	n, _, err := rt.udpConn.ReadFromUDP(rt.udpBuffer)
	if err != nil {
		if isTimeout(err) {
			return &Packet{Type: PacketTypeNoMoreData}, nil
		}
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receiveudp: read: %w", err)
	}
	if n < packetHeaderSize {
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receiveudp: datagram too short (%d bytes)", n)
	}
	p := &Packet{order: rt.order}
	if err := p.UnmarshalBinary(rt.udpBuffer[:n]); err != nil {
		return &Packet{Type: PacketTypeNone}, fmt.Errorf("receiveudp: unmarshal: %w", err)
	}
	return p, nil
}

func (rt *Protocol) TakeControl(password string) error {
	cmd := "TakeControl"
	if password != "" {
		cmd += " " + password
	}
	qtmResponses := []string{"You are now master", "You are already master"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("takecontrol: %w", err)
	}
	return nil
}

func (rt *Protocol) ReleaseControl() error {
	qtmResponses := []string{"You are now a regular client", "You are already a regular client"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "ReleaseControl", qtmResponses); err != nil {
		return fmt.Errorf("releasecontrol: %w", err)
	}
	return nil
}

func (rt *Protocol) New() error {
	qtmResponses := []string{"Creating new connection", "Already connected"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "New", qtmResponses); err != nil {
		return fmt.Errorf("new: %w", err)
	}
	return nil
}

func (rt *Protocol) Close() error {
	qtmResponses := []string{"Closing connection", "File closed", "Closing file", "No connection to close"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "Close", qtmResponses); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}

func (rt *Protocol) Start(rtFromFile bool) error {
	cmd := "Start"
	if rtFromFile {
		cmd += " RTFromFile"
	}
	qtmResponses := []string{"Starting measurement", "Starting RT from file"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

func (rt *Protocol) Stop() error {
	qtmResponses := []string{"Stopping measurement"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "Stop", qtmResponses); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return nil
}

func (rt *Protocol) Load(filename string) error {
	qtmResponses := []string{"Measurement loaded"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "Load "+filename, qtmResponses); err != nil {
		return fmt.Errorf("load: %w", err)
	}
	return nil
}

func (rt *Protocol) Save(filename string, overwrite bool) error {
	cmd := "Save " + filename
	if overwrite {
		cmd += " Overwrite"
	}
	qtmResponses := []string{"Measurement saved", "Measurement saved as " + filename}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func (rt *Protocol) LoadProject(path string) error {
	qtmResponses := []string{"Project loaded"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "LoadProject "+path, qtmResponses); err != nil {
		return fmt.Errorf("loadproject: %w", err)
	}
	return nil
}

// GetCaptureC3D downloads the current capture as a C3D file.
//
// The previous version only sent the command and returned; the file packet that
// followed was left for an unsuspecting Receive caller, and even then
// FilePacket decoding discarded the content. This waits for the transfer and
// returns the bytes.
func (rt *Protocol) GetCaptureC3D() (*FilePacket, error) {
	return rt.getCapture("GetCaptureC3D", PacketTypeC3DFile)
}

// GetCaptureQTM downloads the current capture as a QTM file.
func (rt *Protocol) GetCaptureQTM() (*FilePacket, error) {
	return rt.getCapture("GetCaptureQTM", PacketTypeQTMFile)
}

func (rt *Protocol) getCapture(cmd string, want PacketType) (*FilePacket, error) {
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, []string{"Sending capture"}); err != nil {
		return nil, fmt.Errorf("%s: %w", strings.ToLower(cmd), err)
	}
	p, err := rt.receiveSkippingEvents(DefaultFileTimeout)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", strings.ToLower(cmd), err)
	}
	if p.Type != want {
		return nil, fmt.Errorf("%s: expected packet type %v, got %v", strings.ToLower(cmd), want, p.Type)
	}
	file := p.File
	return &file, nil
}

// SaveCaptureC3D downloads the current capture and writes it to path.
func (rt *Protocol) SaveCaptureC3D(path string) error {
	f, err := rt.GetCaptureC3D()
	if err != nil {
		return err
	}
	return f.WriteFile(path)
}

// SaveCaptureQTM downloads the current capture and writes it to path.
func (rt *Protocol) SaveCaptureQTM(path string) error {
	f, err := rt.GetCaptureQTM()
	if err != nil {
		return err
	}
	return f.WriteFile(path)
}

func (rt *Protocol) Trig() error {
	qtmResponses := []string{"Trig ok"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "Trig", qtmResponses); err != nil {
		return fmt.Errorf("trig: %w", err)
	}
	return nil
}

// SetQTMEvent inserts a labeled event into the current measurement.
func (rt *Protocol) SetQTMEvent(label string) error {
	// The command was renamed from "Event" in protocol version 1.8.
	cmd := "SetQTMEvent "
	if rt.majorVersion == 1 && rt.minorVersion <= 7 {
		cmd = "Event "
	}
	qtmResponses := []string{"Event set"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd+label, qtmResponses); err != nil {
		return fmt.Errorf("setqtmevent: %w", err)
	}
	return nil
}

func (rt *Protocol) Reprocess() error {
	qtmResponses := []string{"Reprocessing file"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "Reprocess", qtmResponses); err != nil {
		return fmt.Errorf("reprocess: %w", err)
	}
	return nil
}

// Calibrate starts a camera calibration and waits for the resulting
// calibration XML.
//
// The old implementation returned as soon as QTM acknowledged the command, so
// the calibration result was never read and the next Receive would return it
// unexpectedly. Calibration also takes minutes, far longer than the one second
// read deadline that used to apply, so it could not have completed anyway.
func (rt *Protocol) Calibrate(refine bool, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = DefaultCalibrationTimeout
	}
	cmd := "Calibrate"
	if refine {
		cmd += " Refine"
	}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, []string{"Starting calibration"}); err != nil {
		return "", fmt.Errorf("calibrate: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p, err := rt.ReceiveTimeout(time.Until(deadline))
		if err != nil {
			return "", fmt.Errorf("calibrate: %w", err)
		}
		switch p.Type {
		case PacketTypeXML:
			return p.XMLResponse, nil
		case PacketTypeEvent:
			if p.Event == EventTypeConnectionClosed {
				return "", fmt.Errorf("calibrate: connection closed during calibration")
			}
		}
	}
	return "", fmt.Errorf("calibrate: %w waiting for calibration result", ErrTimeout)
}

//go:generate stringer -type LedMode -trimprefix LedMode
type LedMode uint8

const (
	LedModeOn LedMode = iota
	LedModeOff
	LedModePulsing
)

//go:generate stringer -type LedColor -trimprefix LedColor
type LedColor uint8

const (
	LedColorAmber LedColor = iota
	LedColorGreen
	LedColorAll
)

func (rt *Protocol) Led(cameraNumber int, mode LedMode, color LedColor) error {
	cmd := "Led " + strconv.Itoa(cameraNumber) + " " + mode.String() + " " + color.String()
	if err := rt.sendCommand(cmd); err != nil {
		return fmt.Errorf("led: %w", err)
	}
	return nil
}

func (rt *Protocol) Quit() error {
	qtmResponses := []string{"Bye bye"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, "Quit", qtmResponses); err != nil {
		return fmt.Errorf("quit: %w", err)
	}
	return nil
}

// sendAndWaitForResponse sends a string and waits for one of the expected
// command responses, skipping any event packets that arrive first.
func (rt *Protocol) sendAndWaitForResponse(sender senderType, s string, expectedResponses []string) error {
	if err := sender(s); err != nil {
		return err
	}
	p, err := rt.receiveSkippingEvents(DefaultCommandTimeout)
	if err != nil {
		return err
	}
	for _, r := range expectedResponses {
		if strings.EqualFold(p.CommandResponse, r) {
			return nil
		}
	}
	if p.CommandResponse == "" && p.ErrorResponse != "" {
		return fmt.Errorf("unexpected error response (%s)", p.ErrorResponse)
	}
	return fmt.Errorf("unexpected response (%s), wanted one of %v", p.CommandResponse, expectedResponses)
}
