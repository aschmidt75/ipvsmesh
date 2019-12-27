package socketfrontproxy

import (
	"io/ioutil"
	"net"
	"os"
	"strings"
)

func readInput(filename *string) ([]byte, error) {
	var b []byte
	var err error
	if *filename == "-" {
		b, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return b, err
		}
	} else {
		b, err = ioutil.ReadFile(*filename)
		if err != nil {
			return b, err
		}
	}

	return b, err
}

func dn(b byte) byte {
	if b >= '0' && b <= '9' {
		return b - '0'
	}
	if b >= 'A' && b <= 'F' {
		return 10 + (b - 'A')
	}
	if b >= 'a' && b <= 'f' {
		return 10 + (b - 'a')
	}
	return 0
}

func db(s string) byte {
	b := []byte(s)
	return dn(b[0])<<4 + dn(b[1])
}

func dw(s string) uint16 {
	return uint16(db(s[0:2]))<<8 + uint16(db(s[2:4]))
}

func dip4(s string) net.IP {
	return net.IPv4(db(s[6:8]), db(s[4:6]), db(s[2:4]), db(s[0:2]))
}

func dip6(s string) net.IP {
	return net.IP{db(s[30:32]), db(s[28:30]), db(s[26:28]), db(s[24:26]),
		db(s[22:24]), db(s[20:22]), db(s[18:20]), db(s[16:18]),
		db(s[14:16]), db(s[12:14]), db(s[10:12]), db(s[8:10]),
		db(s[6:8]), db(s[4:6]), db(s[2:4]), db(s[0:2])}
}

// Listener is an ip (4 or 6) plus port plus protocol (tcp/udp)
type Listener struct {
	ip    net.IP
	port  uint16
	proto string
}

func ParseProcNetTcpUdp(b []byte) []Listener {
	res := make([]Listener, 0)

	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		cols := strings.Split(strings.TrimSpace(line), " ")
		if len(cols) < 4 || cols[0] == "sl" || cols[3] != "0A" {
			// sl: skip title line
			// 0A: we only want listening ports
			continue
		}

		// split source address
		ipPortArr := strings.Split(cols[1], ":")
		if len(ipPortArr) != 2 {
			continue // not ip:port as expected
		}

		port := dw(ipPortArr[1])
		if len(ipPortArr[0]) == 8 {
			ip4 := dip4(ipPortArr[0])
			res = append(res, Listener{ip: ip4, port: port, proto: "tcp"})
		}
		if len(ipPortArr[0]) == 32 {
			ip6 := dip6(ipPortArr[0])
			res = append(res, Listener{ip: ip6, port: port, proto: "tcp"})
		}
	}

	return res
}

func ParseProcNetTcpUdpFromFile(filename string) ([]Listener, error) {
	b, err := readInput(&filename)
	if err != nil {
		return []Listener{}, err
	}
	return ParseProcNetTcpUdp(b), nil
}
