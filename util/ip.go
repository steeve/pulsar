package util

import (
	"errors"
	"fmt"
	"net"

	"github.com/scakemyer/quasar/config"
)

func LocalIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			v4 := ip.To4()
			if v4 != nil && (v4[0] == 192 || v4[0] == 172 || v4[0] == 10) {
				return v4, nil
			}
		}
	}
	return nil, errors.New("cannot find local IP address")
}

func GetHTTPHost() string {
	hostname := "localhost"
	// if localIP, err := LocalIP(); err == nil {
	// 	hostname = localIP.String()
	// }
	return fmt.Sprintf("http://%s:%d", hostname, config.ListenPort)
}
