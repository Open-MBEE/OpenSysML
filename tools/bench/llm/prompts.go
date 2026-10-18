package llm

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed prompts/system.txt prompts/primer.txt prompts/feedback.txt
var promptFS embed.FS

var (
	systemPrompt     = mustRead("prompts/system.txt")
	primerTemplate   = template.Must(template.New("primer").Parse(mustRead("prompts/primer.txt")))
	feedbackTemplate = template.Must(template.New("feedback").Parse(mustRead("prompts/feedback.txt")))
)

func mustRead(name string) string {
	data, err := promptFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// SystemPrompt is the system message every generation starts from.
func SystemPrompt() string { return strings.TrimSpace(systemPrompt) }

// Primer renders the first user message for a task.
func Primer(t Task) string {
	var b strings.Builder
	if err := primerTemplate.Execute(&b, t); err != nil {
		panic(err)
	}
	return strings.TrimSpace(b.String())
}

// Feedback renders the repair message that carries the compiler's diagnostics
// verbatim.
func Feedback(diags []Diagnostic) string {
	var b strings.Builder
	if err := feedbackTemplate.Execute(&b, struct{ Diagnostics []Diagnostic }{diags}); err != nil {
		panic(err)
	}
	return strings.TrimSpace(b.String())
}

// ExtractModel strips Markdown fences a model may have wrapped its answer in.
func ExtractModel(answer string) string {
	s := strings.TrimSpace(answer)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	lines = lines[1:]
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			lines = lines[:i]
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
