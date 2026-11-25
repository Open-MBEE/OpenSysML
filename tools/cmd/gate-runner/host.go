package main

import (
	"bufio"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// hostCapacity reports the effective CPU limit and available memory.
func hostCapacity() (int, uint64) {
	cpus := runtime.NumCPU()
	if cgroup, ok := cgroupCPUs(); ok && cgroup < cpus {
		cpus = cgroup
	}
	if cpus < 1 {
		cpus = 1
	}
	return cpus, availableMemory()
}

// deriveDefaults sizes the shard fleet from the host: one shard per two GiB
// of available memory, never more shards than CPUs, then the CPUs left per shard.
func deriveDefaults() (int, int) {
	cpus, mem := hostCapacity()
	shards := 1
	if mem > 0 {
		shards = int(mem / (2 << 30))
		if shards < 1 {
			shards = 1
		}
	}
	if shards > cpus {
		shards = cpus
	}
	procs := cpus / shards
	if procs < 1 {
		procs = 1
	}
	return shards, procs
}

func cgroupCPUs() (int, bool) {
	if raw, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) == 2 && fields[0] != "max" {
			quota, qerr := strconv.ParseInt(fields[0], 10, 64)
			period, perr := strconv.ParseInt(fields[1], 10, 64)
			if qerr == nil && perr == nil && quota > 0 && period > 0 {
				return int(math.Ceil(float64(quota) / float64(period))), true
			}
		}
	}
	quota := readInt64("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	period := readInt64("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if quota > 0 && period > 0 {
		return int(math.Ceil(float64(quota) / float64(period))), true
	}
	return 0, false
}

func availableMemory() uint64 {
	if limit := readUint64("/sys/fs/cgroup/memory.max"); limit > 0 && limit < math.MaxInt64 {
		if used := readUint64("/sys/fs/cgroup/memory.current"); used < limit {
			return limit - used
		}
		return 0
	}
	if limit := readUint64("/sys/fs/cgroup/memory/memory.limit_in_bytes"); limit > 0 && limit < math.MaxInt64 {
		if used := readUint64("/sys/fs/cgroup/memory/memory.usage_in_bytes"); used < limit {
			return limit - used
		}
		return 0
	}
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			kb, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				return kb << 10
			}
		}
	}
	return 0
}

func readInt64(path string) int64 {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func readUint64(path string) uint64 {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0
	}
	return value
}
