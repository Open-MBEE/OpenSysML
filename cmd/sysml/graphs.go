package main

import (
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// runGraphs writes the lowered graph of the action or state machine -graphs
// names, as the canonical graphs:1 JSON an external engine is sent, to stdout
// or to -output.
func runGraphs(files []string) error {
	sess, err := loadArtifactModel(files, "nothing was exported")
	if err != nil {
		return err
	}
	raw, err := sess.Graphs(graphsSubject)
	if err != nil {
		return fmt.Errorf("-graphs %s: %w", graphsSubject, err)
	}
	if outputPath == "" {
		_, err := os.Stdout.Write(raw)
		return err
	}
	return writeOutputFile(outputPath, raw, fmt.Sprintf("graphs:%d", modelform.GraphsVersion))
}

// writeMetaclassGraph writes the Turtle committed as
// internal/translate/rdf/ontology/sysml-metaclasses.ttl to stdout or to -output.
func writeMetaclassGraph() error {
	out := rdf.WriteTurtle(ontology.MetaclassGraph())
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return err
	}
	return writeOutputFile(outputPath, out, "metaclass hierarchy")
}

// writeMetamodelTable writes the generated metamodel table as JSON to stdout or to -output.
func writeMetamodelTable() error {
	out, err := ontology.MarshalTable()
	if err != nil {
		return err
	}
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return err
	}
	return writeOutputFile(outputPath, out, "metamodel table")
}
