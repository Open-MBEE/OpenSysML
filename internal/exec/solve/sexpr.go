package solve

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// sexpr is one S-expression a solver replied with: an atom or a list.
type sexpr struct {
	// Atom is the token an atom carries, with a `|quoted|` symbol's bars
	// stripped and a string literal's escapes resolved.
	Atom string

	// Quoted marks an atom that was written as a string literal.
	Quoted bool

	// List holds the elements of a list, nil for an atom.
	List []sexpr

	// IsList distinguishes an empty list from an atom.
	IsList bool
}

// errEOF reports that the solver's output ended where a reply was expected.
var errEOF = errors.New("the solver closed its output")

// String renders the expression back as the solver wrote it, for a message
// about a reply that could not be interpreted.
func (s sexpr) String() string {
	if !s.IsList {
		if s.Quoted {
			return `"` + strings.ReplaceAll(s.Atom, `"`, `""`) + `"`
		}
		return s.Atom
	}
	parts := make([]string, 0, len(s.List))
	for _, e := range s.List {
		parts = append(parts, e.String())
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// isError reports whether the expression is an `(error "…")` reply, and the
// message it carries.
func (s sexpr) isError() (string, bool) {
	if !s.IsList || len(s.List) != 2 || s.List[0].Atom != "error" {
		return "", false
	}
	return s.List[1].Atom, true
}

// readSexpr reads one complete S-expression, skipping whitespace and comments
// before it.
func readSexpr(r *bufio.Reader) (sexpr, error) {
	if err := skipSpace(r); err != nil {
		return sexpr{}, err
	}
	c, err := r.ReadByte()
	if err != nil {
		return sexpr{}, ioError(err)
	}
	switch c {
	case '(':
		return readList(r)
	case ')':
		return sexpr{}, fmt.Errorf("unbalanced `)` in the solver's reply")
	case '"':
		text, err := readString(r)
		if err != nil {
			return sexpr{}, err
		}
		return sexpr{Atom: text, Quoted: true}, nil
	case '|':
		text, err := readQuotedSymbol(r)
		if err != nil {
			return sexpr{}, err
		}
		return sexpr{Atom: text}, nil
	default:
		if err := r.UnreadByte(); err != nil {
			return sexpr{}, ioError(err)
		}
		return readAtom(r)
	}
}

// readList reads the rest of a list, its opening parenthesis already consumed.
func readList(r *bufio.Reader) (sexpr, error) {
	out := sexpr{IsList: true}
	for {
		if err := skipSpace(r); err != nil {
			return sexpr{}, err
		}
		c, err := r.ReadByte()
		if err != nil {
			return sexpr{}, ioError(err)
		}
		if c == ')' {
			return out, nil
		}
		if err := r.UnreadByte(); err != nil {
			return sexpr{}, ioError(err)
		}
		elem, err := readSexpr(r)
		if err != nil {
			return sexpr{}, err
		}
		out.List = append(out.List, elem)
	}
}

// readAtom reads a bare atom, which ends at whitespace or a parenthesis.
func readAtom(r *bufio.Reader) (sexpr, error) {
	var b strings.Builder
	for {
		c, err := r.ReadByte()
		if err != nil {
			if b.Len() > 0 && errors.Is(err, io.EOF) {
				return sexpr{Atom: b.String()}, nil
			}
			return sexpr{}, ioError(err)
		}
		if isSpace(c) || c == '(' || c == ')' || c == ';' {
			if err := r.UnreadByte(); err != nil {
				return sexpr{}, ioError(err)
			}
			return sexpr{Atom: b.String()}, nil
		}
		b.WriteByte(c)
	}
}

// readString reads a string literal, its opening quote already consumed. A pair
// of quotes stands for one quote, as SMT-LIB escapes them.
func readString(r *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", ioError(err)
		}
		if c != '"' {
			b.WriteByte(c)
			continue
		}
		next, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return b.String(), nil
			}
			return "", ioError(err)
		}
		if next != '"' {
			if err := r.UnreadByte(); err != nil {
				return "", ioError(err)
			}
			return b.String(), nil
		}
		b.WriteByte('"')
	}
}

// readQuotedSymbol reads a `|…|` symbol, its opening bar already consumed.
func readQuotedSymbol(r *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", ioError(err)
		}
		if c == '|' {
			return b.String(), nil
		}
		b.WriteByte(c)
	}
}

// skipSpace consumes whitespace and `;` comments.
func skipSpace(r *bufio.Reader) error {
	for {
		c, err := r.ReadByte()
		if err != nil {
			return ioError(err)
		}
		if isSpace(c) {
			continue
		}
		if c == ';' {
			for c != '\n' {
				if c, err = r.ReadByte(); err != nil {
					return ioError(err)
				}
			}
			continue
		}
		return r.UnreadByte()
	}
}

// isSpace reports whether c separates tokens.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v'
}

// ioError turns the end of the solver's output into errEOF, which the caller
// reports as a solver that stopped rather than as an unreadable reply.
func ioError(err error) error {
	if errors.Is(err, io.EOF) {
		return errEOF
	}
	return err
}
