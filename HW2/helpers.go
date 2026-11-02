package main

import (
	"encoding/binary"
	"math/rand/v2"
	"strings"
)

const (
	NOERROR  uint16 = 0
	SERVFAIL uint16 = 2
	NXDOMAIN uint16 = 3
)

const ROOTSERVER string = "198.41.0.4"

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
	parts := strings.SplitSeq(name, ".")

	for part := range parts {
		result = append(result, byte(len(part)))
		result = append(result, []byte(part)...)
	}

	result = append(result, 0)
	return result
}

func decodeQuestionName(data []byte, offset int) ([]byte, int) {
	result := []string{}
	curr := offset

	if (data[curr] & 0xC0) == 0xC0 {
		nextOffset := curr + 2
		result = readLabels(data, curr, result)
		return []byte(strings.Join(result, ".")), nextOffset
	}

	for curr < len(data) {
		if (data[curr] & 0xC0) == 0xC0 {
			nextOffset := curr + 2
			result = readLabels(data, curr, result)
			return []byte(strings.Join(result, ".")), nextOffset
		}

		length := int(data[curr])
		if length == 0 {
			curr++
			break
		}
		result = append(result, string(data[curr+1:curr+1+length]))
		curr += 1 + length
	}

	return []byte(strings.Join(result, ".")), curr
}

func readLabels(data []byte, offset int, result []string) []string {
	curr := offset

	for curr < len(data) {
		if (data[curr] & 0xC0) == 0xC0 {
			curr = int(binary.BigEndian.Uint16(data[curr:curr+2]) & 0x3FFF)
			continue
		}

		length := int(data[curr])
		if length == 0 {
			break
		}

		result = append(result, string(data[curr+1:curr+1+length]))
		curr += 1 + length
	}

	return result
}

func buildErrorResponse(clientQuery *dnsMessage, flag uint16) dnsMessage {
	return dnsMessage{
		Header: header{
			ID:       clientQuery.Header.ID,
			Flags:    flag,
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
		name, nextOffset := decodeQuestionName(data, currOffset)
		currOffset = nextOffset

		rr := resourceRecord{
			Name:     name,
			Type:     binary.BigEndian.Uint16(data[currOffset : currOffset+2]),
			Class:    binary.BigEndian.Uint16(data[currOffset+2 : currOffset+4]),
			TTL:      binary.BigEndian.Uint32(data[currOffset+4 : currOffset+8]),
			RDLength: binary.BigEndian.Uint16(data[currOffset+8 : currOffset+10]),
		}

		rr.RData = data[currOffset+10 : currOffset+int(rr.RDLength)+10]
		currOffset += int(rr.RDLength) + 10

		rrs = append(rrs, rr)
	}

	return rrs, currOffset
}
