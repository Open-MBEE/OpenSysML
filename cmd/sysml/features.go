package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

// feature is a group a build leaves out under sysml_prod or sysml_no<name>; its
// files link it from init, and without them its flags are hidden and refused.
type feature struct {
	// name is the group's tag suffix, as "built without <name>" reports it.
	name string
	// flags are the flags the group declares when linked.
	flags []featureFlag
	// values are values of core flags only the group carries out, as -from xmi.
	values map[string][]string
	// sections are the titles of the help sections only the group documents.
	sections []string
	// markers mark a shared section's example or paragraph as the group's.
	markers []string

	linked  bool
	declare func(*flag.FlagSet)
}

// featureFlag is a flag a feature owns; a stand-in must know whether it takes an argument.
type featureFlag struct {
	name    string
	boolean bool
}

func flags(names ...string) []featureFlag {
	out := make([]featureFlag, len(names))
	for i, name := range names {
		out[i] = featureFlag{name: name}
	}
	return out
}

func boolFlags(names ...string) []featureFlag {
	out := flags(names...)
	for i := range out {
		out[i].boolean = true
	}
	return out
}

// The feature groups, one per sysml_no<name> tag; sysml_prod leaves out all.
var (
	v1Feature = &feature{
		name:    "v1",
		flags:   flags("migration-report", "migration-results", "layout", "image-base-url"),
		values:  map[string][]string{"from": {"xmi", "uml", "mdzip"}},
		markers: []string{"SysML v1"},
	}
	syncFeature = &feature{
		name: "sync",
		flags: append(flags("sync-diff", "sync-apply", "sync-base", "sync-state", "sync-annotate"),
			boolFlags("sync-confirm-deletes", "sync-mint-ids")...),
		sections: []string{"Syncing against a repository"},
		markers:  []string{"flexo://", "Flexo"},
	}
	codegenFeature = &feature{
		name:     "codegen",
		flags:    append(flags("compile", "target"), boolFlags("source")...),
		sections: []string{"Native compilation"},
	}
	docpdfFeature = &feature{
		name: "docpdf",
		flags: append(flags("pdf-engine", "html-theme", "html-css", "html-mermaid", "html-math"),
			boolFlags("doc-title-page", "doc-toc", "doc-number-sections",
				"html-no-default-css", "html-fragment", "html-default-css",
				"pdf-title-page", "pdf-toc", "pdf-number-sections")...),
		values:  map[string][]string{"doc-form": {"html", "pdf"}},
		markers: []string{"-doc-form html", "-doc-form pdf"},
	}
	fmiFeature = &feature{
		name:   "fmi",
		values: map[string][]string{"from": {"fmu"}},
	}
	profileFeature = &feature{
		name:  "profile",
		flags: flags("cpuprofile", "memprofile"),
	}
	replextFeature = &feature{
		name:  "replext",
		flags: flags("query"),
	}
)

// features lists every group, in the order their flags are declared.
var features = []*feature{v1Feature, syncFeature, codegenFeature, docpdfFeature, fmiFeature, profileFeature, replextFeature}

// link records the feature's files are in the build; declare is nil for a group owning no flag.
func (f *feature) link(declare func(*flag.FlagSet)) {
	f.linked, f.declare = true, declare
}

// declareFeatureFlags declares a linked feature's flags, and hidden stand-ins for the rest.
func declareFeatureFlags(fs *flag.FlagSet) {
	for _, f := range features {
		if !f.linked {
			for _, ff := range f.flags {
				fs.Var(&omittedFlag{feature: f, boolean: ff.boolean}, ff.name, "")
			}
			continue
		}
		if f.declare == nil {
			continue
		}
		before := countFlags(fs)
		f.declare(fs)
		if countFlags(fs)-before != len(f.flags) {
			panic(fmt.Sprintf("feature %s declares flags its entry does not list", f.name))
		}
		for _, ff := range f.flags {
			if declared := fs.Lookup(ff.name); declared == nil || isBoolFlag(declared) != ff.boolean {
				panic(fmt.Sprintf("feature %s lists -%s, which it does not declare as listed", f.name, ff.name))
			}
		}
	}
}

func countFlags(fs *flag.FlagSet) int {
	n := 0
	fs.VisitAll(func(*flag.Flag) { n++ })
	return n
}

func isBoolFlag(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// omittedFlag stands in for a left-out feature's flag; omittedUse refuses it.
type omittedFlag struct {
	feature *feature
	boolean bool
}

func (*omittedFlag) String() string { return "" }

func (*omittedFlag) Set(string) error { return nil }

func (o *omittedFlag) IsBoolFlag() bool { return o.boolean }

// omittedUse reports the first flag or flag value given of a left-out feature.
func omittedUse(fs *flag.FlagSet) error {
	var err error
	fs.Visit(func(f *flag.Flag) {
		if err != nil {
			return
		}
		if o, ok := f.Value.(*omittedFlag); ok {
			err = fmt.Errorf("-%s is not available in this build (built without %s)", f.Name, o.feature.name)
			return
		}
		value := strings.ToLower(strings.TrimSpace(f.Value.String()))
		for _, feat := range features {
			if feat.linked {
				continue
			}
			for _, omitted := range feat.values[f.Name] {
				if value == omitted {
					err = fmt.Errorf("-%s %s is not available in this build (built without %s)", f.Name, omitted, feat.name)
					return
				}
			}
		}
	})
	return err
}

// withoutOmitted drops from the help what documents a left-out feature; a build
// linking every feature gets d as it is.
func withoutOmitted(d usage.Doc) usage.Doc {
	var omitted []*feature
	for _, f := range features {
		if !f.linked {
			omitted = append(omitted, f)
		}
	}
	if len(omitted) == 0 {
		return d
	}
	owned := func(text string) bool {
		for _, f := range omitted {
			if f.mentionedIn(text) {
				return true
			}
		}
		return false
	}
	ownedFlag := func(name string) bool {
		for _, f := range omitted {
			for _, ff := range f.flags {
				if ff.name == name {
					return true
				}
			}
		}
		return false
	}
	ownedSection := func(title string) bool {
		for _, f := range omitted {
			for _, s := range f.sections {
				if s == title {
					return true
				}
			}
		}
		return false
	}

	var groups []usage.OptionGroup
	for _, g := range d.Options {
		var opts []usage.Option
		for _, o := range g.Options {
			if !ownedFlag(o.Name) {
				opts = append(opts, o)
			}
		}
		if len(opts) > 0 {
			g.Options = opts
			groups = append(groups, g)
		}
	}
	d.Options = groups

	var sections []usage.Section
	for _, s := range d.Sections {
		if ownedSection(s.Title) {
			continue
		}
		var examples []usage.Example
		for _, e := range s.Examples {
			if !owned(e.Command) {
				examples = append(examples, e)
			}
		}
		var paragraphs []string
		for _, p := range s.Paragraphs {
			if !owned(p) {
				paragraphs = append(paragraphs, p)
			}
		}
		s.Examples, s.Paragraphs = examples, paragraphs
		sections = append(sections, s)
	}
	d.Sections = sections
	return d
}

// mentionedIn reports whether text uses one of the feature's flags or markers.
func (f *feature) mentionedIn(text string) bool {
	for _, m := range f.markers {
		if strings.Contains(text, m) {
			return true
		}
	}
	for _, ff := range f.flags {
		if mentionsFlag(text, ff.name) {
			return true
		}
	}
	return false
}

func mentionsFlag(text, name string) bool {
	token := "-" + name
	for i := 0; i < len(text); {
		j := strings.Index(text[i:], token)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(token)
		if (start == 0 || !isFlagByte(text[start-1])) && (end == len(text) || !isFlagByte(text[end])) {
			return true
		}
		i = start + 1
	}
	return false
}

func isFlagByte(b byte) bool {
	return b == '-' || b == '_' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || '0' <= b && b <= '9'
}

// fromUsage is -from's usage, naming the formats this build reads.
func fromUsage() string {
	text := "Input format for -convert: sysml, kerml, ttl, turtle, rdf, api-json"
	if fmiFeature.linked {
		text += ", fmu for a Functional Mock-up Unit to import"
	}
	if v1Feature.linked {
		text += ", or xmi, uml or mdzip for a SysML v1 model to migrate (experimental)"
	}
	return text + "; default the input's extension"
}

// outputUsage is -output's usage, naming the modes this build writes.
func outputUsage() string {
	modes := "-convert, -render or -render-document"
	if codegenFeature.linked {
		modes = "-convert, -compile, -render or -render-document"
	}
	text := "Write what " + modes + " produces to this file instead of stdout"
	if syncFeature.linked {
		text += "; with -convert ttl, a Flexo branch URL pushes the graph to the branch"
	}
	return text
}

// docFormUsage is -doc-form's usage, naming the forms this build writes.
func docFormUsage() string {
	if docpdfFeature.linked {
		return "Form the documents are written in: markdown (default), html or pdf, which drives an external converter"
	}
	return "Form the documents are written in: markdown, the only form this build writes"
}

// docNumberFiguresUsage is -doc-number-figures' usage in this build.
func docNumberFiguresUsage() string {
	if docpdfFeature.linked {
		return "Number the figures and tables in their captions, Figure 1. and Table 1. in document order (markdown, html or pdf)"
	}
	return "Number the figures and tables in their captions, Figure 1. and Table 1. in document order"
}

// convertUsage is -convert's usage, naming the inputs this build reads.
func convertUsage() string {
	text := "Convert the model to this format instead of running it: sysml, kerml, ttl, turtle, rdf or api-json (RDF and the API element form are experimental)"
	if syncFeature.linked {
		text += ". The input may be a Flexo branch URL (host[:port][/base]/projects/{p}/branches/{b} of the FLEXO_SYSMLV2_URL endpoint, or flexo://{p}/{b}), read as its RDF graph"
	}
	return text
}
