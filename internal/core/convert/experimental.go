package convert

import "strings"

// ExperimentalNotice is the wording every surface reports the RDF mapping's
// status in, so the CLI, the REPL, the service and the docs agree.
const ExperimentalNotice = "RDF conversion is experimental: the mapping covers model structure and the " +
	"behavior its bodies state, refuses what it cannot write back, and its vocabulary may change " +
	"without a compatibility path; see docs/reference/rdf-mapping.md § Status"

// MigrationNotice is the wording every surface reports the SysML v1 migration's
// status in.
const MigrationNotice = "SysML v1 migration is experimental: the mapping covers structure, ports and " +
	"connectors, requirements, constraints, instances and allocations, reports every element it " +
	"approximates or leaves behind, and what it writes for a v1 element may change without a " +
	"compatibility path; see docs/reference/sysml-v1-migration.md § Status"

// IsExperimental reports whether a conversion between these formats goes
// through the RDF mapping or the SysML v1 migration. Notation to notation does not.
func IsExperimental(from, to Format) bool {
	return from == FormatTurtle || to == FormatTurtle || from == FormatXMI
}

// Notices lists the experimental notices a conversion between these formats
// carries, migration first; empty when the conversion is stable.
func Notices(from, to Format) []string {
	var notices []string
	if from == FormatXMI {
		notices = append(notices, MigrationNotice)
	}
	if from == FormatTurtle || to == FormatTurtle {
		notices = append(notices, ExperimentalNotice)
	}
	return notices
}

// Notice joins the notices of a conversion into one string, one per line.
func Notice(from, to Format) string {
	return strings.Join(Notices(from, to), "\n")
}
