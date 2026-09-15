// Command stress-model writes a large generated SysML v2 model of a stated shape
// and size to stdout, for measuring how the toolchain scales. The one shape today
// is a satellite network — a constellation of fully modeled spacecraft and the
// ground stations they downlink to. See docs/project/satellite-network-stress-test.md.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/stressmodel"
)

func main() {
	planes := flag.Int("planes", 4, "orbital planes in the constellation")
	perPlane := flag.Int("satellites", 8, "satellites in each orbital plane")
	stations := flag.Int("ground-stations", 3, "ground stations the constellation downlinks to")
	stats := flag.Bool("stats", false, "report on stderr what the model declares")
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
	s, err := n.Generate(os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stress-model: %v\n", err)
		os.Exit(1)
	}
	if *stats {
		fmt.Fprintf(os.Stderr, "satellites=%d ground-stations=%d components=%d connections=%d requirements=%d elements=%d bytes=%d\n",
			s.Satellites, s.GroundStations, s.Components, s.Connections, s.Requirements, s.Elements, s.Bytes)
	}
}
