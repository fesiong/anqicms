package library

import (
	"net"
	"time"
)

func ScanPort(protocol string, hostname string, port string) bool {
	addr := net.JoinHostPort(hostname, port)
	conn, err := net.DialTimeout(protocol, addr, 1*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
