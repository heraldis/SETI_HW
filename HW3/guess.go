package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const txID uint16 = 0x4242

func guessProtocol(protocol ProtocolType, ip net.IP, port uint16, timeout time.Duration) string {
	switch protocol {
	case TCP:
		if probeHTTP(ip, port, timeout) {
			return "HTTP"
		}
		if probeEcho("tcp4", ip, port, timeout) {
			return "ECHO"
		}
		if probeDNS("tcp4", ip, port, timeout) {
			return "DNS"
		}
	case UDP:
		if probeDNS("udp4", ip, port, timeout) {
			return "DNS"
		}
		if probeEcho("udp4", ip, port, timeout) {
			return "ECHO"
		}
	}
	return "-"
}

func dialProbe(network string, ip net.IP, port uint16, timeout time.Duration) (net.Conn, error) {
	addr := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	return conn, nil
}

func probeHTTP(ip net.IP, port uint16, timeout time.Duration) bool {
	conn, err := dialProbe("tcp4", ip, port, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	req := fmt.Sprintf("GET / HTTP/1.0\r\nHost: %s\r\n\r\n", ip.String())
	if _, err := conn.Write([]byte(req)); err != nil {
		return false
	}

	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	return strings.HasPrefix(string(buf[:n]), "HTTP/")
}

func probeEcho(network string, ip net.IP, port uint16, timeout time.Duration) bool {
	conn, err := dialProbe(network, ip, port, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	payload := []byte("echo test message 12345\n")
	if _, err := conn.Write(payload); err != nil {
		return false
	}

	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		return false
	}
	return bytes.Equal(buf, payload)
}

func buildDNSQuery(txID uint16) []byte {
	msg := make([]byte, 0, 29)
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint16(hdr[0:2], txID)
	binary.BigEndian.PutUint16(hdr[2:4], 0x0100)
	binary.BigEndian.PutUint16(hdr[4:6], 1)
	msg = append(msg, hdr...)

	for _, label := range []string{"example", "com"} {
		msg = append(msg, byte(len(label)))
		msg = append(msg, label...)
	}
	msg = append(msg, 0x00, 0x00, 0x01, 0x00, 0x01)
	return msg
}

func probeDNS(network string, ip net.IP, port uint16, timeout time.Duration) bool {
	conn, err := dialProbe(network, ip, port, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	query := buildDNSQuery(txID)

	var resp []byte
	if network == "tcp4" {
		framed := make([]byte, 2+len(query))
		binary.BigEndian.PutUint16(framed[0:2], uint16(len(query)))
		copy(framed[2:], query)
		if _, err := conn.Write(framed); err != nil {
			return false
		}

		lenBuf := make([]byte, 2)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return false
		}
		respLen := int(binary.BigEndian.Uint16(lenBuf))
		if respLen < 12 || respLen > 4096 {
			return false
		}
		resp = make([]byte, respLen)
		if _, err := io.ReadFull(conn, resp); err != nil {
			return false
		}
	} else {
		if _, err := conn.Write(query); err != nil {
			return false
		}
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil {
			return false
		}
		resp = buf[:n]
	}

	if len(resp) < 12 {
		return false
	}
	respID := binary.BigEndian.Uint16(resp[0:2])
	flags := binary.BigEndian.Uint16(resp[2:4])
	return respID == txID && (flags&0x8000) != 0
}
