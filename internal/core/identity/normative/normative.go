// Package normative derives the name-based UUIDs (RFC 4122 v5) KerML fixes for
// the named elements of the standard library, as the SysML v2 pilot does:
//
//	package    = uuid5(NAMESPACE_URL, prefix + escaped(package name))
//	descendant = uuid5(package, escaped qualified name)
//	membership = uuid5(package, escaped qualified name + "/owningMembership")
package normative

import (
	"crypto/sha1" // #nosec G505 -- RFC 4122 defines a version 5 UUID over SHA-1; it names, it does not protect
	"encoding/hex"
	"strings"
)

// Language is the half of the standard library an element belongs to, which
// picks the prefix its library package's id is minted under.
type Language uint8

const (
	// KerML is the Kernel libraries: Kernel Semantic, Data Type and Function.
	KerML Language = iota + 1
	// SysML is the Systems and Domain libraries.
	SysML
)

// Prefix is the specification URL the language's library package ids hash under.
func (l Language) Prefix() string {
	switch l {
	case KerML:
		return "https://www.omg.org/spec/KerML/"
	case SysML:
		return "https://www.omg.org/spec/SysML/"
	}
	return ""
}

func (l Language) String() string {
	switch l {
	case KerML:
		return "KerML"
	case SysML:
		return "SysML"
	}
	return "unknown"
}

// NamespaceURL is the RFC 4122 name space for URLs, the root of every library id.
var NamespaceURL = mustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")

// Separator joins the segments of a qualified name.
const Separator = "::"

// owningMembershipPath is the path suffix an owning membership's id hashes.
const owningMembershipPath = "/owningMembership"

// ElementID is the normative id of the library element with the given qualified
// name (segments as declared, the first its library package); "" for none.
func ElementID(lang Language, qualifiedName string) string {
	pkg, path, top, ok := split(lang, qualifiedName)
	if !ok {
		return ""
	}
	if top {
		return pkg.String()
	}
	return uuid5(pkg, path).String()
}

// OwningMembershipID is the normative id of the OwningMembership owning the
// library element with the given qualified name, a library package included.
func OwningMembershipID(lang Language, qualifiedName string) string {
	pkg, path, _, ok := split(lang, qualifiedName)
	if !ok {
		return ""
	}
	return uuid5(pkg, path+owningMembershipPath).String()
}

// split derives the library package's id and the escaped qualified name hashed
// under it; top reports that the name is the package's own.
func split(lang Language, qualifiedName string) (pkg uuid, path string, top, ok bool) {
	if lang.Prefix() == "" || qualifiedName == "" {
		return uuid{}, "", false, false
	}
	segments := strings.Split(qualifiedName, Separator)
	for i, s := range segments {
		segments[i] = EscapeName(s)
	}
	pkg = uuid5(NamespaceURL, lang.Prefix()+segments[0])
	return pkg, strings.Join(segments, Separator), len(segments) == 1, true
}

// EscapeName spells a name as the pilot's qualified names do: an ASCII identifier
// or `$` as is, anything else single-quoted with control and quote characters escaped.
func EscapeName(name string) string {
	if name == "" || name == "$" || isIdentifier(name) {
		return name
	}
	var b strings.Builder
	b.Grow(len(name) + 2)
	b.WriteByte('\'')
	for _, r := range name {
		if i := strings.IndexRune("\b\t\n\f\r\"'\\", r); i >= 0 {
			b.WriteByte('\\')
			b.WriteByte("btnfr\"'\\"[i])
			continue
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

// isIdentifier matches [a-zA-Z_][a-zA-Z0-9_]*, ASCII only as the pilot reads it.
func isIdentifier(name string) bool {
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return name != ""
}

// uuid is an RFC 4122 UUID in its binary form.
type uuid [16]byte

// uuid5 is the version 5 name-based UUID of name in the given name space.
func uuid5(space uuid, name string) uuid {
	h := sha1.New() // #nosec G401 -- the hash the UUID version prescribes
	h.Write(space[:])
	h.Write([]byte(name))
	var u uuid
	copy(u[:], h.Sum(nil))
	u[6] = u[6]&0x0f | 0x50
	u[8] = u[8]&0x3f | 0x80
	return u
}

// String spells the UUID in its canonical lowercase 8-4-4-4-12 form.
func (u uuid) String() string {
	var b [36]byte
	hex.Encode(b[0:8], u[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], u[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], u[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], u[8:10])
	b[23] = '-'
	hex.Encode(b[24:36], u[10:16])
	return string(b[:])
}

// mustParse reads a canonical UUID; it is only applied to constants.
func mustParse(s string) uuid {
	var u uuid
	raw := strings.ReplaceAll(s, "-", "")
	if len(raw) != 32 {
		panic("normative: malformed uuid " + s)
	}
	if _, err := hex.Decode(u[:], []byte(raw)); err != nil {
		panic("normative: malformed uuid " + s)
	}
	return u
}
