package main

import (
	"encoding/binary"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type scanner struct {
	SourceIP net.IP
	TargetIP net.IP
	Timeout  time.Duration

	TCPConn  net.PacketConn
	UDPConn  net.PacketConn
	ICMPConn net.PacketConn

	Mutex   sync.Mutex
	Waiters map[uint16]waiter

	PortSeq atomic.Uint32
}

type job struct {
	Protocol ProtocolType
	Port     uint16
}

type waiter struct {
	TargetPort uint16
	Channel    chan string
}

func NewScanner(srcIP, targetIP net.IP, timeout time.Duration) *scanner {
	s := &scanner{
		SourceIP: srcIP.To4(),
		TargetIP: targetIP.To4(),
		Timeout:  timeout,
		Waiters:  make(map[uint16]waiter),
	}
	s.PortSeq.Store(40000 + rand.Uint32N(10000))
	return s
}

func (s *scanner) nextLocalPort() uint16 {
	return uint16(40000 + (s.PortSeq.Add(1) % 25000))
}

func (s *scanner) openSockets(protocols []ProtocolType) error {
	for _, p := range protocols {
		switch p {
		case TCP:
			c, err := net.ListenPacket("ip4:tcp", "0.0.0.0")
			if err != nil {
				return err
			}
			s.TCPConn = c
		case UDP:
			c, err := net.ListenPacket("ip4:udp", "0.0.0.0")
			if err != nil {
				return err
			}
			s.UDPConn = c

			c, err = net.ListenPacket("ip4:icmp", "0.0.0.0")
			if err != nil {
				s.closeSockets()
				return err
			}
			s.ICMPConn = c
		}

	}
	return nil
}

func (s *scanner) closeSockets() {
	for _, c := range []net.PacketConn{s.TCPConn, s.UDPConn, s.ICMPConn} {
		if c != nil {
			_ = c.Close()
		}
	}
}

func (s *scanner) registerWaiter(localPort, targetPort uint16) chan string {
	ch := make(chan string, 1)
	s.Mutex.Lock()
	s.Waiters[localPort] = waiter{
		TargetPort: targetPort,
		Channel:    ch,
	}
	s.Mutex.Unlock()
	return ch
}

func (s *scanner) unregisterWaiter(port uint16) {
	s.Mutex.Lock()
	delete(s.Waiters, port)
	s.Mutex.Unlock()
}

func (s *scanner) notifyWaiter(localPort, remotePort uint16, state string) {
	s.Mutex.Lock()
	w, ok := s.Waiters[localPort]
	s.Mutex.Unlock()

	if ok && w.TargetPort == remotePort {
		select {
		case w.Channel <- state:
		default:
		}
	}
}

func (s *scanner) listenTCP() {
	if s.TCPConn == nil {
		return
	}
	buf := make([]byte, 4096)
	dstAddr := &net.IPAddr{IP: s.TargetIP}

	for {
		n, addr, err := s.TCPConn.ReadFrom(buf)
		if err != nil {
			return
		}
		if !s.isFromTarget(addr) {
			continue
		}

		packet := buf[:n]
		if len(packet) < 20 {
			continue
		}

		respSrcPort := binary.BigEndian.Uint16(packet[0:2])
		respDstPort := binary.BigEndian.Uint16(packet[2:4])
		seqNum := binary.BigEndian.Uint32(packet[4:8])
		ackNum := binary.BigEndian.Uint32(packet[8:12])
		flags := packet[13]

		switch {
		case (flags & (SYN | ACK)) == (SYN | ACK):
			rst := buildTCPPacket(s.SourceIP, s.TargetIP, respDstPort, respSrcPort, ackNum, seqNum+1, RST)
			_, _ = s.TCPConn.WriteTo(rst, dstAddr)
			s.notifyWaiter(respDstPort, respSrcPort, "open")
		case (flags & RST) != 0:
			s.notifyWaiter(respDstPort, respSrcPort, "closed")
		}
	}
}

func (s *scanner) listenUDP() {
	if s.UDPConn == nil {
		return
	}

	buf := make([]byte, 4096)
	for {
		n, addr, err := s.UDPConn.ReadFrom(buf)
		if err != nil {
			return
		}
		if !s.isFromTarget(addr) {
			continue
		}

		srcPort, dstPort, ok := parseUDPResponse(buf[:n])
		if !ok {
			continue
		}

		s.notifyWaiter(dstPort, srcPort, "open")
	}
}

func (s *scanner) listenICMP() {
	if s.ICMPConn == nil {
		return
	}
	buf := make([]byte, 4096)

	for {
		n, _, err := s.ICMPConn.ReadFrom(buf)
		if err != nil {
			return
		}

		origSrcPort, origDstPort, ok := parseICMPPortUnreachable(buf[:n], s.TargetIP)
		if !ok {
			continue
		}

		s.notifyWaiter(origSrcPort, origDstPort, "closed")
	}
}

func (s *scanner) isFromTarget(addr net.Addr) bool {
	ipAddr, ok := addr.(*net.IPAddr)
	return ok && ipAddr.IP.To4().Equal(s.TargetIP)
}

func (s *scanner) probePort(protocol ProtocolType, targetPort uint16) (string, bool) {
	localPort := s.nextLocalPort()
	ch := s.registerWaiter(localPort, targetPort)
	defer s.unregisterWaiter(localPort)

	dstAddr := &net.IPAddr{IP: s.TargetIP}

	switch protocol {
	case TCP:
		pkt := buildTCPPacket(s.SourceIP, s.TargetIP, localPort, targetPort, rand.Uint32(), 0, SYN)
		if _, err := s.TCPConn.WriteTo(pkt, dstAddr); err != nil {
			return "", false
		}
	case UDP:
		pkt := buildUDPPacket(s.SourceIP, s.TargetIP, localPort, targetPort, []byte("\x00"))
		if _, err := s.UDPConn.WriteTo(pkt, dstAddr); err != nil {
			return "", false
		}
	default:
		return "", false
	}

	select {
	case state := <-ch:
		return state, true
	case <-time.After(s.Timeout):
		return "filtered", true
	}
}
