package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func formatOpenPort(
	protocol ProtocolType,
	port uint16,
	verbose bool,
	rtt time.Duration,
	guess bool,
	guessedProto string,
) string {
	line := fmt.Sprintf("%s %d", strings.ToUpper(string(protocol)), port)

	if verbose {
		ms := float64(rtt) / float64(time.Millisecond)
		line += fmt.Sprintf(" %.2f", ms)
	}

	if guess {
		if guessedProto == "" {
			guessedProto = "-"
		}
		line += " " + guessedProto
	}

	return line
}

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

	var (
		hasOpen atomic.Bool
		wg      sync.WaitGroup
	)
	workers := max(cfg.NumThreads, 1)
	wg.Add(workers)
	for range workers {
		go s.runWorker(jobs, cfg, &hasOpen, &sync.Mutex{}, &wg)
	}
	wg.Wait()

	if !hasOpen.Load() {
		fmt.Fprintln(os.Stderr, "No open ports found.")
	}
}

func (s *scanner) runWorker(
	jobs <-chan job,
	cfg options,
	hasOpen *atomic.Bool,
	outMu *sync.Mutex,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for j := range jobs {
		state, rtt, ok := s.probePort(j.Protocol, j.Port, cfg.IsVerbose)
		if !ok || state != "open" {
			continue
		}

		hasOpen.Store(true)

		var guessed string
		if cfg.IsGuess {
			guessed = guessProtocol(j.Protocol, s.TargetIP, j.Port, s.Timeout)
		}

		line := formatOpenPort(j.Protocol, j.Port, cfg.IsVerbose, rtt, cfg.IsGuess, guessed)

		outMu.Lock()
		fmt.Println(line)
		outMu.Unlock()
	}
}
