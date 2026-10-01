//go:build !sysml_prod && !sysml_noprofile

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
)

func init() {
	profileFeature.link(func(fs *flag.FlagSet) {
		fs.StringVar(&cpuProfilePath, "cpuprofile", "", "Write a CPU profile of the run to this file, for go tool pprof")
		fs.StringVar(&memProfilePath, "memprofile", "", "Write a heap profile of the run to this file, for go tool pprof")
	})
}

// startPprof begins the -cpuprofile and -memprofile profiles, returning the
// functions that end them in the order they were started.
func startPprof() ([]func(), error) {
	var ends []func()
	stop := func() {
		for i := len(ends) - 1; i >= 0; i-- {
			ends[i]()
		}
	}

	if cpuProfilePath != "" {
		// #nosec G304 -- the profile is written where the command line says.
		f, err := os.Create(cpuProfilePath)
		if err != nil {
			return nil, fmt.Errorf("-cpuprofile: %w", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("-cpuprofile: %w", err)
		}
		ends = append(ends, func() {
			pprof.StopCPUProfile()
			if err := f.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "sysml: -cpuprofile:", err)
			}
		})
	}

	if memProfilePath != "" {
		// #nosec G304 -- the profile is written where the command line says.
		f, err := os.Create(memProfilePath)
		if err != nil {
			stop()
			return nil, fmt.Errorf("-memprofile: %w", err)
		}
		ends = append(ends, func() {
			// The profile is written where the run ends, at which point the model it
			// loaded is unreachable: what it records of use is where the run allocated,
			// read with `go tool pprof -sample_index=alloc_space`.
			if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
				fmt.Fprintln(os.Stderr, "sysml: -memprofile:", err)
			}
			if err := f.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "sysml: -memprofile:", err)
			}
		})
	}
	return ends, nil
}
