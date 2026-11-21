package main

import (
	"flag"
	"net"
	"time"
)

const (
	SYN byte = 0x02
	RST byte = 0x04
	ACK byte = 0x10
)

type ProtocolType string

const (
	TCP ProtocolType = "tcp"
	UDP ProtocolType = "udp"
)

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
	fs.StringVar(timeoutStr, "timeout", "2", "timeout")
	fs.IntVar(&o.NumThreads, "j", 4, "number of threads")
	fs.IntVar(&o.NumThreads, "num-threads", 4, "number of threads")
	fs.BoolVar(&o.IsVerbose, "v", false, "verbose mode")
	fs.BoolVar(&o.IsVerbose, "verbose", false, "verbose mode")
	fs.BoolVar(&o.IsGuess, "g", false, "define application layer protocol")
	fs.BoolVar(&o.IsGuess, "guess", false, "define application layer protocol")
}
