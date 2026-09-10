package repl

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// completionLimit bounds a completion answer: the library registers tens of
// thousands of names, and a terminal cannot usefully offer all of them.
const completionLimit = 200

// Completion answers a completion request: the words that can replace the one
// being typed, and the part of it already typed, which every candidate starts
// with.
type Completion struct {
	Candidates []string
	Prefix     string
}

// Complete offers the words that can continue line at byte offset pos: the meta
// commands where a command is being typed, file paths where %load and %save
// take one, and otherwise the names the session and the library declare.
func (s *Session) Complete(line string, pos int) Completion {
	// Reads beside a running command: readline asks from its input goroutine
	// while the loop may be evaluating the previous line.
	defer s.reading()()

	if pos < 0 || pos > len(line) {
		pos = len(line)
	}
	head := line[:pos]
	command := firstToken(head)

	// A "%" word is a command only while it is still the only word: after it,
	// the line is arguments.
	if word := lastField(head); strings.HasPrefix(word, "%") && word == command {
		return completion(word, matchingPrefix(metaCommands(), word))
	}
	if command == "%load" || command == "%save" {
		word := lastField(head)
		return completion(word, pathCompletions(word))
	}
	// %render takes the form after the view name, which is no name to look up.
	if command == "%render" && atSecondArgument(head) {
		word := lastField(head)
		return completion(word, matchingPrefix(renderForms(), word))
	}
	if atObjectArgument(head) {
		word := objectWord(head)
		return completion(word, s.objectCompletions(word))
	}
	word := nameWord(head)
	return completion(word, s.nameCompletions(word))
}

// completion assembles an answer in name order, bounding how many candidates
// are offered.
func completion(word string, candidates []string) Completion {
	out := append([]string(nil), candidates...)
	sort.Strings(out)
	if len(out) > completionLimit {
		// The prompt inserts what the candidates share, so dropping matches
		// must not lengthen it: where it would, only what every match shares
		// is offered, and nothing where that is what is already typed.
		shared := sharedPrefix(out)
		out = out[:completionLimit]
		if len(sharedPrefix(out)) > len(shared) {
			if shared == word {
				return Completion{Prefix: word}
			}
			out = []string{shared}
		}
	}
	return Completion{Candidates: out, Prefix: word}
}

// sharedPrefix returns the longest prefix every candidate begins with.
func sharedPrefix(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	prefix := candidates[0]
	for _, c := range candidates[1:] {
		for !strings.HasPrefix(c, prefix) {
			// Trimmed a character at a time: a prefix cut mid-character is
			// inserted at the prompt as a replacement character.
			_, size := utf8.DecodeLastRuneInString(prefix)
			prefix = prefix[:len(prefix)-size]
		}
	}
	return prefix
}

// atSecondArgument reports whether the word being typed is the second argument
// of the command, the first one having been typed and followed by a space. The
// arguments are counted the way dispatch splits them, so a quoted name holding a
// space ('My View') is one argument, and one still being typed is not yet past.
func atSecondArgument(head string) bool {
	return !inUnfinishedName(head) && argumentIndex(head) == 2
}

// atObjectArgument reports whether the word being typed is an argument the
// command takes an object in: the object %features, %invoke and %state inspect,
// the performer of %action and %state, the destination of %send after `to`, and
// the context of a pinned %eval. A quoted name still being typed is part of that
// argument.
func atObjectArgument(head string) bool {
	switch firstToken(head) {
	case "%features", "%invoke":
		return argumentIndex(head) == 1
	case "%state":
		at := argumentIndex(head)
		return at == 1 || at == 2
	case "%action":
		return argumentIndex(head) == 2
	case "%send":
		at, args := argumentIndex(head), typedArgs(head)
		return at >= 2 && at-1 < len(args) && args[at-1] == "to"
	case "%eval":
		tail := strings.TrimLeft(strings.TrimPrefix(strings.TrimLeft(head, " \t"), "%eval"), " \t")
		rest, pinned := cutWord(tail, "in")
		return pinned && contextSeparator(rest) < 0
	}
	return false
}

// argumentIndex is the position of the word being typed among the command's
// arguments, the command itself being 0. Counted the way dispatch splits them,
// with a quoted name still being typed closed so it counts as one argument.
func argumentIndex(head string) int {
	args := typedArgs(head)
	if !inUnfinishedName(head) && (strings.HasSuffix(head, " ") || strings.HasSuffix(head, "\t")) {
		return len(args)
	}
	return len(args) - 1
}

// typedArgs splits head the way dispatch splits a line, closing a quoted name
// still being typed so it counts as one argument.
func typedArgs(head string) []string {
	if inUnfinishedName(head) {
		return parseArgs(head + "'")
	}
	return parseArgs(head)
}

// inUnfinishedName reports whether head ends inside an unrestricted name whose
// closing quote has not been typed. The notation's own escape is honoured, so
// 'it\'s' is a finished name.
func inUnfinishedName(head string) bool {
	inName, escaped := false, false
	for _, r := range head {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '\'':
			inName = !inName
		}
	}
	return inName
}

// firstToken returns the first whitespace-separated token of a line.
func firstToken(line string) string {
	if fields := strings.Fields(line); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// lastField returns the text after the last whitespace, which is the word a
// path is being typed into.
func lastField(head string) string {
	if cut := strings.LastIndexAny(head, " \t"); cut >= 0 {
		return head[cut+1:]
	}
	return head
}

// nameWord returns the name being typed at the end of head: the trailing run of
// characters a qualified SysML name is written with, a quoted name among them
// taken whole, closing quote typed or not.
func nameWord(head string) string {
	return trailingWord(head, "_:'")
}

// objectWord returns the object reference being typed at the end of head: the
// text after the last whitespace outside a quoted name, since a name may hold
// a space and a reference is one argument.
func objectWord(head string) string {
	start, inName, escaped := 0, false, false
	for i, r := range head {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '\'':
			inName = !inName
		case !inName && (r == ' ' || r == '\t'):
			start = i + 1
		}
	}
	return head[start:]
}

// trailingWord returns the trailing run of letters, digits and the punctuation
// in extra at the end of head, whatever a quoted name holds included.
func trailingWord(head, extra string) string {
	start, inName, escaped := 0, false, false
	for i, r := range head {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '\'':
			inName = !inName
		case inName, strings.ContainsRune(extra, r), unicode.IsLetter(r), unicode.IsDigit(r):
		default:
			start = i + utf8.RuneLen(r)
		}
	}
	return head[start:]
}

// matchingPrefix returns the candidates that start with prefix.
func matchingPrefix(candidates []string, prefix string) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// splitPath splits a typed path after its last separator, accepting either
// separator so a path is split the way the platform reads it.
func splitPath(word string) (dir, base string) {
	for i := len(word) - 1; i >= 0; i-- {
		if os.IsPathSeparator(word[i]) {
			return word[:i+1], word[i+1:]
		}
	}
	return "", word
}

// pathCompletions returns the files and directories the typed path can name.
// Candidates extend what was typed, so a directory keeps its trailing separator
// and a "~" is left as written.
func pathCompletions(word string) []string {
	dir, base := splitPath(word)
	read := dir
	if read == "" {
		read = "."
	}
	entries, err := os.ReadDir(expandHome(read))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		if e.IsDir() {
			// The separator already typed, so a candidate reads back the way
			// the path was written.
			sep := byte(os.PathSeparator)
			if dir != "" {
				sep = dir[len(dir)-1]
			}
			name += string(sep)
		}
		out = append(out, dir+name)
	}
	return out
}

// objectCompletions returns the object references that can continue word: the
// ids of the objects the session holds, the declared names, and after a `.` or
// `::` the features of the object typed so far that hold objects, an element of
// a multi-valued one picked by index.
func (s *Session) objectCompletions(word string) []string {
	if root, sep, partial, walked := lastSegment(word); walked {
		if shape, ok := s.peekObject(root); ok {
			return s.featureCompletions(shape, root+sep, partial)
		}
		if strings.HasPrefix(word, "#") || sep == "." {
			return nil
		}
	}
	if strings.HasPrefix(word, "#") {
		return matchingPrefix(s.objectIDs(), word)
	}
	out := s.nameCompletions(word)
	if word == "" {
		out = append(out, s.objectIDs()...)
	}
	return out
}

// lastSegment splits an object reference before the segment being typed: the
// reference walked so far, the separator after it, and the partial segment,
// which may be a quoted name not yet closed. A reference with no separator yet
// is not walked.
func lastSegment(word string) (root, sep, partial string, walked bool) {
	inName, escaped := false, false
	at, width := -1, 0
	for i := 0; i < len(word); i++ {
		switch {
		case escaped:
			escaped = false
		case word[i] == '\\':
			escaped = true
		case word[i] == '\'':
			inName = !inName
		case inName:
		case word[i] == '.':
			at, width = i, 1
		case strings.HasPrefix(word[i:], "::"):
			at, width = i, 2
			i++
		}
	}
	if at < 0 {
		return "", "", word, false
	}
	return word[:at], word[at : at+width], word[at+width:], true
}

// objectIDs lists the objects the session holds as `#<id>` references.
func (s *Session) objectIDs() []string {
	if s.rtCtx == nil {
		return nil
	}
	ids := s.heldIDs()
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, fmt.Sprintf("#%d", id))
	}
	return out
}

// objectShape is what completion knows of the object a reference reaches:
// the object once it exists, until then only its type.
type objectShape struct {
	inst *runtime.Instance
	typ  *symbols.Symbol
}

// peekObject follows an object reference through what the session already
// holds, materializing nothing: an unmaterialized part is followed by type.
func (s *Session) peekObject(text string) (objectShape, bool) {
	if s.rtCtx == nil {
		return objectShape{}, false
	}
	ref, err := parseObjectRef(text)
	if err != nil {
		return objectShape{}, false
	}
	var (
		root *runtime.Instance
		rest []objectSegment
		ok   bool
	)
	if ref.id > 0 {
		// A connector a carry-over set aside is offered by id but not built to
		// complete a path from.
		root, _ = s.heldByID(ref.id)
		ok = root != nil
		rest = ref.segments
	} else {
		var rerr error
		root, _, rest, rerr = s.namedRoot(ref)
		ok = rerr == nil
	}
	if !ok {
		return objectShape{}, false
	}
	shape := objectShape{inst: root, typ: root.Type}
	for _, seg := range rest {
		feat := featureNamed(s.rtCtx.FeaturesOf(shape.typ), seg.name)
		if feat == nil || feat.Scalar() != (seg.index == 0) {
			return objectShape{}, false
		}
		if fv := heldFeatureValue(shape.inst, seg.name); fv != nil {
			val := fv.Value
			if seg.index > 0 {
				elements := collectionElements(fv.Values)
				if seg.index > len(elements) {
					return objectShape{}, false
				}
				val = elements[seg.index-1]
			}
			id, isObject := val.Object()
			if !isObject {
				return objectShape{}, false
			}
			child, has := s.rtCtx.Instance(id)
			if !has {
				return objectShape{}, false
			}
			shape = objectShape{inst: child, typ: child.Type}
			continue
		}
		typ := s.objectTypeOf(feat)
		if typ == nil || max(seg.index, 1) > s.elementCount(shape, feat) {
			return objectShape{}, false
		}
		shape = objectShape{typ: typ}
	}
	return shape, true
}

// holdsObjects reports whether a feature is a path segment of the object holding
// it: once materialized, whether it holds an object (a selected variation's
// object as much as a part's); before that, whether reading it would.
func (s *Session) holdsObjects(shape objectShape, feat *runtime.EffectiveFeature) bool {
	fv := heldFeatureValue(shape.inst, feat.Name)
	if fv == nil {
		return s.elementCount(shape, feat) > 0
	}
	if fv.Values.Kind == runtime.ValInvalid {
		_, isObject := fv.Value.Object()
		return isObject
	}
	for _, el := range collectionElements(fv.Values) {
		if _, isObject := el.Object(); isObject {
			return true
		}
	}
	return false
}

// objectTypeOf is the type of the object a feature holds once read, as the
// runtime materializes it (a part, port or structured attribute from its type, a
// connector from its own usage), or nil for a feature a value binds and for a
// variation, whose object is of whichever variant is selected.
func (s *Session) objectTypeOf(feat *runtime.EffectiveFeature) *symbols.Symbol {
	if feat.Name == "" {
		return nil
	}
	if typ := s.rtCtx.CompositeTypeOf(feat); typ != nil {
		return typ
	}
	if s.rtCtx.Model().IsConnectorUsage(feat.Symbol) {
		return feat.Symbol
	}
	return nil
}

// featureNamed is the effective feature called name, or nil.
func featureNamed(features []runtime.EffectiveFeature, name string) *runtime.EffectiveFeature {
	for i := range features {
		if features[i].Name == name {
			return &features[i]
		}
	}
	return nil
}

// heldFeatureValue is the value inst already holds for name, nil until
// materialized or while there is no object to hold it.
func heldFeatureValue(inst *runtime.Instance, name string) *runtime.FeatureValue {
	if inst == nil {
		return nil
	}
	if fv, ok := inst.FeatureValues[name]; ok && fv.Materialized {
		return fv
	}
	return nil
}

// featureCompletions offers the object-holding features of shape, written after
// prefix, that continue partial. A multi-valued feature is offered as indexed
// elements, since a reference has to pick one.
func (s *Session) featureCompletions(shape objectShape, prefix, partial string) []string {
	if s.rtCtx == nil {
		return nil
	}
	var candidates []string
	features := s.rtCtx.FeaturesOf(shape.typ)
	for i := range features {
		feat := &features[i]
		if feat.Name == "" || !s.holdsObjects(shape, feat) {
			continue
		}
		name := prefix + lexer.NameText(feat.Name)
		if feat.Scalar() {
			candidates = append(candidates, name)
			continue
		}
		for n := 1; n <= s.elementCount(shape, feat); n++ {
			candidates = append(candidates, fmt.Sprintf("%s[%d]", name, n))
		}
	}
	var out []string
	for _, c := range matchingPrefix(candidates, prefix+partial) {
		if c != prefix+partial {
			out = append(out, c)
		}
	}
	return out
}

// elementCount is how many elements completion may index: the ones held once
// materialized, before that the ones reading the feature would hold.
func (s *Session) elementCount(shape objectShape, feat *runtime.EffectiveFeature) int {
	return min(s.elementsToHold(shape, feat, map[string]bool{}), elementLimit)
}

// elementsToHold is how many objects feat holds, or would once read, as the
// runtime materializes it: what the features subsetting it contribute first,
// then — unless it is abstract or optional, which hold only contributions —
// anonymous objects up to its lower bound. reading guards a cycle of subsetting
// features, which reading reports instead.
func (s *Session) elementsToHold(shape objectShape, feat *runtime.EffectiveFeature, reading map[string]bool) int {
	if fv := heldFeatureValue(shape.inst, feat.Name); fv != nil {
		if fv.Values.Kind == runtime.ValInvalid {
			if _, isObject := fv.Value.Object(); isObject {
				return 1
			}
			return 0
		}
		return len(collectionElements(fv.Values))
	}
	if reading[feat.Name] || s.objectTypeOf(feat) == nil {
		return 0
	}
	if feat.Scalar() && !feat.HoldsOnlyContributions() {
		return 1
	}
	lower := feat.Multiplicity.Lower
	if !lower.Known || lower.Infinite {
		return 0
	}
	reading[feat.Name] = true
	defer delete(reading, feat.Name)
	contributed := 0
	for _, sub := range s.rtCtx.SubsettingFeatures(shape.inst, shape.typ, feat.Name) {
		contributed += s.elementsToHold(shape, &sub, reading)
	}
	if feat.HoldsOnlyContributions() {
		// A scalar holding more than one contribution holds none: reading it is refused.
		if feat.Scalar() && contributed > 1 {
			return 0
		}
		return contributed
	}
	return max(contributed, int(lower.Value))
}

// elementLimit bounds the indexed elements one feature is completed to.
const elementLimit = 16

// nameCompletions returns the names that can continue word: the session's own
// declarations, the next segment of a qualified library name, and the library
// functions callable by their unqualified name. Names are offered as the
// notation writes them, a name that needs quoting in quotes.
func (s *Session) nameCompletions(word string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		// A candidate equal to the typed word inserts nothing and empties the
		// prefix readline shares between the real candidates.
		if name == "" || name == word || seen[name] || !strings.HasPrefix(name, word) {
			return
		}
		seen[name] = true
		out = append(out, name)
	}

	for _, name := range s.declaredSymbolNames() {
		add(lexer.NameText(name))
	}
	for _, b := range runtime.Builtins() {
		add(b.Name)
	}
	if idx := s.browseIndex(); idx != nil {
		for _, fqn := range idx.FQNs() {
			text := s.declaredName(fqn)
			if !strings.HasPrefix(text, word) {
				continue
			}
			// One segment at a time: completing "ISQ" to every name under it
			// would answer with the whole library. A word already spelling a
			// namespace is answered with that namespace's members.
			from := len(word)
			if strings.HasPrefix(text[from:], "::") {
				from += len("::")
			}
			if cut := nextQualifier(text, from); cut >= 0 {
				add(text[:cut])
				continue
			}
			add(text)
		}
	}
	return out
}

// nextQualifier is the index of the first `::` at or after from that is not
// inside a quoted name, or -1.
func nextQualifier(text string, from int) int {
	inName, escaped := false, false
	for i := 0; i < len(text); i++ {
		switch {
		case escaped:
			escaped = false
		case text[i] == '\\':
			escaped = true
		case text[i] == '\'':
			inName = !inName
		case !inName && i >= from && strings.HasPrefix(text[i:], "::"):
			return i
		}
	}
	return -1
}
