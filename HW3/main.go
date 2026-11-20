package main

import (
	"fmt"
	"log"
	"os"
	"sync"
)

func main() {
	cfg, err := parseOptions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	srcIP, err := getLocalIP(cfg.TargetIP)
	if err != nil {
		log.Fatal(err)
	}

	var protocols []ProtocolType
	if len(cfg.TCPPorts) > 0 {
		protocols = append(protocols, TCP)
	}
	if len(cfg.UDPPorts) > 0 {
		protocols = append(protocols, UDP)
	}

	s := NewScanner(srcIP, cfg.TargetIP, cfg.Timeout)
	if err := s.openSockets(protocols); err != nil {
		log.Fatal(err)
	}
	defer s.closeSockets()

	go s.listenTCP()
	go s.listenUDP()
	go s.listenICMP()

	totalPortsLen := len(cfg.TCPPorts) + len(cfg.UDPPorts)
	jobs := make(chan job, totalPortsLen)
	for _, p := range cfg.TCPPorts {
		jobs <- job{Protocol: TCP, Port: p}
	}
	for _, p := range cfg.UDPPorts {
		jobs <- job{Protocol: UDP, Port: p}
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < max(cfg.NumThreads, 1); i++ {
		wg.Go(func() {
			for p := range jobs {
				if st, ok := s.probePort(p.Protocol, p.Port); ok && st == "open" {
					fmt.Printf("TCP %v open\n", p)
				}
			}
		})
	}
	wg.Wait()
}
