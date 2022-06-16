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

type Protocol struct {
	conn     net.Conn
	buffer   []byte
	ip       string
	basePort int
}

const DefaultLittleEndianPort = 22223

func NewProtocol(ip string, basePort int) *Protocol {
	const startBufferSize int = 4096
	rt := new(Protocol)
	rt.buffer = make([]byte, startBufferSize)
	rt.ip = ip
	rt.basePort = basePort
	return rt
}

func (rt *Protocol) Connect() error {
	if rt.IsConnected() {
		rt.Disconnect()
	}
	portLittleEndian := rt.basePort + 1
	ipAndPort := rt.ip + ":" + strconv.Itoa(portLittleEndian)
	raddr, err := net.ResolveTCPAddr("tcp", ipAndPort)
	if err != nil {
		return fmt.Errorf("connect: resolvetcpaddr: %w", err)
	}
	conn, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		return fmt.Errorf("connect: dial: %w", err)
	}
	rt.conn = conn
	p, err := rt.Receive()
	if err != nil {
		return fmt.Errorf("connect: receive: %w", err)
	}
	const qtmConnectedResponse string = "QTM RT Interface connected"
	if p.CommandResponse != qtmConnectedResponse {
		return fmt.Errorf("connect: unexpected response " + p.CommandResponse)
	}
	const (
		majorVer = 1
		minorVer = 22
	)
	err = rt.SetVersion(majorVer, minorVer)
	return err
}

func (rt *Protocol) IsConnected() bool {
	return rt.conn != nil
}

func (rt *Protocol) Disconnect() {
	if !rt.IsConnected() {
		return
	}
	rt.conn.Close()
	rt.conn = nil
}

func (rt *Protocol) Receive() (*Packet, error) {
	for i := range rt.buffer {
		rt.buffer[i] = 0
	}
	if err := rt.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		return nil, fmt.Errorf("receive: setreaddeadline: %w", err)
	}
	const packetHeaderSize = 8
	packetSize, err := rt.conn.Read(rt.buffer[0:packetHeaderSize])
	if err != nil {
		var netError net.Error
		if errors.As(err, &netError) && netError.Timeout() {
			return &Packet{Type: PacketTypeNoMoreData}, nil
		}
		return nil, fmt.Errorf("receive: read %w", err)
	}
	if packetSize < packetHeaderSize {
		return nil, fmt.Errorf("receive: packet to small for header")
	}
	var p Packet
	p.Size = int(binary.LittleEndian.Uint32(rt.buffer[0:4]))
	p.Type = PacketType(binary.LittleEndian.Uint32(rt.buffer[4:8]))
	if len(rt.buffer) < p.Size {
		rt.buffer = append(rt.buffer, make([]byte, p.Size)...)
	}
	if packetSize >= p.Size {
		return &p, nil
	}
	if err := rt.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		return nil, fmt.Errorf("receive: setreaddeadline: %w", err)
	}
	pos := packetSize
	for {
		n, err := rt.conn.Read(rt.buffer[pos:p.Size])
		if err != nil {
			var netError net.Error
			if errors.As(err, &netError) && netError.Timeout() {
				return &Packet{Type: PacketTypeNoMoreData}, nil
			}
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("receive: read %w", err)
			}
			break
		}
		pos += n
		if pos >= p.Size {
			break
		}
	}
	if err = p.UnmarshalBinary(rt.buffer[0:p.Size]); err != nil {
		return nil, fmt.Errorf("receive: unmarshalbinary %w", err)
	}
	if p.Type == PacketTypeError {
		return &p, fmt.Errorf("receive: error packet returned " + p.ErrorResponse)
	}
	return &p, nil
}
