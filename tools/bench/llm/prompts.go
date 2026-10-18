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
	lines := strings.Split(strings.TrimSpace(answer), "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			start = i
			break
		}
	}
	if start < 0 {
		return strings.TrimSpace(answer)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start+1:end], "\n"))
}
