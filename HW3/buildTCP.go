package main

import (
	"encoding/binary"
	"net"
	"syscall"
)

func buildTCPPacket(srcIP, dstIP net.IP, srcPort, dstPort uint16, seq, ack uint32, flags uint8) []byte {
	packet := make([]byte, 20)

	binary.BigEndian.PutUint16(packet[0:2], srcPort)                                                  //src
	binary.BigEndian.PutUint16(packet[2:4], dstPort)                                                  //dst
	binary.BigEndian.PutUint32(packet[4:8], seq)                                                      //seq
	binary.BigEndian.PutUint32(packet[8:12], ack)                                                     //ack
	packet[12] = 5 << 4                                                                               //offset
	packet[13] = flags                                                                                //flags
	binary.BigEndian.PutUint16(packet[14:16], 65535)                                                  // win
	binary.BigEndian.PutUint16(packet[18:20], 0)                                                      // up
	binary.BigEndian.PutUint16(packet[16:18], getChecksum(srcIP, dstIP, syscall.IPPROTO_TCP, packet)) // checksum

	return packet
}

func parseTCPResponse(buf []byte) (uint16, uint16, bool) {
	if len(buf) < 20 {
		return 0, 0, false
	}

	srcPort := binary.BigEndian.Uint16(buf[0:2])
	dstPort := binary.BigEndian.Uint16(buf[2:4])
	// seqNum := binary.BigEndian.Uint32(buf[4:8])
	// ackNum := binary.BigEndian.Uint32(buf[8:12])
	flags := (buf[13])

	if (flags & (SYN | ACK)) == (SYN | ACK) {
		return srcPort, dstPort, true
	}

	return 0, 0, false
}
