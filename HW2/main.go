package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func packMessage(msg dnsMessage) []byte {
	resHeader := make([]byte, 12)
	binary.BigEndian.PutUint16(resHeader[0:2], msg.Header.ID)
	binary.BigEndian.PutUint16(resHeader[2:4], msg.Header.Flags)
	binary.BigEndian.PutUint16(resHeader[4:6], msg.Header.QDCount)
	binary.BigEndian.PutUint16(resHeader[6:8], msg.Header.ANCount)
	binary.BigEndian.PutUint16(resHeader[8:10], msg.Header.NSCCount)
	binary.BigEndian.PutUint16(resHeader[10:12], msg.Header.ARCount)

	name := codeQuestionName(string(msg.Questions[0].Name))
	nameLen := len(name)
	resQuestion := make([]byte, 4+nameLen)
	copy(resQuestion[0:len(name)], name)
	binary.BigEndian.PutUint16(resQuestion[nameLen:nameLen+2], msg.Questions[0].Type)
	binary.BigEndian.PutUint16(resQuestion[nameLen+2:nameLen+4], msg.Questions[0].Class)

	return append(resHeader, resQuestion...)
}

func unpackMessage(data []byte) *dnsMessage {
	msg := &dnsMessage{}
	msg.Header = header{
		ID:       binary.BigEndian.Uint16(data[0:2]),
		Flags:    binary.BigEndian.Uint16(data[2:4]),
		QDCount:  binary.BigEndian.Uint16(data[4:6]),
		ANCount:  binary.BigEndian.Uint16(data[6:8]),
		NSCCount: binary.BigEndian.Uint16(data[8:10]),
		ARCount:  binary.BigEndian.Uint16(data[10:12]),
	}

	offset := 12
	for i := 0; i < int(msg.Header.QDCount); i++ {
		name, nextOffset := decodeQuestionName(data, offset)
		offset = nextOffset

		q := question{
			Name:  name,
			Type:  binary.BigEndian.Uint16(data[offset : offset+2]),
			Class: binary.BigEndian.Uint16(data[offset+2 : offset+4]),
		}
		offset += 4
		msg.Questions = append(msg.Questions, q)
	}

	msg.Answers, offset = parseResourceRecords(data, offset, int(msg.Header.ANCount))
	msg.Authorities, offset = parseResourceRecords(data, offset, int(msg.Header.NSCCount))
	msg.Additionals, offset = parseResourceRecords(data, offset, int(msg.Header.ARCount))

	return msg
}

func askDomain(dialer *net.Dialer, timeout context.Context, ns string, question question) (*dnsMessage, []byte, error) {
	conn, err := dialer.DialContext(timeout, "udp", net.JoinHostPort(ns, "53"))
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()

	deadline, _ := timeout.Deadline()
	conn.SetDeadline(deadline)

	n, err := conn.Write(packMessage(sendMsg(question)))
	if err != nil {
		return nil, nil, err
	}

	respBuff := make([]byte, 2048)
	n, err = conn.Read(respBuff)
	if err != nil {
		return nil, nil, err
	}

	return unpackMessage(respBuff[:n]), respBuff[:n], nil
}

func isAnswer(answers []resourceRecord, ttype uint16) bool {
	for _, ans := range answers {
		if ans.Type == ttype {
			return true
		}
	}
	return false
}

func resolve(dialer *net.Dialer, clientQuery *dnsMessage) []byte {
	timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	currentNS := []string{ROOTSERVER}

	for timeout.Err() == nil {
		nextNS := []string{}
		for _, ns := range currentNS {

			nsResp, rawResp, err := askDomain(dialer, timeout, ns, clientQuery.Questions[0])
			if err != nil {
				continue
			}
			codeResp := nsResp.Header.Flags & 0x000F

			if isAnswer(nsResp.Answers, clientQuery.Questions[0].Type) || codeResp == NXDOMAIN {
				binary.BigEndian.PutUint16(rawResp[:2], clientQuery.Header.ID)
				return rawResp
			}

			for _, add := range nsResp.Additionals {
				if add.Type == 1 && add.RDLength == 4 {
					nextNS = append(nextNS, net.IP(add.RData).String())
				}
			}
		}

		currentNS = nextNS
		if len(nextNS) == 0 {
			break
		}
	}

	return packMessage(buildErrorResponse(clientQuery, SERVFAIL))
}

func main() {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 53,
	})
	if err != nil {
		fmt.Printf("Connection failed")
		return
	}

	defer conn.Close()
	dialer := net.Dialer{}

	for {
		buff := make([]byte, 512)
		n, clientAddr, err := conn.ReadFromUDP(buff)
		if err != nil {
			continue
		}

		clientQuery := unpackMessage(buff[:n])
		if len(clientQuery.Questions) == 0 {
			continue
		}

		resp := resolve(&dialer, clientQuery)
		_, _ = conn.WriteToUDP(resp, clientAddr)
	}
}
