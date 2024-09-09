package discover

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func combineIPAndMask(n *net.IPNet) (net.IP, error) {
	if n.IP.To4() == nil {
		return net.IP{}, errors.New("no support for IPv6 addresses")
	}
	ip := make(net.IP, len(n.IP.To4()))
	binary.BigEndian.PutUint32(ip, binary.BigEndian.Uint32(n.IP.To4())|^binary.BigEndian.Uint32(net.IP(n.Mask).To4()))
	return ip, nil
}

func getBroadcastAddresses() ([]string, error) {
	var ips []string
	// list of system network interfaces
	// https://golang.org/pkg/net/#Interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips, err
	}
	// mapping between network interface name and index
	// https://golang.org/pkg/net/#Interface
	for _, intf := range interfaces {
		// skip down interface & check next intf
		if intf.Flags&net.FlagUp == 0 {
			continue
		}
		// skip loopback & check next intf
		if intf.Flags&net.FlagLoopback != 0 {
			continue
		}
		// list of unicast interface addresses for specific interface
		// https://golang.org/pkg/net/#Interface.Addrs
		addrs, err := intf.Addrs()
		if err != nil {
			return ips, err
		}
		// network end point address
		// https://golang.org/pkg/net/#Addr
		for _, addr := range addrs {
			var ip net.IP

			// Addr type switch required as a result of IPNet & IPAddr return in
			// https://golang.org/src/net/interface_windows.go?h=interfaceAddrTable
			switch v := addr.(type) {
			// net.IPNet satisfies Addr interface
			// since it contains Network() & String()
			// https://golang.org/pkg/net/#IPNet
			case *net.IPNet:
				ip = v.IP
			// net.IPAddr satisfies Addr interface
			// since it contains Network() & String()
			// https://golang.org/pkg/net/#IPAddr
			case *net.IPAddr:
				ip = v.IP
			}
			// skip loopback & check next addr
			if ip == nil || ip.IsLoopback() {
				continue
			}
			// convert IP IPv4 address to 4-byte
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			ipb, err := combineIPAndMask(addr.(*net.IPNet))
			if err == nil {
				// return IP address as string
				ips = append(ips, ipb.String())
			}
		}
	}
	return ips, nil
}

type Response struct {
	Address    string
	Hostname   string
	QtmVersion string
	Cameras    int
	BasePort   int
}

func (dr *Response) String() string {
	return fmt.Sprintf(
		"IP: %s\nHost: %s\nQTM Version: %s\nNumner of cameras: %d\nBase port: %d\n",
		dr.Address,
		dr.Hostname,
		dr.QtmVersion,
		dr.Cameras,
		dr.BasePort,
	)
}

type Discovery struct {
	receivePort uint16
	timeout     time.Duration
}

func NewDiscovery(receivePort uint16, timeout time.Duration) *Discovery {
	return &Discovery{receivePort: receivePort, timeout: timeout}
}

func (d *Discovery) Discover() ([]Response, error) {
	const broadcastPort string = "22226"
	ips, err := getBroadcastAddresses()
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenPacket("udp", ":"+strconv.Itoa(int(d.receivePort)))
	if err != nil {
		return nil, err
	}
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	data := make([]byte, 10)
	binary.LittleEndian.PutUint32(data, 10)
	binary.LittleEndian.PutUint32(data[4:8], 7)
	binary.BigEndian.PutUint16(data[8:10], d.receivePort)
	for _, ip := range ips {
		ipp := net.JoinHostPort(ip, broadcastPort)
		addr, err := net.ResolveUDPAddr("udp", ipp)
		if err != nil {
			return nil, fmt.Errorf("discover resolve udp address: %w", err)
		}
		_, err = conn.WriteTo(data, addr)
		if err != nil {
			return nil, fmt.Errorf("discover write to connection: %w", err)
		}
	}
	b := make([]byte, 1024)
	if err := conn.SetReadDeadline(time.Now().Add(d.timeout)); err != nil {
		return nil, fmt.Errorf("discover: setreaddeadline: %w", err)
	}
	responses := make([]Response, 0, 1)
	for {
		size, addr, err := conn.ReadFrom(b)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			var netError net.Error
			if errors.As(err, &netError) && netError.Timeout() {
				break
			}
			return nil, fmt.Errorf("discover: readfrom: %w", err)
		}
		ipAndPort := strings.Split(addr.String(), ":")
		dr := Response{Address: ipAndPort[0]}
		if err := dr.UnmarshalBinary(b[0:size]); err != nil {
			return nil, fmt.Errorf("discover: unmarshal: %w", err)
		}
		responses = append(responses, dr)
	}
	return responses, nil
}

func (dr *Response) UnmarshalBinary(data []byte) error {
	size := len(data)
	if size < 16 {
		return errors.New("too little data to UnmarshalBinary from")
	}
	info := string(data[8 : size-2])
	parts := splitAndTrimStrings(info, ",")
	if len(parts) != 3 {
		return errors.New("information part doesn't contain correct data")
	}
	dr.Hostname = parts[0]
	dr.QtmVersion = parts[1]
	if cameras, err := strconv.Atoi(strings.Split(parts[2], " ")[0]); err == nil {
		dr.Cameras = cameras
	}
	dr.BasePort = int(binary.BigEndian.Uint16(data[size-2 : size]))
	return nil
}

func splitAndTrimStrings(s, sep string) []string {
	parts := strings.Split(s, sep)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
