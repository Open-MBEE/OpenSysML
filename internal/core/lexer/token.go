// Package lexer is a hand-written pull-based scanner for SysML v2 / KerML.
package lexer

import "github.com/Open-MBEE/OpenSysML/internal/core/source"

// Kind enumerates token categories.
type Kind int

const (
	Invalid Kind = iota
	EOF

	// Literals & names
	Identifier       // ID: [A-Za-z_][A-Za-z_0-9]*
	UnrestrictedName // '...'
	String           // "..."
	Decimal          // [0-9]+
	Real             // 1.5, .5, 1e3, 1.5e-2

	// Trivia
	Whitespace     // WS (hidden)
	SLNote         // // ...   (hidden)
	MLNote         // //* */   (hidden)
	RegularComment // /* */    (NOT hidden)

	// Punctuation / operators
	Question    // ?
	QuestionQ   // ??
	Pipe        // |
	Amp         // &
	EqEq        // ==
	NotEq       // !=
	EqEqEq      // ===
	NotEqEq     // !==
	At          // @
	AtAt        // @@
	Lt          // <
	Gt          // >
	Le          // <=
	Ge          // >=
	DotDot      // ..
	Plus        // +
	Minus       // -
	Star        // *
	Slash       // /
	Percent     // %
	StarStar    // **
	Caret       // ^
	Tilde       // ~
	Dot         // .
	Hash        // #
	LParen      // (
	RParen      // )
	LBracket    // [
	RBracket    // ]
	Arrow       // ->
	DotQuestion // .?
	Comma       // ,
	ColonColon  // ::
	Dollar      // $
	Eq          // =
	LBrace      // {
	RBrace      // }
	Semicolon   // ;
	Colon       // :
	ColonEq     // :=

	ColonGt      // :>
	ColonGtGt    // :>>
	ColonColonGt // ::>
	EqGt         // =>

	Keyword // generic keyword marker; specific keyword identity via Token.KeywordID

	Error // illegal char / unterminated literal
)

var kindNames = map[Kind]string{
	Invalid: "Invalid", EOF: "EOF",
	Identifier: "Identifier", UnrestrictedName: "UnrestrictedName",
	String: "String", Decimal: "Decimal", Real: "Real",
	Whitespace: "Whitespace", SLNote: "SLNote", MLNote: "MLNote",
	RegularComment: "RegularComment",
	Question:       "?", QuestionQ: "??", Pipe: "|", Amp: "&",
	EqEq: "==", NotEq: "!=", EqEqEq: "===", NotEqEq: "!==",
	At: "@", AtAt: "@@", Lt: "<", Gt: ">", Le: "<=", Ge: ">=",
	DotDot: "..", Plus: "+", Minus: "-", Star: "*", Slash: "/",
	Percent: "%", StarStar: "**", Caret: "^", Tilde: "~", Dot: ".",
	Hash: "#", LParen: "(", RParen: ")", LBracket: "[", RBracket: "]",
	Arrow: "->", DotQuestion: ".?", Comma: ",", ColonColon: "::",
	Dollar: "$", Eq: "=", LBrace: "{", RBrace: "}", Semicolon: ";", Colon: ":",
	ColonEq: ":=", ColonGt: ":>", ColonGtGt: ":>>", ColonColonGt: "::>", EqGt: "=>",
	Keyword: "Keyword", Error: "Error",
}

// String returns a human-readable name for the kind.
func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "Kind(" + itoa(k) + ")"
}

func itoa(k Kind) string {
	// tiny local itoa to avoid strconv import churn; fine for error paths
	if k == 0 {
		return "0"
	}
	neg := k < 0
	n := int(k)
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Token is a single lexeme: a Kind plus its source span.
// KeywordID is set only when Kind == Keyword, giving the specific keyword text.
type Token struct {
	Kind      Kind
	Span      source.Span
	KeywordID string // populated for Kind==Keyword
	// Unterminated marks a comment or note the scanner ran to the end of the
	// file without finding its closing "*/", so everything after the opener —
	// possibly the rest of the document — is inside it.
	Unterminated bool
	// BadEscape marks a string or unrestricted name spelling a backslash escape
	// outside the set its terminal admits; InvalidEscapes locates them.
	BadEscape bool
}

// IsTrivia reports whether the token is hidden trivia (skipped by the parser).
// WS, SLNote, MLNote are hidden. RegularComment is NOT hidden.
func (t Token) IsTrivia() bool {
	switch t.Kind {
	case Whitespace, SLNote, MLNote:
		return true
	default:
		return false
	}
}
