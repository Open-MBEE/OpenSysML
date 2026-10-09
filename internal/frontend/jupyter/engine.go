package jupyter

// MIMEBundle is the data of one output, keyed by MIME type, each the
// representation a front end may pick. Text values are strings; a JSON value
// is any value that marshals to the JSON a front end expects.
type MIMEBundle map[string]any

// Output is where an engine writes what a cell produces, in the order produced.
type Output interface {
	// Stream writes text to a stream: "stdout" or "stderr".
	Stream(name, text string)
	// Display shows a rich output.
	Display(data MIMEBundle, metadata map[string]any)
	// Result shows the value of the cell: the last expression's.
	Result(data MIMEBundle)
	// Error reports a failure, with the lines a traceback shows for it.
	Error(name, value string, traceback []string)
}

// ExecError is how an engine reports a cell that failed: the name of the error,
// what it says, and the traceback lines.
type ExecError struct {
	Name      string
	Value     string
	Traceback []string
}

func (e *ExecError) Error() string { return e.Value }

// IsCompleteStatus is the answer to whether a cell is complete.
type IsCompleteStatus string

// The answers the protocol admits.
const (
	Complete   IsCompleteStatus = "complete"
	Incomplete IsCompleteStatus = "incomplete"
	Invalid    IsCompleteStatus = "invalid"
	Unknown    IsCompleteStatus = "unknown"
)

// Completion is the answer to a completion request: the candidates, and the
// span of the code they replace, as rune offsets.
type Completion struct {
	Matches     []string
	CursorStart int
	CursorEnd   int
}

// Inspection is the answer to an inspection request: whether the cursor is on
// something known, and what to show about it.
type Inspection struct {
	Found bool
	Data  MIMEBundle
}

// LanguageInfo is what the kernel tells a front end about its language.
type LanguageInfo struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	MIMEType       string `json:"mimetype"`
	FileExtension  string `json:"file_extension"`
	PygmentsLexer  string `json:"pygments_lexer,omitempty"`
	CodeMirrorMode string `json:"codemirror_mode,omitempty"`
}

// Engine is what the kernel asks of the language: it runs cells and answers
// what a front end asks about code. The kernel calls Execute, Complete,
// IsComplete and Inspect one at a time; Interrupt may be called while Execute
// runs, from another goroutine.
type Engine interface {
	// Execute runs one cell, writing what it produces to out, and reports a
	// failure as an *ExecError; out.Error is the kernel's to call.
	Execute(code string, out Output) error
	Complete(code string, cursor int) Completion
	IsComplete(code string) (IsCompleteStatus, string)
	Inspect(code string, cursor int, detail int) Inspection
	// Interrupt stops the cell running, if one is.
	Interrupt()
	// Shutdown ends the engine; restart says whether another is to follow.
	Shutdown(restart bool)
}
