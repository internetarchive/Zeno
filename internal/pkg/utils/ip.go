package utils

import (
	"log"
	"net"
)

// Note: GetOutboundIP does not establish any connection and the
// destination does not need to exist for this function to work.
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "24.24.24.24:24200")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
