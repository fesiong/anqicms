package library

import (
	"net"
	"strconv"
	"time"
)

func ScanPort(protocol string, hostname string, port int) bool {
	p := strconv.Itoa(port)
	addr := net.JoinHostPort(hostname, p)
	conn, err := net.DialTimeout(protocol, addr, 1*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
