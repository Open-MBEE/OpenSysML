// Command stress-model writes a large generated satellite-network model to stdout,
// or one file per orbital plane; see docs/project/satellite-network-stress-test.md.
package main

import (
	"crypto/sha256"
	"encoding/hex"
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
// generation wrote there, each file with the digest of its content, so the
// next one removes only its own files and only while they read as written.
const manifestName = ".stress-model-files"

// digest is the manifest's identity of a file's content.
func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// writeSplit writes the network one file per plane into dir, creating it, and
// removes what an earlier generation wrote there that this one did not. The
// files are staged beside their places and each is recorded in the manifest
// before it is moved in, so a generation that fails leaves nothing unrecorded;
// a record whose move never happened names a file that does not read as recorded.
func writeSplit(n stressmodel.SatelliteNetwork, dir string) (stressmodel.Stats, error) {
	files, stats := n.Split()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return stats, err
	}
	previous, err := readManifest(dir)
	if err != nil {
		return stats, err
	}
	if err := replaceable(dir, files, previous); err != nil {
		return stats, err
	}
	staging, err := os.MkdirTemp(dir, ".stress-model-*")
	if err != nil {
		return stats, err
	}
	defer os.RemoveAll(staging)
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(staging, f.Name), []byte(f.Source), 0o600); err != nil {
			return stats, err
		}
	}
	manifest, err := os.OpenFile(filepath.Join(dir, manifestName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600) // #nosec G304 -- the output directory is named on the command line.
	if err != nil {
		return stats, err
	}
	written := make(map[string]bool, len(files))
	var current strings.Builder
	for _, f := range files {
		line := digest([]byte(f.Source)) + " " + f.Name + "\n"
		current.WriteString(line)
		if _, err := manifest.WriteString(line); err != nil {
			return stats, errors.Join(err, manifest.Close())
		}
		if err := os.Rename(filepath.Join(staging, f.Name), filepath.Join(dir, f.Name)); err != nil {
			return stats, errors.Join(err, manifest.Close())
		}
		written[f.Name] = true
	}
	if err := manifest.Close(); err != nil {
		return stats, err
	}
	for _, rec := range previous {
		if written[rec.Name] {
			continue
		}
		if err := removeIfRecorded(filepath.Join(dir, rec.Name), rec.Digest); err != nil {
			return stats, err
		}
	}
	return stats, os.WriteFile(filepath.Join(dir, manifestName), []byte(current.String()), 0o600)
}

// replaceable reports an error naming every file the generation would write
// over that the last generation did not write, or that has changed since: the
// generator replaces only its own unedited output, and writes nothing otherwise.
func replaceable(dir string, files []stressmodel.File, previous []record) error {
	recorded := make(map[string]string, len(previous))
	for _, rec := range previous {
		recorded[rec.Name] = rec.Digest
	}
	var errs []error
	for _, f := range files {
		path := filepath.Join(dir, f.Name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		want, ok := recorded[f.Name]
		switch {
		case !info.Mode().IsRegular():
			errs = append(errs, fmt.Errorf("%s: not a regular file", path))
		case !ok:
			errs = append(errs, fmt.Errorf("%s: not written by the last generation into %s", path, dir))
		default:
			content, err := os.ReadFile(path) // #nosec G304 -- path is a generated name under the output directory.
			if err != nil {
				return err
			}
			if digest(content) != want {
				errs = append(errs, fmt.Errorf("%s: changed since the last generation wrote it", path))
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("nothing written: %w", errors.Join(errs...))
}

// removeIfRecorded removes the regular file at path when its content still
// has the recorded digest; anything else standing there is not the generator's.
func removeIfRecorded(path, recorded string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	content, err := os.ReadFile(path) // #nosec G304 -- path is a recorded name under the output directory.
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if digest(content) != recorded {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// record is one manifest line: a file the last generation wrote, and the
// digest of what it wrote there.
type record struct {
	Name   string
	Digest string
}

// readManifest returns what the last generation into dir recorded; nothing
// when there was no generation. Only plain names in dir with a digest are honored.
func readManifest(dir string) ([]record, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestName)) // #nosec G304 -- the output directory is named on the command line.
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []record
	for _, line := range strings.Split(string(data), "\n") {
		sum, name, ok := strings.Cut(line, " ")
		if !ok || sum == "" || name == "" || name == manifestName || filepath.Base(name) != name {
			continue
		}
		records = append(records, record{Name: name, Digest: sum})
	}
	return records, nil
}
