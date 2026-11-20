package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"syscall"
)

func buildUDPPacket(srcIP, dstIP net.IP, srcPort, dstPort uint16, payload []byte) []byte {
	totalLen := 8 + len(payload)
	packet := make([]byte, totalLen)

	binary.BigEndian.PutUint16(packet[0:2], srcPort)
	binary.BigEndian.PutUint16(packet[2:4], dstPort)
	binary.BigEndian.PutUint16(packet[4:6], uint16(totalLen))
	copy(packet[8:], payload)
	binary.BigEndian.PutUint16(packet[6:8], getChecksum(srcIP, dstIP, syscall.IPPROTO_UDP, packet))

	return packet
}

func parseUDPResponse(buf []byte) (uint16, uint16, bool) {
	if len(buf) < 8 {
		return 0, 0, false
	}

	srcPort := binary.BigEndian.Uint16(buf[0:2])
	dstPort := binary.BigEndian.Uint16(buf[2:4])

	return srcPort, dstPort, true
}

func parseICMPUnreachable(buf []byte, expectedTargetIP net.IP) (uint16, uint16, bool) {
	if len(buf) < 28 {
		return 0, 0, false
	}
	if buf[0] != 3 || buf[1] != 3 {
		return 0, 0, false
	}

	headerLen := int(buf[8]&0x0f) * 4
	dstPort := buf[8+headerLen-4 : 8+headerLen]
	if !bytes.Equal(dstPort, expectedTargetIP) {
		return 0, 0, false
	}

	offset := 8 + headerLen
	origSrcPort := binary.BigEndian.Uint16(buf[offset : offset+2])
	origDstPort := binary.BigEndian.Uint16(buf[offset+2 : offset+4])

	return origSrcPort, origDstPort, true
}
