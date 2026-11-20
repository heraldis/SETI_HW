package main

import (
	"flag"
	"net"
	"time"
)

type tcpHeader struct {
	SourcePort      uint16
	DestinationPort uint16
	SeqNumber       uint32
	AckNumber       uint32
	Offset          uint8
	Flags           uint8
	WindowSize      uint16
	Checksum        uint16
	UrgentPointer   uint16
}

type options struct {
	TargetIP   net.IP
	Timeout    time.Duration
	NumThreads int
	IsVerbose  bool
	IsGuess    bool
	TCPPorts   []uint16
	UDPPorts   []uint16
}

func (o *options) bindFlags(fs *flag.FlagSet, timeoutStr *string) {
	fs.StringVar(timeoutStr, "timeout", "2s", "timeout")
	fs.IntVar(&o.NumThreads, "j", 4, "number of threads")
	fs.IntVar(&o.NumThreads, "num-threads", 4, "number of threads")
	fs.BoolVar(&o.IsVerbose, "v", false, "verbose mode")
	fs.BoolVar(&o.IsVerbose, "verbose", false, "verbose mode")
	fs.BoolVar(&o.IsGuess, "g", false, "define application layer protocol")
	fs.BoolVar(&o.IsGuess, "guess", false, "define application layer protocol")
}
