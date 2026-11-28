package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cgroupFixture is a small stand-in for /proc and the cgroup mounts, so the
// hierarchy walk can be exercised without touching the host's controllers.
type cgroupFixture struct {
	src     cgroupSource
	meminfo string
	cpuBase string
	memBase string
}

func writeFixture(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newV2Fixture(t *testing.T, procPath, mountRoot string) cgroupFixture {
	t.Helper()
	base := t.TempDir()
	mount := filepath.Join(base, "sys", "fs", "cgroup")
	proc := filepath.Join(base, "proc")
	fixture := cgroupFixture{
		src: cgroupSource{
			cgroup:    filepath.Join(proc, "cgroup"),
			mountinfo: filepath.Join(proc, "mountinfo"),
		},
		meminfo: filepath.Join(base, "meminfo"),
		cpuBase: mount,
		memBase: mount,
	}
	writeFixture(t, fixture.src.cgroup, "0::"+procPath+"\n")
	writeFixture(t, fixture.src.mountinfo, fmt.Sprintf(
		"2593 338 0:32 %s %s rw shared:1321 - cgroup2 cgroup2 rw,seclabel\n", mountRoot, mount))
	writeFixture(t, fixture.meminfo, "MemTotal: 16777216 kB\nMemAvailable: 8388608 kB\n")
	return fixture
}

func newV1Fixture(t *testing.T, cpuPath, memPath string) cgroupFixture {
	t.Helper()
	base := t.TempDir()
	cpuMount := filepath.Join(base, "sys", "fs", "cgroup", "cpu")
	memMount := filepath.Join(base, "sys", "fs", "cgroup", "memory")
	proc := filepath.Join(base, "proc")
	fixture := cgroupFixture{
		src: cgroupSource{
			cgroup:    filepath.Join(proc, "cgroup"),
			mountinfo: filepath.Join(proc, "mountinfo"),
		},
		meminfo: filepath.Join(base, "meminfo"),
		cpuBase: cpuMount,
		memBase: memMount,
	}
	writeFixture(t, fixture.src.cgroup, "5:cpu,cpuacct:"+cpuPath+"\n4:memory:"+memPath+"\n")
	writeFixture(t, fixture.src.mountinfo,
		fmt.Sprintf("2593 338 0:32 / %s rw - cgroup cgroup rw,cpu\n", cpuMount)+
			fmt.Sprintf("2594 338 0:33 / %s rw - cgroup cgroup rw,memory\n", memMount))
	writeFixture(t, fixture.meminfo, "MemTotal: 16777216 kB\nMemAvailable: 8388608 kB\n")
	return fixture
}

func inFixture(base, procPath string) string {
	return filepath.Join(base, filepath.FromSlash(strings.TrimPrefix(procPath, "/")))
}

func TestCgroupV2OwnLimitBoundsCapacity(t *testing.T) {
	const procPath = "/user.slice/app"
	fixture := newV2Fixture(t, procPath, "/")
	dir := inFixture(fixture.cpuBase, procPath)
	writeFixture(t, filepath.Join(dir, "cpu.max"), "200000 100000\n")
	writeFixture(t, filepath.Join(dir, "memory.max"), fmt.Sprintf("%d\n", 1<<30))
	writeFixture(t, filepath.Join(dir, "memory.current"), fmt.Sprintf("%d\n", 256<<20))

	cpus, mem := capacityFrom(fixture.src, fixture.meminfo, 16)
	if cpus != 2 {
		t.Errorf("cpus = %d, want the process cgroup's 2", cpus)
	}
	if want := uint64(1<<30 - 256<<20); mem != want {
		t.Errorf("memory = %d, want %d", mem, want)
	}
}

func TestCgroupV2ParentLimitIsInherited(t *testing.T) {
	const procPath = "/user.slice/user-1000.slice/app"
	fixture := newV2Fixture(t, procPath, "/")
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, procPath), "cpu.max"), "max 100000\n")
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, procPath), "memory.max"), "max\n")
	parent := "/user.slice/user-1000.slice"
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, parent), "cpu.max"), "300000 100000\n")
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, parent), "memory.max"), fmt.Sprintf("%d\n", 2<<30))
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, parent), "memory.current"), fmt.Sprintf("%d\n", 1<<30))
	grandparent := "/user.slice"
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, grandparent), "cpu.max"), "600000 100000\n")
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, grandparent), "memory.max"), fmt.Sprintf("%d\n", 8<<30))

	cpus, mem := capacityFrom(fixture.src, fixture.meminfo, 16)
	if cpus != 3 {
		t.Errorf("cpus = %d, want the tightest ancestor quota, 3", cpus)
	}
	if want := uint64(1 << 30); mem != want {
		t.Errorf("memory = %d, want the tightest ancestor headroom, %d", mem, want)
	}
}

func TestCgroupV2TightestLimitAtTheProcessCgroup(t *testing.T) {
	const procPath = "/slice/app"
	fixture := newV2Fixture(t, procPath, "/")
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, procPath), "cpu.max"), "200000 100000\n")
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, "/slice"), "memory.max"), fmt.Sprintf("%d\n", 4<<30))
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, "/slice"), "memory.current"), fmt.Sprintf("%d\n", 3<<30))
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, procPath), "memory.max"), fmt.Sprintf("%d\n", 1<<30))
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, procPath), "memory.current"), fmt.Sprintf("%d\n", 256<<20))

	cpus, mem := capacityFrom(fixture.src, fixture.meminfo, 16)
	if cpus != 2 {
		t.Errorf("cpus = %d, want 2", cpus)
	}
	if want := uint64(1<<30 - 256<<20); mem != want {
		t.Errorf("memory = %d, want the process cgroup's %d", mem, want)
	}
}

func TestCgroupV1ParentLimitIsInherited(t *testing.T) {
	const procPath = "/user.slice/app"
	fixture := newV1Fixture(t, procPath, procPath)
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, procPath), "cpu.cfs_quota_us"), "-1\n")
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, procPath), "cpu.cfs_period_us"), "100000\n")
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, procPath), "memory.limit_in_bytes"), "9223372036854771712\n")
	parent := "/user.slice"
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, parent), "cpu.cfs_quota_us"), "500000\n")
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, parent), "cpu.cfs_period_us"), "100000\n")
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, parent), "memory.limit_in_bytes"), fmt.Sprintf("%d\n", 2<<30))
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, parent), "memory.usage_in_bytes"), fmt.Sprintf("%d\n", 512<<20))

	cpus, mem := capacityFrom(fixture.src, fixture.meminfo, 16)
	if cpus != 5 {
		t.Errorf("cpus = %d, want the parent's 5", cpus)
	}
	if want := uint64(2<<30 - 512<<20); mem != want {
		t.Errorf("memory = %d, want %d", mem, want)
	}
}

func TestCgroupNamespaceRootMapsTheProcessCgroup(t *testing.T) {
	fixture := newV2Fixture(t, "/", "/../../../../..")
	writeFixture(t, filepath.Join(fixture.cpuBase, "cpu.max"), "400000 100000\n")
	writeFixture(t, filepath.Join(fixture.memBase, "memory.max"), fmt.Sprintf("%d\n", 3<<30))
	writeFixture(t, filepath.Join(fixture.memBase, "memory.current"), fmt.Sprintf("%d\n", 1<<30))

	cpus, mem := capacityFrom(fixture.src, fixture.meminfo, 16)
	if cpus != 4 {
		t.Errorf("cpus = %d, want 4 from the namespace's cgroup root", cpus)
	}
	if want := uint64(2 << 30); mem != want {
		t.Errorf("memory = %d, want %d", mem, want)
	}
}

func TestRelativeCgroupPathRespectsTheMountRoot(t *testing.T) {
	for _, tc := range []struct {
		root, procPath, want string
	}{
		{root: "/", procPath: "/slice/app", want: "slice/app"},
		{root: "/docker/abc", procPath: "/docker/abc/slice", want: "slice"},
		{root: "/foo", procPath: "/foobar/app", want: "foobar/app"},
		{root: "/../../..", procPath: "/slice", want: "slice"},
	} {
		if got := relativeCgroupPath(tc.root, tc.procPath); got != tc.want {
			t.Errorf("relativeCgroupPath(%q, %q) = %q, want %q", tc.root, tc.procPath, got, tc.want)
		}
	}
}

func TestNoFiniteCgroupLimitFallsBackToMemAvailable(t *testing.T) {
	fixture := newV2Fixture(t, "/slice/app", "/")
	writeFixture(t, filepath.Join(inFixture(fixture.cpuBase, "/slice/app"), "cpu.max"), "max 100000\n")
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, "/slice"), "memory.max"), "max\n")

	cpus, mem := capacityFrom(fixture.src, fixture.meminfo, 16)
	if cpus != 16 {
		t.Errorf("cpus = %d, want the host's 16", cpus)
	}
	if want := uint64(8 << 30); mem != want {
		t.Errorf("memory = %d, want MemAvailable %d", mem, want)
	}
}

func TestMemAvailableCapsFiniteCgroupHeadroom(t *testing.T) {
	fixture := newV2Fixture(t, "/slice/app", "/")
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, "/slice/app"), "memory.max"), fmt.Sprintf("%d\n", 16<<30))
	writeFixture(t, filepath.Join(inFixture(fixture.memBase, "/slice/app"), "memory.current"), "0\n")

	_, mem := capacityFrom(fixture.src, fixture.meminfo, 16)
	if want := uint64(8 << 30); mem != want {
		t.Errorf("memory = %d, want MemAvailable to cap the finite cgroup headroom at %d", mem, want)
	}
}
