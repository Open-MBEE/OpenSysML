package stressmodel

import (
	"fmt"
	"strings"
)

// File is one document of a network split across files.
type File struct {
	Name, Source string
}

// Split generates the network Generate writes as one document per orbital plane
// beside the shared library and the constellation joining the planes. Each plane
// imports the library by qualified name, so a file resolves against the others.
// The fleet form declares its planes inside the network, so it splits into the
// library and the constellation alone.
func (n SatelliteNetwork) Split() ([]File, Stats) {
	var b strings.Builder
	g := &generator{b: &b}
	take := func(name string) File {
		f := File{Name: name, Source: b.String()}
		g.stats.Bytes += b.Len()
		b.Reset()
		return f
	}
	files := make([]File, 0, n.Planes+2)

	g.library()
	g.line(0, "}")
	files = append(files, take("library.sysml"))

	if n.Fleet {
		g.decl(0, "package Constellation {")
		g.planeImports()
		g.line(0, "")
		g.fleetBody(n)
		g.line(0, "}")
		return append(files, take("constellation.sysml")), g.stats
	}

	id := 0
	for p := 0; p < n.Planes; p++ {
		g.decl(0, "package Plane%d {", p)
		g.planeImports()
		g.line(0, "")
		for s := 0; s < n.Satellites; s++ {
			g.satellite(id, p, s)
			id++
		}
		g.line(0, "}")
		files = append(files, take(fmt.Sprintf("plane%03d.sysml", p)))
	}

	g.decl(0, "package Constellation {")
	g.planeImports()
	g.line(1, "private import SatelliteNetwork::*;")
	for p := 0; p < n.Planes; p++ {
		g.line(1, "private import Plane%d::*;", p)
	}
	g.line(0, "")
	for k := 0; k < n.GroundStations; k++ {
		g.groundStation(k)
	}
	g.network(n)
	g.line(0, "}")
	files = append(files, take("constellation.sysml"))
	return files, g.stats
}

// planeImports writes what a file outside the library package imports of it.
func (g *generator) planeImports() {
	g.line(1, "private import ScalarValues::*;")
	g.line(1, "private import ISQ::*;")
	g.line(1, "private import SI::*;")
	g.line(1, "private import SatelliteNetwork::Interfaces::*;")
	g.line(1, "private import SatelliteNetwork::Platform::*;")
	g.line(1, "private import SatelliteNetwork::Requirements::*;")
	g.line(1, "private import SatelliteNetwork::Behavior::*;")
}
