package docpdf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// plantumlRasterizer draws PlantUML blocks to SVG with the PlantUML jar, run
// by java in pipe mode. Both are optional: without the jar or java, a
// document's PlantUML blocks are kept as source.
type plantumlRasterizer struct {
	java string
	jar  string
	dot  string
}

func (*plantumlRasterizer) name() string { return "plantuml" }

func (p *plantumlRasterizer) prepare(string) error {
	jar, err := locatePlantUMLJar()
	if err != nil {
		return err
	}
	java, err := javaTool.locate("")
	if err != nil {
		return err
	}
	p.jar, p.java = jar, java
	// PlantUML lays most diagrams out with Graphviz; the dot this backend was
	// pointed at is the one it should use too.
	if dot, err := graphvizTool.locate(""); err == nil {
		p.dot = dot
	}
	return nil
}

func (p *plantumlRasterizer) draw(dir, source, output string) error {
	var svg bytes.Buffer
	run := toolRun{
		dir:    dir,
		path:   p.java,
		args:   []string{"-Djava.awt.headless=true", "-jar", p.jar, "-tsvg", "-pipe"},
		stdin:  strings.NewReader(source + "\n"),
		stdout: &svg,
		detail: plantumlDetail,
	}
	if p.dot != "" {
		run.env = []string{"GRAPHVIZ_DOT=" + p.dot}
	}
	if err := run.run(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, output), svg.Bytes(), 0o600)
}

// locatePlantUMLJar finds the jar PlantUMLJarEnv names; unset, or naming no
// file, is the jar missing.
func locatePlantUMLJar() (string, error) {
	jar := strings.TrimSpace(os.Getenv(PlantUMLJarEnv))
	if jar == "" {
		return "", &Error{Kind: ErrorToolMissing, Tool: "the PlantUML jar", EnvVar: PlantUMLJarEnv}
	}
	if info, err := os.Stat(jar); err != nil || info.IsDir() {
		return "", &Error{Kind: ErrorToolMissing, Tool: jar, EnvVar: PlantUMLJarEnv}
	}
	return jar, nil
}

// plantumlDetail keeps what PlantUML says of a failure, dropping the JVM's
// own notes (preferences, logging) that precede it on stderr.
func plantumlDetail(stderr string) string {
	var kept []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, "INFO:") || strings.HasPrefix(line, "WARNING:") || strings.Contains(line, "java.util.prefs") {
			continue
		}
		kept = append(kept, line)
	}
	return tail(strings.TrimSpace(strings.Join(kept, "\n")))
}
