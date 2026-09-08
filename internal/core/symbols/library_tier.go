package symbols

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// LibraryTier is the part of the bundled library a document belongs to. The
// Kernel tiers are the metamodel frame every element specializes; the Systems,
// Domain and OpenSysML tiers declare features with semantics of their own.
type LibraryTier uint8

const (
	// TierNone is the workspace's own content.
	TierNone LibraryTier = iota
	// TierLibrary is library content whose place in the bundle is not stated.
	TierLibrary
	// TierKernelSemantic is the Kernel Semantic Library (Base, Occurrences, Objects, …).
	TierKernelSemantic
	// TierKernelDataType is the Kernel Data Type Library (ScalarValues, Collections, …).
	TierKernelDataType
	// TierKernelFunction is the Kernel Function Library.
	TierKernelFunction
	// TierSystems is the Systems Library (Items, Parts, Requirements, …).
	TierSystems
	// TierDomain is the Domain Libraries (Geometry, Quantities and Units, Metadata, …).
	TierDomain
	// TierOpenSysML is this implementation's own library packages.
	TierOpenSysML
	numLibraryTiers
)

// tierNames spell the tiers for diagnostics and tests.
var tierNames = [numLibraryTiers]string{
	TierNone:           "model",
	TierLibrary:        "library",
	TierKernelSemantic: "Kernel Semantic Library",
	TierKernelDataType: "Kernel Data Type Library",
	TierKernelFunction: "Kernel Function Library",
	TierSystems:        "Systems Library",
	TierDomain:         "Domain Libraries",
	TierOpenSysML:      "OpenSysML Libraries",
}

func (t LibraryTier) String() string {
	if t < numLibraryTiers {
		return tierNames[t]
	}
	return "unknown library tier"
}

// Library reports whether the tier is bundled library content of any kind.
func (t LibraryTier) Library() bool {
	return t != TierNone
}

// Frame reports whether the tier frames every element rather than describing
// the objects a model asks for: the Kernel libraries, and library content of no
// stated tier, which is held to the same standard.
func (t LibraryTier) Frame() bool {
	switch t {
	case TierSystems, TierDomain, TierOpenSysML:
		return false
	default:
		return t.Library()
	}
}

// Semantic reports whether the tier states the language's own semantics — the
// Kernel and Systems libraries, which an implementation realizes rather than
// executes as written — or library content of no stated tier, held to the same
// standard. A Domain or OpenSysML library is a model, executed as one.
func (t LibraryTier) Semantic() bool {
	switch t {
	case TierDomain, TierOpenSysML:
		return false
	default:
		return t.Library()
	}
}

// LibraryDocument describes the bundled library content a document holds: the
// tier of its bundle and a digest of its text, empty when the loader stated none.
type LibraryDocument struct {
	Tier   LibraryTier
	Digest string
}

// TextDigest is the digest of a library document's text that LibraryDocument
// carries.
func TextDigest(text []byte) string {
	sum := sha256.Sum256(text)
	return hex.EncodeToString(sum[:])
}

// libraryIdentityOf digests the library documents by name, tier and text, and
// reports false when one states no text digest, since it may then hold anything.
func libraryIdentityOf(docs map[string]LibraryDocument) (string, bool) {
	names := make([]string, 0, len(docs))
	for name, doc := range docs {
		if doc.Digest == "" {
			return "", false
		}
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		fmt.Fprintf(h, "%s\x00%d\x00%s\x00", name, docs[name].Tier, docs[name].Digest)
	}
	return hex.EncodeToString(h.Sum(nil)), true
}
