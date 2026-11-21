package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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

	var hasOpen atomic.Bool
	var wg sync.WaitGroup
	for range max(cfg.NumThreads, 1) {
		wg.Go(func() {
			for j := range jobs {
				if state, ok := s.probePort(j.Protocol, j.Port); ok && state == "open" {
					hasOpen.Store(true)
					fmt.Printf("%s %d\n", strings.ToUpper(string(j.Protocol)), j.Port)
				}
			}
		})
	}
	wg.Wait()

	if !hasOpen.Load() {
		fmt.Fprintln(os.Stderr, "No open ports found.")
	}
}
