package main

import (
	"encoding/binary"
	"math/rand/v2"
	"strings"
)

type DNSFlags uint16

const (
	SERVFAIL uint16 = 2
	NXDOMAIN uint16 = 3
	NOERROR  uint16 = 0
)

func sendMsg(q question) dnsMessage {
	return dnsMessage{
		Header: header{
			ID:       uint16(rand.IntN(65535)),
			Flags:    0,
			QDCount:  1,
			ANCount:  0,
			NSCCount: 0,
			ARCount:  0,
		},
		Questions: []question{q},
	}
}

func codeQuestionName(name string) []byte {
	result := []byte{}
	parts := strings.Split(name, ".")

	for _, part := range parts {
		result = append(result, byte(len(part)))
		result = append(result, []byte(part)...)
	}

	result = append(result, 0)
	return result
}

func decodeQuestionName(data []byte, offset int) ([]byte, int) {
	result := []byte{}
	curr := offset
	jumped := false
	nextOffset := 0

	for {
		if (data[curr] & 0xC0) == 0xC0 {
			ptr := int(binary.BigEndian.Uint16(data[curr:curr+2]) & 0x3FFF)

			if !jumped {
				nextOffset = curr + 2
				jumped = true
			}
			curr = ptr
			continue
		}

		length := int(data[curr])

		if length == 0 {
			if !jumped {
				nextOffset = curr + 1
			}
			break
		}
		if len(result) > 0 {
			result = append(result, '.')
		}

		curr++
		result = append(result, data[curr:curr+length]...)
		curr += length
	}

	return result, nextOffset
}

func buildErrorResponse(clientQuery *dnsMessage, flag uint16) dnsMessage {
	return dnsMessage{
		Header: header{
			ID:       clientQuery.Header.ID,
			Flags:    uint16(flag),
			QDCount:  1,
			ANCount:  0,
			NSCCount: 0,
			ARCount:  0,
		},
		Questions: clientQuery.Questions,
	}
}

func parseResourceRecords(data []byte, offset int, count int) ([]resourceRecord, int) {
	rrs := []resourceRecord{}
	currOffset := offset

	for range count {
		name, next := decodeQuestionName(data, currOffset)
		currOffset = next

		rr := resourceRecord{
			Name:  name,
			Type:  binary.BigEndian.Uint16(data[currOffset : currOffset+2]),
			Class: binary.BigEndian.Uint16(data[currOffset+2 : currOffset+4]),
			TTL:   binary.BigEndian.Uint32(data[currOffset+4 : currOffset+8]),
		}
		rdLen := binary.BigEndian.Uint16(data[currOffset+8 : currOffset+10])
		currOffset += 10

		rr.RDLength = rdLen
		rr.RData = data[currOffset : currOffset+int(rdLen)]
		currOffset += int(rdLen)

		rrs = append(rrs, rr)
	}

	return rrs, currOffset
}
