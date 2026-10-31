package main

import (
	"encoding/binary"
	"net"
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
		name, next := decodeQuestionName(data, offset)
		offset = next

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

func main() {
	conn, _ := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 53,
	})

	for {
		buff := make([]byte, 1024)
		n, clientAddr, _ := conn.ReadFromUDP(buff)

		clientQuery := unpackMessage(buff[:n])
		if len(clientQuery.Questions) == 0 {
			continue
		}

		currentNS := []string{"198.41.0.4"}
		resolved := false

		for range 10 {
			nextNS := []string{}

			for _, ns := range currentNS {
				rootConn, err := net.Dial("udp", ns+":53")
				if err != nil {
					continue
				}

				_, err = rootConn.Write(packMessage(sendMsg(clientQuery.Questions[0])))
				if err != nil {
					rootConn.Close()
					continue
				}

				rootRespBuf := make([]byte, 2048)
				rootN, err := rootConn.Read(rootRespBuf)
				rootConn.Close()
				if err != nil {
					continue
				}

				nsResp := unpackMessage(rootRespBuf[:rootN])
				println("received response from", ns)
				println("answers:", nsResp.Header.ANCount)
				println("authorities:", nsResp.Header.NSCCount)
				println("additionals:", nsResp.Header.ARCount)
				rcode := nsResp.Header.Flags & 0x000F

				hasTargetAnswer := false
				if nsResp.Header.ANCount > 0 {
					for _, ans := range nsResp.Answers {
						if ans.Type == clientQuery.Questions[0].Type {
							hasTargetAnswer = true
							break
						}
					}
				}

				if hasTargetAnswer || rcode == NXDOMAIN {
					binary.BigEndian.PutUint16(rootRespBuf[0:2], clientQuery.Header.ID)
					conn.WriteToUDP(rootRespBuf[:rootN], clientAddr)
					resolved = true
					break
				}

				for _, add := range nsResp.Additionals {
					if add.Type == 1 && add.RDLength == 4 {
						nextNS = append(nextNS, net.IP(add.RData).String())
					}
				}
			}

			if resolved {
				break
			}

			println("next NS count:", len(nextNS))
			if len(nextNS) == 0 {
				break
			}

			currentNS = nextNS
		}

		if !resolved {
			errMsg := buildErrorResponse(clientQuery, SERVFAIL)
			conn.WriteToUDP(packMessage(errMsg), clientAddr)
		}
	}
}
