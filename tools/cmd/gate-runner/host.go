package main

import (
	"bufio"
	"bytes"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	procCgroupPath    = "/proc/self/cgroup"
	procMountinfoPath = "/proc/self/mountinfo"
	meminfoPath       = "/proc/meminfo"
	shardMemoryBudget = 2 << 30
)

// cgroupSource names the kernel files used to size the runner. Fixture tests
// point it at a tree instead of /proc.
type cgroupSource struct {
	cgroup    string
	mountinfo string
}

var systemCgroupSource = cgroupSource{cgroup: procCgroupPath, mountinfo: procMountinfoPath}

// hostCapacity reports the effective CPU limit and available memory.
func hostCapacity() (int, uint64) {
	return capacityFrom(systemCgroupSource, meminfoPath, runtime.NumCPU())
}

// capacityFrom walks the process's cgroup and its ancestors, taking the
// tightest CPU quota and smallest finite memory headroom from the chain.
func capacityFrom(src cgroupSource, meminfo string, numCPU int) (int, uint64) {
	cpus := numCPU
	if quota, ok := cgroupCPUs(src); ok && quota < cpus {
		cpus = quota
	}
	if cpus < 1 {
		cpus = 1
	}
	return cpus, availableMemory(src, meminfo)
}

// defaultShards sizes the shard fleet from the host: one shard per two GiB of
// available memory, never more shards than CPUs, at least one.
func defaultShards(cpus int, mem uint64) int {
	shards := 1
	if mem > 0 {
		shards = int(mem / shardMemoryBudget)
		if shards < 1 {
			shards = 1
		}
	}
	if shards > cpus {
		shards = cpus
	}
	if shards < 1 {
		shards = 1
	}
	return shards
}

// procsForWorkers divides the CPU budget over the workers that will actually
// run, at least one CPU each.
func procsForWorkers(cpus, workers int) int {
	if workers < 1 {
		workers = 1
	}
	procs := cpus / workers
	if procs < 1 {
		procs = 1
	}
	return procs
}

// cgroupEntry is one line of /proc/self/cgroup.
type cgroupEntry struct {
	id          string
	controllers []string
	dir         string
}

func parseCgroupEntries(raw []byte) []cgroupEntry {
	var entries []cgroupEntry
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 {
			continue
		}
		var controllers []string
		for _, name := range strings.Split(fields[1], ",") {
			if name != "" {
				controllers = append(controllers, name)
			}
		}
		entries = append(entries, cgroupEntry{
			id:          fields[0],
			controllers: controllers,
			dir:         path.Clean("/" + strings.TrimSpace(fields[2])),
		})
	}
	return entries
}

// cgroupMount is one cgroup filesystem from /proc/self/mountinfo.
type cgroupMount struct {
	opts    []string
	root    string
	dir     string
	unified bool
}

func (m cgroupMount) has(controller string) bool {
	for _, opt := range m.opts {
		if opt == controller {
			return true
		}
	}
	return false
}

func parseCgroupMounts(raw []byte) []cgroupMount {
	var mounts []cgroupMount
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		sep := -1
		for i, field := range fields {
			if field == "-" {
				sep = i
				break
			}
		}
		if sep < 5 || sep+3 >= len(fields) {
			continue
		}
		mount := cgroupMount{
			root: path.Clean("/" + strings.TrimPrefix(fields[3], "/")),
			dir:  fields[4],
		}
		switch fields[sep+1] {
		case "cgroup2":
			mount.unified = true
		case "cgroup":
			for _, opt := range strings.Split(strings.Join(fields[sep+3:], ","), ",") {
				if opt != "" {
					mount.opts = append(mount.opts, opt)
				}
			}
		default:
			continue
		}
		mounts = append(mounts, mount)
	}
	return mounts
}

// chain returns the process's cgroup and its ancestors for one controller,
// nearest first, with the cgroup version (1 or 2).
func (s cgroupSource) chain(controller string) ([]string, int) {
	entries := parseCgroupEntries(readFile(s.cgroup))
	mounts := parseCgroupMounts(readFile(s.mountinfo))
	for _, m := range mounts {
		if m.unified {
			dirs := ancestorDirs(m, unifiedPath(entries))
			if hasV2Controller(dirs, controller) {
				return dirs, 2
			}
		}
	}
	for _, m := range mounts {
		if m.has(controller) {
			return ancestorDirs(m, controllerPath(entries, controller)), 1
		}
	}
	return nil, 0
}

// hasV2Controller reports whether the unified hierarchy exposes the controller.
// A disabled v2 controller can still be limited by a v1 mount instead.
func hasV2Controller(dirs []string, controller string) bool {
	name := ""
	switch controller {
	case "cpu":
		name = "cpu.max"
	case "memory":
		name = "memory.max"
	}
	if name == "" {
		return false
	}
	for _, dir := range dirs {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func unifiedPath(entries []cgroupEntry) string {
	for _, entry := range entries {
		if entry.id == "0" {
			return entry.dir
		}
	}
	return "/"
}

func controllerPath(entries []cgroupEntry, controller string) string {
	for _, entry := range entries {
		for _, name := range entry.controllers {
			if name == controller {
				return entry.dir
			}
		}
	}
	return "/"
}

// ancestorDirs maps the process's cgroup path into a mount and walks up to the
// mount root, stopping at the hierarchy root visible to this process.
func ancestorDirs(m cgroupMount, procPath string) []string {
	dir := filepath.Join(m.dir, filepath.FromSlash(relativeCgroupPath(m.root, procPath)))
	var dirs []string
	for {
		dirs = append(dirs, dir)
		if dir == m.dir || dir == "/" || dir == "." {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

func relativeCgroupPath(root, procPath string) string {
	root = strings.TrimSuffix(root, "/")
	if root == "" {
		root = "/"
	}
	if root != "/" && (procPath == root || strings.HasPrefix(procPath, root+"/")) {
		return strings.TrimPrefix(strings.TrimPrefix(procPath, root), "/")
	}
	return strings.TrimPrefix(procPath, "/")
}

func cgroupCPUs(src cgroupSource) (int, bool) {
	dirs, version := src.chain("cpu")
	best := 0
	for _, dir := range dirs {
		var quota, period int64
		if version == 2 {
			fields := strings.Fields(string(readFile(filepath.Join(dir, "cpu.max"))))
			if len(fields) != 2 || fields[0] == "max" {
				continue
			}
			quota, _ = strconv.ParseInt(fields[0], 10, 64)
			period, _ = strconv.ParseInt(fields[1], 10, 64)
		} else {
			quota = readInt64(filepath.Join(dir, "cpu.cfs_quota_us"))
			period = readInt64(filepath.Join(dir, "cpu.cfs_period_us"))
		}
		if quota <= 0 || period <= 0 {
			continue
		}
		n := int(math.Ceil(float64(quota) / float64(period)))
		if best == 0 || n < best {
			best = n
		}
	}
	return best, best > 0
}

func availableMemory(src cgroupSource, meminfo string) uint64 {
	dirs, version := src.chain("memory")
	var headroom uint64
	found := false
	for _, dir := range dirs {
		var limit, used uint64
		if version == 2 {
			limit = readUint64(filepath.Join(dir, "memory.max"))
			used = readUint64(filepath.Join(dir, "memory.current"))
		} else {
			limit = readUint64(filepath.Join(dir, "memory.limit_in_bytes"))
			used = readUint64(filepath.Join(dir, "memory.usage_in_bytes"))
		}
		// cgroup v1 writes a page-aligned MaxInt64 for "unlimited"; no real
		// host has that much RAM, so treat it the same as v2's "max".
		if limit == 0 || limit >= math.MaxInt64/2 {
			continue
		}
		room := uint64(0)
		if used < limit {
			room = limit - used
		}
		if !found || room < headroom {
			headroom, found = room, true
		}
	}
	host := memAvailable(readFile(meminfo))
	switch {
	case host > 0 && found:
		return min(host, headroom)
	case found:
		return headroom
	default:
		return host
	}
}

func readFile(name string) []byte {
	data, err := os.ReadFile(name) // #nosec G304 -- /proc, /sys or a fixture tree
	if err != nil {
		return nil
	}
	return data
}

func readInt64(name string) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(string(readFile(name))), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func readUint64(name string) uint64 {
	value, err := strconv.ParseUint(strings.TrimSpace(string(readFile(name))), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

// memAvailable reads MemAvailable (in kB) from meminfo, 0 when it is not stated.
func memAvailable(raw []byte) uint64 {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			kb, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0
			}
			return kb << 10
		}
	}
	return 0
}
