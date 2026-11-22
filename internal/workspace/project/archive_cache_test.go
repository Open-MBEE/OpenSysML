package project

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestArchiveCacheEvictsTheLeastRecentlyRead(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 3)
	for i := range paths {
		paths[i] = writeArchive(t, filepath.Join(dir, fmt.Sprintf("p%d.kpar", i)),
			[2]string{"Probe.sysml", probeModel}, [2]string{".project.json", probeProject}, [2]string{".meta.json", probeMeta})
		if _, err := openCached(paths[i]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := openCached(paths[0]); err != nil { // p1 is now the least recently read
		t.Fatal(err)
	}

	archives.mu.Lock()
	defer archives.mu.Unlock()
	one := archives.open[paths[0]].bytes
	if one == 0 {
		t.Fatal("a cached archive accounts for no bytes")
	}
	before := archives.bytes
	evictArchives(paths[2], before-1)
	if _, ok := archives.open[paths[1]]; ok {
		t.Error("the least recently read archive was kept")
	}
	for _, kept := range []string{paths[0], paths[2]} {
		if _, ok := archives.open[kept]; !ok {
			t.Errorf("%s was evicted though one eviction met the budget", filepath.Base(kept))
		}
	}
	if archives.bytes != before-one {
		t.Errorf("the cache accounts for %d bytes, want %d", archives.bytes, before-one)
	}
	evictArchives(paths[2], 0)
	if _, ok := archives.open[paths[2]]; !ok {
		t.Error("the archive being read was evicted")
	}
}
