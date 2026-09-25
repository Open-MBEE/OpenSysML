package analysis

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"path/filepath"
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
// host's MemAvailable and the tightest cgroup memory.max less its use; 0 where neither is known.
func availableMemory() int64 {
	host := memAvailable(readFile("/proc/meminfo"))
	limit := cgroupTreeHeadroom("/sys/fs/cgroup", readFile("/proc/self/cgroup"))
	switch {
	case host > 0 && limit > 0:
		return min(host, limit)
	case limit > 0:
		return limit
	default:
		return host
	}
}

// readFile is the file's bytes, nil when it cannot be read; the paths are the kernel's own.
func readFile(name string) []byte {
	data, err := os.ReadFile(name) // #nosec G304 -- /proc and /sys files named by the kernel
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

// cgroupTreeHeadroom is the least headroom of the process's cgroup v2 cgroup and its
// ancestors under root, since a limit anywhere above applies; 0 when none is limited.
func cgroupTreeHeadroom(root string, procCgroup []byte) int64 {
	var least int64
	for dir := cgroupV2Path(procCgroup); ; dir = path.Dir(dir) {
		base := filepath.Join(root, filepath.FromSlash(dir))
		room := cgroupHeadroom(readFile(filepath.Join(base, "memory.max")), readFile(filepath.Join(base, "memory.current")))
		if room > 0 && (least == 0 || room < least) {
			least = room
		}
		if dir == "/" {
			return least
		}
	}
}

// cgroupV2Path is the process's cgroup on the unified hierarchy, the "0::/path" line of
// /proc/self/cgroup; "/" when the file does not name one.
func cgroupV2Path(procCgroup []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(procCgroup))
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "0::"); ok {
			return path.Clean("/" + strings.TrimSpace(rest))
		}
	}
	return "/"
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
