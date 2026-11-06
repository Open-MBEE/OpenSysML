package doccounts

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SpecComplianceDir holds the compliance map's sources, one per area, which
// scripts/spec_compliance.py combines into SpecCompliancePath when the site is
// built and when a release is prepared; develop does not commit the page.
const SpecComplianceDir = "docs/project/spec-compliance"

var (
	sourceNamePattern  = regexp.MustCompile(`^[0-9]{2}-[a-z0-9][a-z0-9-]*\.md$`)
	sectionNamePattern = regexp.MustCompile(`^[0-9]{2}-[a-z0-9][a-z0-9-]*$`)
	sourceHeading      = regexp.MustCompile(`^#{1,6} \S`)
)

// SpecComplianceSources lists the sources in page order as slash paths under
// root: `<NN>-<slug>.md` files at the top of SpecComplianceDir or one level down
// in a `<NN>-<slug>` section directory. A README.md is skipped; anything else
// in the directory is an error, so a misnamed source cannot silently drop out.
func SpecComplianceSources(root string) ([]string, error) {
	dir := filepath.Join(root, filepath.FromSlash(SpecComplianceDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var sources []string
	for _, entry := range entries {
		rel := path.Join(SpecComplianceDir, entry.Name())
		switch {
		case entry.IsDir():
			if !sectionNamePattern.MatchString(entry.Name()) {
				return nil, fmt.Errorf("%s: a section directory is named <NN>-<slug>", rel)
			}
			nested, err := os.ReadDir(filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, err
			}
			if len(nested) == 0 {
				return nil, fmt.Errorf("%s: an empty section directory", rel)
			}
			for _, sub := range nested {
				if sub.IsDir() || !sourceNamePattern.MatchString(sub.Name()) {
					return nil, fmt.Errorf("%s: a source is a file named <NN>-<slug>.md", path.Join(rel, sub.Name()))
				}
				sources = append(sources, path.Join(rel, sub.Name()))
			}
		case entry.Name() == "README.md":
		case !sourceNamePattern.MatchString(entry.Name()):
			return nil, fmt.Errorf("%s: a source is a file named <NN>-<slug>.md", rel)
		default:
			sources = append(sources, rel)
		}
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("%s: no sources", SpecComplianceDir)
	}
	sort.Strings(sources)
	return sources, nil
}

// ReadSpecComplianceSource reads one source, which holds exactly the text the page shows at
// its place: it starts with a heading and ends with one newline.
func ReadSpecComplianceSource(root, source string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source))) // #nosec G304 -- a source SpecComplianceSources listed
	if err != nil {
		return "", err
	}
	text := string(content)
	if !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") {
		return "", fmt.Errorf("%s: a source ends with exactly one newline", source)
	}
	if !sourceHeading.MatchString(text) {
		return "", fmt.Errorf("%s: a source starts with a heading", source)
	}
	return text, nil
}

// ReadSpecCompliance combines the sources into the page's text, byte for byte
// what scripts/spec_compliance.py renders: the sources in order, a blank line
// between them.
func ReadSpecCompliance(root string) (string, error) {
	sources, err := SpecComplianceSources(root)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		text, err := ReadSpecComplianceSource(root, source)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n"), nil
}

// CountSpecComplianceRules counts the rule rows across the sources, so a drift
// is reported at the source and line that carries it.
func CountSpecComplianceRules(root string) (RuleCounts, error) {
	sources, err := SpecComplianceSources(root)
	if err != nil {
		return RuleCounts{}, err
	}
	counts := RuleCounts{}
	for _, source := range sources {
		text, err := ReadSpecComplianceSource(root, source)
		if err != nil {
			return RuleCounts{}, err
		}
		rows, err := ruleRows(source, text)
		if err != nil {
			return RuleCounts{}, err
		}
		for _, row := range rows {
			counts.add(row.Marker)
		}
	}
	counts.total()
	return counts, nil
}
