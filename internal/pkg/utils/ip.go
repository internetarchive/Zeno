package utils

import (
	"net"

	"github.com/sirupsen/logrus"
)

// Note: GetOutboundIP does not establish any connection and the
// destination does not need to exist for this function to work.
func GetOutboundIP() net.IP {
	var conn net.Conn

	for {
		conn, err := net.Dial("udp", "24.24.24.24:24200")
		if err != nil {
			logrus.Errorf("error getting outbound IP, retrying: %s", err)
			continue
		}
		defer conn.Close()
		break
	}

	return conn.LocalAddr().(*net.UDPAddr).IP
}
