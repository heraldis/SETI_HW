package main

import (
	"encoding/binary"
	"fmt"
	"net"
)

const (
	SYN byte = 0x02
	RST byte = 0x04
	ACK byte = 0x10
)

func getLocalIP(targetIP net.IP) (net.IP, error) {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: targetIP, Port: 80})
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr.IP.To4() == nil {
		return nil, fmt.Errorf("failed to get local address")
	}

	return localAddr.IP.To4(), nil
}

func getChecksum(srcIP, dstIP net.IP, protocol uint8, segment []byte) uint16 {
	header := make([]byte, 12)
	copy(header[0:4], srcIP.To4())
	copy(header[4:8], dstIP.To4())
	header[8] = 0
	header[9] = protocol
	binary.BigEndian.PutUint16(header[10:12], uint16(len(segment)))

	data := append(header, segment...)
	if len(data)%2 != 0 {
		data = append(data, 0)
	}

	var sum uint32
	for i := 0; i < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}

	for (sum >> 16) > 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}

	return ^uint16(sum)
}
