// Command stress-model writes a large generated satellite-network model to stdout,
// or one file per orbital plane; see docs/project/satellite-network-stress-test.md.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/stressmodel"
)

func main() {
	planes := flag.Int("planes", 4, "orbital planes in the constellation")
	perPlane := flag.Int("satellites", 8, "satellites in each orbital plane")
	stations := flag.Int("ground-stations", 3, "ground stations the constellation downlinks to")
	stats := flag.Bool("stats", false, "report on stderr what the model declares")
	split := flag.String("split-planes", "", "write one .sysml per orbital plane, beside the library and the constellation, into this directory instead of stdout")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "stress-model: unexpected argument %q\n", flag.Arg(0))
		os.Exit(2)
	}
	if *planes < 1 || *perPlane < 1 || *stations < 0 {
		fmt.Fprintln(os.Stderr, "stress-model: -planes and -satellites must be at least 1 and -ground-stations at least 0")
		os.Exit(2)
	}
	n := stressmodel.SatelliteNetwork{Planes: *planes, Satellites: *perPlane, GroundStations: *stations}
	var (
		s   stressmodel.Stats
		err error
	)
	if *split != "" {
		s, err = writeSplit(n, *split)
	} else {
		s, err = n.Generate(os.Stdout)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "stress-model: %v\n", err)
		os.Exit(1)
	}
	if *stats {
		fmt.Fprintf(os.Stderr, "satellites=%d ground-stations=%d components=%d connections=%d requirements=%d elements=%d bytes=%d\n",
			s.Satellites, s.GroundStations, s.Components, s.Connections, s.Requirements, s.Elements, s.Bytes)
	}
}

// manifestName is the file in a -split-planes directory listing what the last
// generation wrote there, so the next one removes only its own files.
const manifestName = ".stress-model-files"

// writeSplit writes the network one file per plane into dir, creating it, and
// removes what an earlier generation wrote there that this one did not.
func writeSplit(n stressmodel.SatelliteNetwork, dir string) (stressmodel.Stats, error) {
	files, stats := n.Split()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return stats, err
	}
	previous, err := readManifest(dir)
	if err != nil {
		return stats, err
	}
	written := make(map[string]bool, len(files))
	var manifest strings.Builder
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.Name), []byte(f.Source), 0o600); err != nil {
			return stats, err
		}
		written[f.Name] = true
		manifest.WriteString(f.Name + "\n")
	}
	for _, name := range previous {
		if written[name] {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
			continue
		}
		if err != nil {
			return stats, err
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return stats, err
		}
	}
	return stats, os.WriteFile(filepath.Join(dir, manifestName), []byte(manifest.String()), 0o600)
}

// readManifest returns the file names the last generation into dir recorded;
// none when there was no generation. Only plain names in dir are honored.
func readManifest(dir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestName)) // #nosec G304 -- the output directory is named on the command line.
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range strings.Split(string(data), "\n") {
		if name != "" && name != manifestName && filepath.Base(name) == name {
			names = append(names, name)
		}
	}
	return names, nil
}
