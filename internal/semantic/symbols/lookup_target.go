package symbols

import (
	"math/bits"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// targetLookup is one reading of what the imports left to name lookup may bring.
type targetLookup struct {
	// leaving memoizes leavesToLookup.
	leaving map[string]bool
	// pending holds, with its place in frames, what is being resolved: it brings
	// nothing to its own resolution (KerML 8.2.3.5.4).
	pending map[namedIn]int
	frames  []int
	serial  int
	// passed marks the pending frames, by depth, the current one passed over.
	passed uint64
	// targets keeps what lookupTargets answered, so a lattice of named imports is
	// read once per name.
	targets map[namedIn]lookedUp
	// spare holds the emptied sets of finished walks, for the next ones.
	spare []map[string]bool
}

// unseen returns an empty set for a lookupWalk to mark the namespaces it saw in.
func (look *targetLookup) unseen() map[string]bool {
	if n := len(look.spare); n != 0 {
		seen := look.spare[n-1]
		look.spare = look.spare[:n-1]
		return seen
	}
	return map[string]bool{}
}

// recycle takes back the set of a walk that is over.
func (look *targetLookup) recycle(seen map[string]bool) {
	clear(seen)
	look.spare = append(look.spare, seen)
}

// lookedUp is one answer of lookupTargets: it holds while the innermost frame it
// passed over, if any, is still pending.
type lookedUp struct {
	targets []string
	known   bool
	passed  uint64
	frame   int
}

// holds reports whether the answer still holds in look.
func (kept lookedUp) holds(look *targetLookup) bool {
	below := bits.Len64(kept.passed)
	return below == 0 || (below <= len(look.frames) && look.frames[below-1] == kept.frame)
}

// How enter found what it was asked to make pending.
const (
	entered = iota
	alreadyPending
	tooDeep
)

// lookupDepth bounds the frames of one reading: passed marks one bit for each.
const lookupDepth = 64

// enter makes key pending. Already pending, the caller passes over it; leave reports
// the outer frames passed over meanwhile, with the innermost of them.
func (look *targetLookup) enter(key namedIn) (leave func() (passed uint64, frame int), found int) {
	if look.passes(key) {
		return nil, alreadyPending
	}
	if len(look.frames) == lookupDepth {
		return nil, tooDeep
	}
	if look.pending == nil {
		look.pending, look.targets = map[namedIn]int{}, map[namedIn]lookedUp{}
	}
	look.serial++
	look.frames = append(look.frames, look.serial)
	depth, outer := len(look.frames), look.passed
	look.pending[key], look.passed = depth, 0
	return func() (passed uint64, frame int) {
		delete(look.pending, key)
		look.frames = look.frames[:depth-1]
		passed = look.passed & (1<<(depth-1) - 1)
		look.passed = outer | passed
		if passed != 0 {
			frame = look.frames[bits.Len64(passed)-1]
		}
		return passed, frame
	}, entered
}

// passes reports whether key is pending, passing over it.
func (look *targetLookup) passes(key namedIn) bool {
	at, pending := look.pending[key]
	if pending {
		look.passed |= 1 << (at - 1)
	}
	return pending
}

// The statements a targetLookup reads a qualified name of.
const (
	ofWildcardImport = ""
	ofNamedImport    = "import "
	ofAlias          = "alias "
	ofMember         = "member "
)

// leftBrings reports whether a wildcard import the namespace under fqn leaves to
// name lookup may bring a member called name: it may, or the index cannot tell.
func (idx *Index) leftBrings(fqn, name string, look *targetLookup) bool {
	if !idx.leavesToLookup(fqn, look) {
		return false
	}
	for _, imp := range idx.imports(fqn) {
		if !idx.leftBy(fqn, imp.target, look) || !idx.surfacesBeside(fqn, imp.target, name) {
			continue
		}
		targets, known := idx.lookupTargets(fqn, imp.target, ofWildcardImport, false, nil, look)
		if !known {
			return true
		}
		for _, target := range targets {
			if members, known := idx.membersByLookup(target, name, false, nil, look); !known || len(members) != 0 {
				return true
			}
		}
	}
	return false
}

// boundBeside resolves name, the first segment of a wildcard import's target, where an
// import the namespace under fqn leaves to name lookup may bring it: to the one
// namespace name lookup reads there, or "" when the index cannot tell which.
func (idx *Index) boundBeside(fqn, name string, look *targetLookup) string {
	members, known := idx.membersByLookup(fqn, name, true, nil, look)
	if !known || len(members) != 1 {
		return ""
	}
	return members[0]
}

// surfacesBeside reports whether name lookup, resolving another import's target, reads
// name through the import of target stated under fqn (the resolver's importPrefixAvailable).
//
// It does when the target's first segment is name itself, is declared on the way out
// or names a top-level element; not when only an import binds it, or an alias beside it.
func (idx *Index) surfacesBeside(fqn, target, name string) bool {
	first, _, _ := strings.Cut(target, "::")
	if first == name {
		return true
	}
	for prefix := fqn; prefix != ""; prefix = prefix[:max(lastSeparator(prefix), 0)] {
		key := prefix + "::" + first
		idx.readName(key)
		reexported := idx.reexported.at(key)
		for _, sym := range idx.fqn.at(key) {
			if !reexported.has(sym) {
				return sym.Kind != SymbolAlias || prefix != fqn
			}
		}
	}
	idx.readName(first)
	return len(idx.fqn.at(first)) != 0
}

// lookupTargets lists every namespace path may name from the namespace under fqn, as
// name lookup reads it through named imports and aliases; not known when the index cannot tell.
func (idx *Index) lookupTargets(fqn, path, of string, global bool, pending *ast.Import, look *targetLookup) (targets []string, known bool) {
	reading := namedIn{fqn, of + path}
	if kept, ok := look.targets[reading]; ok && kept.holds(look) {
		look.passed |= kept.passed
		return kept.targets, kept.known
	}
	leave, found := look.enter(reading)
	if found != entered {
		return nil, false
	}
	defer func() {
		passed, frame := leave()
		look.targets[reading] = lookedUp{targets, known, passed, frame}
	}()

	// A path qualified from the root (`$::`) is sought there alone.
	if global {
		fqn = ""
	}
	name, rest, more := strings.Cut(path, "::")
	for prefix := fqn; ; prefix = prefix[:max(lastSeparator(prefix), 0)] {
		members, known := idx.membersByLookup(prefix, name, true, pending, look)
		if !known {
			return nil, false
		}
		targets = union(targets, members)
		// Members that hide each other bind nothing, and the name is sought further out.
		if len(members) == 1 || prefix == "" {
			break
		}
	}
	for more {
		name, rest, more = strings.Cut(rest, "::")
		var below []string
		for _, target := range targets {
			members, known := idx.membersByLookup(target, name, false, nil, look)
			if !known {
				return nil, false
			}
			below = union(below, members)
		}
		targets = below
	}
	return targets, true
}

// membersByLookup lists what name may name in the namespace under fqn, seen from inside
// it or through an import of it: several when they collide. Not known when the index cannot tell.
func (idx *Index) membersByLookup(fqn, name string, inside bool, pending *ast.Import, look *targetLookup) (members []string, known bool) {
	key := name
	if fqn != "" {
		key = fqn + "::" + name
	}
	idx.readName(fqn)
	idx.readName(key)
	if idx.declaresUnder(key) {
		if !inside && !idx.namedFromOutside(key) {
			return nil, true
		}
		member, ok := idx.wildcardTargetAt(key)
		return []string{member}, ok
	}
	if members, known = idx.boundByLookup(fqn, key, name, inside, pending, look); !known || len(members) != 0 {
		return members, known
	}

	// Past its own named imports and aliases, what its wildcard imports bring: the
	// members of the targets the index resolved, then of the ones left to name lookup.
	leave, found := look.enter(namedIn{fqn, ofMember + name})
	if found != entered {
		return nil, found == alreadyPending
	}
	defer leave()
	for _, imp := range idx.lastTargets.at(fqn) {
		if imp.fqn == "" || (!inside && imp.private) {
			continue
		}
		brought, known := idx.membersByLookup(imp.fqn, name, false, nil, look)
		if !known {
			return nil, false
		}
		members = union(members, brought)
	}
	for _, imp := range idx.imports(fqn) {
		if !inside && imp.private {
			continue
		}
		if look.passes(namedIn{fqn, ofWildcardImport + imp.target}) || !idx.surfacesBeside(fqn, imp.target, name) {
			continue
		}
		if !idx.leftBy(fqn, imp.target, look) {
			continue
		}
		targets, known := idx.lookupTargets(fqn, imp.target, ofWildcardImport, false, nil, look)
		if !known {
			return nil, false
		}
		for _, target := range targets {
			brought, known := idx.membersByLookup(target, name, false, nil, look)
			if !known {
				return nil, false
			}
			members = union(members, brought)
		}
	}
	return members, true
}

// boundByLookup lists what the named imports and aliases the namespace under fqn states
// bind name to; not known when one of them does not resolve.
func (idx *Index) boundByLookup(fqn, key, name string, inside bool, pending *ast.Import, look *targetLookup) (bound []string, known bool) {
	reexported := idx.reexported.at(fqn)
	for _, sym := range idx.fqn.at(fqn) {
		if sym.Scope == nil || reexported.has(sym) {
			continue
		}
		for _, imp := range sym.Scope.Imports() {
			if imp == pending || imp.Kind != ast.ImportMembership || imp.Imported == nil || len(imp.Imported.Parts) == 0 ||
				imp.Imported.Parts[len(imp.Imported.Parts)-1].Text != name || (!inside && !VisibleOutside(imp.Visibility)) {
				continue
			}
			if look.passes(namedIn{fqn, ofNamedImport + imp.Imported.Text()}) {
				continue
			}
			targets, known := idx.lookupTargets(fqn, imp.Imported.Text(), ofNamedImport, imp.Imported.Global, imp, look)
			if !known {
				return nil, false
			}
			bound = union(bound, targets)
		}
	}
	reexported = idx.reexported.at(key)
	for _, sym := range idx.fqn.at(key) {
		if sym.Kind != SymbolAlias || reexported.has(sym) || (!inside && !VisibleOutside(sym.Visibility)) {
			continue
		}
		if sym.Recorded() {
			if sym.Facts.Alias.IsZero() {
				return nil, false
			}
			bound = union(bound, []string{sym.Facts.Alias.FQN})
			continue
		}
		al, ok := sym.Decl.(*ast.Alias)
		if !ok || al.For == nil {
			return nil, false
		}
		targets, known := idx.lookupTargets(fqn, al.For.Text(), ofAlias, al.For.Global, nil, look)
		if !known {
			return nil, false
		}
		bound = union(bound, targets)
	}
	return bound, true
}

// union appends to names the ones of more it does not hold.
func union(names, more []string) []string {
	for _, name := range more {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}
