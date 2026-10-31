package main

type header struct {
	ID       uint16
	Flags    uint16
	QDCount  uint16
	ANCount  uint16
	NSCCount uint16
	ARCount  uint16
}

type question struct {
	Name  []byte
	Type  uint16
	Class uint16
}

type resourceRecord struct {
	Name     []byte
	Type     uint16
	Class    uint16
	TTL      uint32
	RDLength uint16
	RData    []byte
}

type dnsMessage struct {
	Header      header
	Questions   []question
	Answers     []resourceRecord
	Authorities []resourceRecord
	Additionals []resourceRecord
}
