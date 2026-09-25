package analysis

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
)

// JobMemoryBudget is the memory the default jobs reserve per worker: one per CPU, fewer
// when the memory available to the process would leave a worker less than this.
const JobMemoryBudget = 512 << 20

// jobsForMemory is how many workers of JobMemoryBudget each the memory available leaves
// room for, at least one; cpus when the memory is unknown.
func jobsForMemory(available int64, cpus int) int {
	if available <= 0 {
		return cpus
	}
	return max(1, min(cpus, int(available/JobMemoryBudget)))
}

// availableMemory is the memory the process may still take, in bytes: the lesser of the
// host's MemAvailable and the cgroup's memory.max less its use; 0 where neither is known.
func availableMemory() int64 {
	host := memAvailable(readFile("/proc/meminfo"))
	limit := cgroupHeadroom(readFile("/sys/fs/cgroup/memory.max"), readFile("/sys/fs/cgroup/memory.current"))
	switch {
	case host > 0 && limit > 0:
		return min(host, limit)
	case limit > 0:
		return limit
	default:
		return host
	}
}

func readFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// memAvailable reads MemAvailable (in kB) off /proc/meminfo, 0 when it is not stated.
func memAvailable(meminfo []byte) int64 {
	sc := bufio.NewScanner(bytes.NewReader(meminfo))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0
			}
			return kb << 10
		}
	}
	return 0
}

// cgroupHeadroom is what a cgroup v2 memory limit leaves over the memory already charged,
// 0 when the limit is "max" or unstated; a limit already reached leaves room for one job.
func cgroupHeadroom(limit, current []byte) int64 {
	maxText := strings.TrimSpace(string(limit))
	if maxText == "" || maxText == "max" {
		return 0
	}
	limitBytes, err := strconv.ParseInt(maxText, 10, 64)
	if err != nil {
		return 0
	}
	used, _ := strconv.ParseInt(strings.TrimSpace(string(current)), 10, 64)
	return max(limitBytes-used, 1)
}
