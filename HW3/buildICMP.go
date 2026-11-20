package main

import (
	"encoding/binary"
	"net"
)

const UNREACH = 3

func parseICMPPortUnreachable(buf []byte, expectedDstIP net.IP) (uint16, uint16, bool) {
	if len(buf) < 8+20+4 {
		return 0, 0, false
	}
	if buf[0] != UNREACH || buf[1] != UNREACH {
		return 0, 0, false
	}

	ipStart := 8
	ihl := int(buf[ipStart]&0x0f) * 4
	if ihl < 20 || len(buf) < ipStart+ihl+4 {
		return 0, 0, false
	}

	innerDst := net.IP(buf[ipStart+16 : ipStart+20]).To4()
	if innerDst == nil || !innerDst.Equal(expectedDstIP.To4()) {
		return 0, 0, false
	}

	udpStart := ipStart + ihl
	origSrcPort := binary.BigEndian.Uint16(buf[udpStart : udpStart+2])
	origDstPort := binary.BigEndian.Uint16(buf[udpStart+2 : udpStart+4])
	return origSrcPort, origDstPort, true
}
