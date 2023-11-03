package numa

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Node represent NUMA node ID, CPU IDs and memory information.
type Node struct {
	ID           int
	CPU          []int
	MemAvailable uint64
	MemFree      uint64
	MemTotal     uint64
}

type memInfo struct {
	MemTotal     uint64
	MemFree      uint64
	ActiveFile   uint64
	InactiveFile uint64
	SReclaimable uint64
}

// GetNodes returns NUMA nodes information.
func GetNodes() ([]Node, error) {
	dir, err := os.ReadDir("/sys/devices/system/node/")
	if err != nil {
		return nil, err
	}

	var nodes []Node
	for _, i := range dir {
		if !i.IsDir() {
			continue
		}

		if !strings.HasPrefix(i.Name(), "node") {
			continue
		}

		nodeID, err := strconv.Atoi(strings.TrimPrefix(i.Name(), "node"))
		if err != nil {
			return nil, err
		}

		nodePath := filepath.Join("/sys/devices/system/node", i.Name())

		meminfo, err := parseMemInfo(filepath.Join(nodePath, "meminfo"))
		if err != nil {
			return nil, fmt.Errorf("parse meminfo: %w", err)
		}

		cpuIDs, err := parseCpuList(filepath.Join(nodePath, "cpulist"))
		if err != nil {
			return nil, fmt.Errorf("parse cpulist: %w", err)
		}

		nodes = append(nodes, Node{
			ID:           nodeID,
			CPU:          cpuIDs,
			MemAvailable: calculateAvailableMemory(meminfo),
			MemFree:      meminfo.MemFree,
			MemTotal:     meminfo.MemTotal,
		})
	}

	return nodes, nil
}

func parseMemInfo(path string) (memInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return memInfo{}, err
	}

	var m memInfo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// Node 0 MemTotal:       263777956 kB
		tokens := strings.Split(scanner.Text(), ":")
		if len(tokens) != 2 {
			continue
		}

		keyTokens := strings.Split(strings.TrimSpace(tokens[0]), " ")
		if len(keyTokens) != 3 {
			continue
		}
		key := keyTokens[2]
		value := strings.Replace(strings.TrimSpace(tokens[1]), " kB", "", -1)

		switch key {
		case "MemTotal":
			t, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return memInfo{}, err
			}
			m.MemTotal = t * 1024
		case "MemFree":
			t, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return memInfo{}, err
			}
			m.MemFree = t * 1024
		case "Active(file)":
			t, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return memInfo{}, err
			}

			m.ActiveFile = t * 1024
		case "Inactive(file)":
			t, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return memInfo{}, err
			}

			m.InactiveFile = t * 1024
		case "SReclaimable":
			t, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return memInfo{}, err
			}

			m.SReclaimable = t * 1024
		}
	}

	return m, nil
}

func parseCpuList(path string) ([]int, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 0-31\n
	tokens := strings.Split(strings.TrimRight(string(f), "\n"), "-")
	if len(tokens) != 2 {
		return nil, fmt.Errorf("invalid format: %q", string(f))
	}

	first, err := strconv.Atoi(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("convert first %q: %w", tokens[0], err)
	}

	last, err := strconv.Atoi(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("convert last %q: %w", tokens[1], err)
	}

	var ids []int
	for i := first; i <= last; i++ {
		ids = append(ids, i)
	}

	return ids, nil
}

func calculateAvailableMemory(m memInfo) uint64 {
	watermarkLow, err := getWatermarkLow()
	if err != nil {
		return m.MemFree + m.SReclaimable + m.ActiveFile + m.InactiveFile
	}

	memAvailable := m.MemFree - watermarkLow
	pageCache := m.ActiveFile + m.InactiveFile
	pageCache -= uint64(math.Min(float64(pageCache/2), float64(watermarkLow)))
	memAvailable += pageCache
	memAvailable += m.SReclaimable - uint64(math.Min(float64(m.SReclaimable/2.0), float64(watermarkLow)))

	if memAvailable < 0 {
		memAvailable = 0
	}

	return memAvailable
}

func getWatermarkLow() (uint64, error) {
	var watermarkLow uint64
	watermarkLow = 0

	f, err := os.Open("/proc/zoneinfo")
	if err != nil {
		return watermarkLow, err
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if strings.HasPrefix(fields[0], "low") {
			lowValue, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				lowValue = 0
			}
			watermarkLow += lowValue
		}
	}

	return watermarkLow * uint64(os.Getpagesize()), nil
}
