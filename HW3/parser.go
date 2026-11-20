package main

import (
	"flag"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
)

func parseOptions(optns []string) (options, error) {
	var config options
	var timeoutStr string

	//bind arguments
	fs := flag.NewFlagSet("portscan", flag.ContinueOnError)
	config.bindFlags(fs, &timeoutStr)
	if err := fs.Parse(optns); err != nil {
		return config, err
	}
	args := fs.Args()
	if len(args) == 0 {
		return config, fmt.Errorf("IP address is required")
	}
	if config.NumThreads <= 0 {
		return config, fmt.Errorf("numeber of threads must be positive")
	}

	//parse ip
	ip := net.ParseIP(args[0])
	if ip == nil || ip.To4() == nil {
		return config, fmt.Errorf("invalid IPv4 address: %v", args[0])
	}
	config.TargetIP = ip.To4()

	//parse timeout
	if sec, err := strconv.ParseFloat(timeoutStr, 64); err == nil && sec > 0 {
		config.Timeout = time.Duration(sec * float64(time.Second))
	} else {
		return config, fmt.Errorf("invalid timeout: %v", timeoutStr)
	}

	//parse ports
	if err := parseSpecs(args[1:], &config); err != nil {
		return config, err
	}

	return config, nil
}

func parseSpecs(args []string, config *options) error {
	tcpSet := make(map[uint16]struct{})
	udpSet := make(map[uint16]struct{})

	if len(args) > 0 {
		for _, spec := range args {
			protocol, portsStr, hasSlash := strings.Cut(spec, "/")
			protocol = strings.ToLower(protocol)
			var targetSet map[uint16]struct{}
			switch protocol {
			case "tcp":
				targetSet = tcpSet
			case "udp":
				targetSet = udpSet
			default:
				return fmt.Errorf("unknown protocol: %v", protocol)
			}

			if !hasSlash {
				fillAllPorts(targetSet)
				continue
			}

			items := strings.SplitSeq(portsStr, ",")
			for item := range items {
				if item == "" {
					return fmt.Errorf("port range is empty")
				}

				startStr, endStr, isRange := strings.Cut(item, "-")
				start, err := parsePort(startStr)
				if err != nil {
					return err
				}
				end := start
				if isRange {
					end, err = parsePort(endStr)
					if err != nil {
						return err
					}
					if start > end {
						return fmt.Errorf("invalid port range: start: %v , end :%v", start, end)
					}
				}

				for p := uint32(start); p <= uint32(end); p++ {
					targetSet[uint16(p)] = struct{}{}
				}
			}
		}
	} else {
		fillAllPorts(tcpSet)
		fillAllPorts(udpSet)
	}

	config.TCPPorts = setToSortedSlice(tcpSet)
	config.UDPPorts = setToSortedSlice(udpSet)

	return nil
}

func parsePort(s string) (uint16, error) {
	val, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid port %v: %v", s, err)
	}
	if val == 0 {
		return 0, fmt.Errorf("port cannot be 0")
	}
	return uint16(val), nil
}

func setToSortedSlice(set map[uint16]struct{}) []uint16 {
	res := make([]uint16, 0, len(set))
	for p := range set {
		res = append(res, p)
	}
	slices.Sort(res)
	return res
}

func fillAllPorts(set map[uint16]struct{}) {
	for i := 1; i <= 65535; i++ {
		set[uint16(i)] = struct{}{}
	}
}
