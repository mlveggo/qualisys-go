package qualisys

// func dialConnectionReused(ipaddress string) (net.Conn, error) {
// 	d := net.Dialer{
// 		Control: func(network, address string, c syscall.RawConn) error {
// 			var optErr error
// 			if err := c.Control(func(fd uintptr) {
// 				optErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
// 				if optErr != nil {
// 					return
// 				}
// 				optErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
// 				if optErr != nil {
// 					return
// 				}
// 			}); err != nil {
// 				return fmt.Errorf("dial: control failed: %w", err)
// 			}
// 			if optErr != nil {
// 				return fmt.Errorf("dial: set socket options failed: %w", optErr)
// 			}
// 			return nil
// 		},
// 	}
// 	conn, err := d.Dial("udp", ipaddress)
// 	if err != nil {
// 		return nil, fmt.Errorf("dial: %w", err)
// 	}
// 	return conn, nil
// }

// func openListeningConnectionReused(ctx context.Context, addr string) (*net.UDPConn, error) {
// 	lc := net.ListenConfig{
// 		Control: func(network, address string, c syscall.RawConn) error {
// 			var optErr error
// 			if err := c.Control(func(fd uintptr) {
// 				optErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
// 				if optErr != nil {
// 					return
// 				}
// 				optErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
// 				if optErr != nil {
// 					return
// 				}
// 			}); err != nil {
// 				return fmt.Errorf("control failed: %w", err)
// 			}
// 			if optErr != nil {
// 				return fmt.Errorf("set socket options failed: %w", optErr)
// 			}
// 			return nil
// 		},
// 	}
// 	lp, err := lc.ListenPacket(ctx, "udp", addr)
// 	if err != nil {
// 		return nil, fmt.Errorf("open connection, listen packet: %w", err)
// 	}
// 	udpConn, ok := lp.(*net.UDPConn)
// 	if !ok {
// 		return nil, fmt.Errorf("udpconn type conversion failed")
// 	}
// 	return udpConn, nil
// }
