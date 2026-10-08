package main

import (
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
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
