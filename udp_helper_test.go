package qualisys

import "net"

type netConn interface {
	Write([]byte) (int, error)
	Close() error
}

func dialUDPLoopback(port int) (netConn, error) {
	return net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
}
