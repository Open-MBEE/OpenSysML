package main

import (
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// convertLibrary converts the bundled standard library, every file of it as
// one model, to -convert's RDF form: to stdout, to the -o file, or pushed to
// the repository branch -o names. Its ids are the library's normative ones;
// the derived ones are uuids too unless -id says otherwise.
func convertLibrary(to convert.Format) (int, error) {
	if to != convert.FormatTurtle && to != convert.FormatAPIJSON {
		return 0, fmt.Errorf("-stdlib converts the standard library to ttl or api-json, not %s", to)
	}
	opts, err := convertOptions(convert.FormatSysML, to)
	if err != nil {
		return 0, err
	}
	if idForm == "" {
		opts.ID = export.IDUUID
	}
	toBranch := false
	if outputPath != "" {
		_, isURL, err := branchURL(outputPath)
		if err != nil {
			return 0, err
		}
		toBranch = isURL
	}
	if !toBranch && syncState != "" {
		return 0, fmt.Errorf("-sync-state records a repository branch's head; -o names no branch")
	}
	inputs, err := libraryInputs()
	if err != nil {
		return 0, err
	}
	for _, notice := range convert.Notices(convert.FormatSysML, to) {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
	graph, err := convert.ModelGraph(inputs, opts)
	if err != nil {
		return 0, err
	}
	if toBranch {
		return pushLibrary(graph, to, len(inputs))
	}
	out, err := convert.FromGraphWith(graph, to, opts)
	if err != nil {
		return 0, err
	}
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return exitHolds, err
	}
	if err := writeConversion(outputPath, out, to); err != nil {
		return 0, err
	}
	return exitHolds, nil
}

// libraryInputs is the bundled standard library as one model's conversion inputs.
func libraryInputs() ([]convert.Input, error) {
	files, err := libs.BundledModelFiles()
	if err != nil {
		return nil, err
	}
	inputs := make([]convert.Input, 0, len(files))
	for _, file := range files {
		inputs = append(inputs, convert.Input{Name: file.Name, Data: file.Data})
	}
	return inputs, nil
}
