package formal

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// Main is the formal-constraints command; it returns the process exit status.
func Main(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("formal-constraints", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root above the working directory)")
	pdftotext := flags.String("pdftotext", "pdftotext", "the pdftotext program (poppler) that reads the PDFs")
	list := flags.Bool("list", false, "print the extracted constraints as JSON, in document order, without touching the manifest")
	check := flags.Bool("check", false, "verify the manifest records the documents' constraints as -update would write them")
	update := flags.Bool("update", false, "record the documents' constraints in the manifest, keeping every recorded row's hand-edited fields")
	paths := map[string]*string{}
	for _, key := range []string{"kerml", "sysml"} {
		paths[key] = flags.String(key, "", "the pinned "+strings.ToUpper(key[:1])+key[1:]+" PDF (required)")
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "formal-constraints: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	modes := 0
	for _, on := range []bool{*list, *check, *update} {
		if on {
			modes++
		}
	}
	if modes != 1 {
		fmt.Fprintln(stderr, "formal-constraints: exactly one of -list, -check and -update is required")
		return 2
	}
	root, err := repo.Choose(*repoDir)
	if err != nil {
		fmt.Fprintf(stderr, "formal-constraints: %v\n", err)
		return 1
	}
	m, err := manifest.Load(root)
	if err == nil {
		err = m.Validate()
	}
	if err != nil {
		fmt.Fprintf(stderr, "formal-constraints: %v\n", err)
		return 1
	}
	extracted, err := extractAll(*pdftotext, root, m, paths)
	if err != nil {
		fmt.Fprintf(stderr, "formal-constraints: %v\n", err)
		return 1
	}
	switch {
	case *list:
		err = writeList(stdout, extracted)
	case *update:
		err = runUpdate(root, m, extracted, stdout)
	default:
		err = runCheck(m, extracted, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "formal-constraints: %v\n", err)
		return 1
	}
	return 0
}

// extractAll reads every declared specification's PDF, in key order.
func extractAll(pdftotext, root string, m *manifest.Manifest, paths map[string]*string) ([]Constraint, error) {
	keys := make([]string, 0, len(m.Specifications))
	for key := range m.Specifications {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var all []Constraint
	for _, key := range keys {
		path, ok := paths[key]
		if !ok {
			return nil, fmt.Errorf("the manifest declares specification %q, which this program has no flag for", key)
		}
		if *path == "" {
			return nil, fmt.Errorf("-%s is required: the path of the pinned %s PDF", key, m.Specifications[key].Name)
		}
		constraints, err := Extract(pdftotext, repo.Resolve(root, *path), key, m.Specifications[key])
		if err != nil {
			return nil, err
		}
		all = append(all, constraints...)
	}
	return all, nil
}

func writeList(out io.Writer, extracted []Constraint) error {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(extracted)
}

// totals states the extracted counts per document and per kind.
func totals(extracted []Constraint) string {
	byDocument := map[string]map[string]int{}
	var documents []string
	for _, e := range extracted {
		if byDocument[e.Document] == nil {
			byDocument[e.Document] = map[string]int{}
			documents = append(documents, e.Document)
		}
		byDocument[e.Document][e.Kind]++
		byDocument[e.Document][""]++
	}
	sort.Strings(documents)
	parts := make([]string, 0, len(documents))
	for _, d := range documents {
		kinds := make([]string, 0, len(manifest.Kinds))
		for _, kind := range manifest.Kinds {
			kinds = append(kinds, fmt.Sprintf("%d %s*", byDocument[d][kind], kind))
		}
		parts = append(parts, fmt.Sprintf("%s %d (%s)", d, byDocument[d][""], strings.Join(kinds, ", ")))
	}
	return fmt.Sprintf("%d constraints: %s", len(extracted), strings.Join(parts, "; "))
}

func runUpdate(root string, m *manifest.Manifest, extracted []Constraint, out io.Writer) error {
	next, changed, problems := Merge(m, extracted)
	if err := disagreements(problems); err != nil {
		return fmt.Errorf("%w\nnothing written; correct the pilot rows' citations first", err)
	}
	if err := next.Validate(); err != nil {
		return err
	}
	if changed {
		if err := manifest.Write(root, next); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "formal-constraints: %s; %d spec-sourced rows recorded\n", totals(extracted), len(next.SpecRows()))
	return nil
}

func runCheck(m *manifest.Manifest, extracted []Constraint, out io.Writer) error {
	next, changed, problems := Merge(m, extracted)
	if changed {
		problems = append(problems, staleRows(m, next)...)
		if len(problems) == 0 {
			problems = append(problems, "the formal extraction record is stale")
		}
	}
	if err := disagreements(problems); err != nil {
		return fmt.Errorf("%w\nrun `go run -C tools ./cmd/formal-constraints -update` with the pinned PDFs", err)
	}
	fmt.Fprintf(out, "formal-constraints: %s; the manifest's %d spec-sourced rows are current\n", totals(extracted), len(m.SpecRows()))
	return nil
}

// staleRows names every spec-sourced row -update would add, drop or rewrite.
func staleRows(m, next *manifest.Manifest) []string {
	encoded := func(rows []manifest.Constraint) map[string]string {
		out := make(map[string]string, len(rows))
		for _, c := range rows {
			b, _ := json.Marshal(c)
			out[c.Name] = string(b)
		}
		return out
	}
	have, want := encoded(m.SpecRows()), encoded(next.SpecRows())
	var problems []string
	for name, row := range want {
		switch recorded, ok := have[name]; {
		case !ok:
			problems = append(problems, name+" is stated by the documents but not recorded")
		case recorded != row:
			problems = append(problems, name+" is recorded with another text, clause or metaclass than the documents state")
		}
	}
	for name := range have {
		if _, ok := want[name]; !ok {
			problems = append(problems, name+" is recorded but the documents state no such constraint, or a pilot row now cites it")
		}
	}
	sort.Strings(problems)
	return problems
}

func disagreements(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return errors.New("the manifest does not match the documents:\n  " + strings.Join(problems, "\n  "))
}
