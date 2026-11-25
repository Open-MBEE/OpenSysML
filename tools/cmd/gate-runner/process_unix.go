//go:build !windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func interruptSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }

// setProcessGroup puts the shard in a session of its own. A descendant that
// starts a process group of its own (an analysis engine, say) stays in that
// session, so cancellation can still find it.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

// killProcessGroup signals the shard's whole session: its direct process group
// and any process groups its descendants created while it ran.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	groups := append([]int{cmd.Process.Pid}, sessionProcessGroups(cmd.Process.Pid)...)
	seen := make(map[int]bool, len(groups))
	var errs []error
	for _, pgid := range groups {
		if pgid <= 0 || seen[pgid] {
			continue
		}
		seen[pgid] = true
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			errs = append(errs, fmt.Errorf("signal process group %d: %w", pgid, err))
		}
	}
	return errors.Join(errs...)
}

// sessionProcessGroups lists the process groups of live processes still in sid.
// Without /proc only the direct group, supplied by the caller, is known.
func sessionProcessGroups(sid int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var groups []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		pgid, found, ok := statSession(filepath.Join("/proc", entry.Name(), "stat")) // #nosec G304 -- /proc/<pid>/stat only
		if !ok || found != sid || pgid <= 0 {
			continue
		}
		groups = append(groups, pgid)
	}
	return groups
}

// statSession reads the process group and session from /proc/<pid>/stat. The
// command name in field 2 is parenthesised and may contain spaces or ')'.
func statSession(name string) (pgid, session int, ok bool) {
	stat, err := os.ReadFile(name) // #nosec G304 -- /proc/<pid>/stat only
	if err != nil {
		return 0, 0, false
	}
	end := bytes.LastIndexByte(stat, ')')
	if end < 0 {
		return 0, 0, false
	}
	fields := strings.Fields(string(stat[end+1:]))
	if len(fields) < 4 {
		return 0, 0, false
	}
	pgid, err = strconv.Atoi(fields[2])
	if err != nil {
		return 0, 0, false
	}
	session, err = strconv.Atoi(fields[3])
	return pgid, session, err == nil
}
