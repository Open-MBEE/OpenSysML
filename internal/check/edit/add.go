package edit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const (
	kindIndividualDef       = "individual def"
	kindAssertNot           = "assert not"
	kindAssertConstraint    = "assert constraint"
	kindAssertNotConstraint = "assert not constraint"
	kindExhibitState        = "exhibit state"
	kindEntryAction         = "entry action"
	kindDoAction            = "do action"
	kindExitAction          = "exit action"
)

type memberKind struct {
	languages  map[source.Kind]bool
	definition bool
	typed      bool
	body       bool
}

// memberKinds are the kinds an OpAddMember writes by name alone: not connectors
// (ConnectionKinds) nor connector definitions, which need ends; `fork f;` is neither typed nor a definition.
// A kind only some bodies offer (`subject`, `actor`) is refused for any other owner.
var memberKinds = map[string]memberKind{
	"package":               {languages: bothLangs},
	"ref":                   {languages: sysmlOnly, typed: true},
	"return":                {languages: sysmlOnly, typed: true},
	"part def":              {languages: sysmlOnly, definition: true},
	"part":                  {languages: sysmlOnly, typed: true},
	"attribute def":         {languages: sysmlOnly, definition: true},
	"attribute":             {languages: sysmlOnly, typed: true},
	"item def":              {languages: sysmlOnly, definition: true},
	"item":                  {languages: sysmlOnly, typed: true},
	"port def":              {languages: sysmlOnly, definition: true},
	"port":                  {languages: sysmlOnly, typed: true},
	"enum def":              {languages: sysmlOnly, definition: true},
	"enum":                  {languages: sysmlOnly, typed: true},
	kindIndividualDef:       {languages: sysmlOnly, definition: true},
	"individual":            {languages: sysmlOnly, typed: true},
	"metadata def":          {languages: sysmlOnly, definition: true},
	"metadata":              {languages: sysmlOnly, typed: true},
	"view def":              {languages: sysmlOnly, definition: true},
	"view":                  {languages: sysmlOnly, typed: true},
	"viewpoint def":         {languages: sysmlOnly, definition: true},
	"viewpoint":             {languages: sysmlOnly, typed: true},
	"rendering def":         {languages: sysmlOnly, definition: true},
	"rendering":             {languages: sysmlOnly, typed: true},
	"concern def":           {languages: sysmlOnly, definition: true},
	"concern":               {languages: sysmlOnly, typed: true},
	"calc def":              {languages: sysmlOnly, definition: true, body: true},
	"calc":                  {languages: sysmlOnly, typed: true, body: true},
	"action def":            {languages: sysmlOnly, definition: true},
	"action":                {languages: sysmlOnly, typed: true},
	"perform action":        {languages: sysmlOnly, typed: true},
	"perform":               {languages: sysmlOnly},
	"assert":                {languages: sysmlOnly},
	kindAssertNot:           {languages: sysmlOnly},
	kindAssertConstraint:    {languages: sysmlOnly, typed: true, body: true},
	kindAssertNotConstraint: {languages: sysmlOnly, typed: true, body: true},
	kindExhibitState:        {languages: sysmlOnly, typed: true},
	"exhibit":               {languages: sysmlOnly},
	kindEntryAction:         {languages: sysmlOnly, typed: true},
	kindDoAction:            {languages: sysmlOnly, typed: true},
	kindExitAction:          {languages: sysmlOnly, typed: true},
	"state def":             {languages: sysmlOnly, definition: true},
	"state":                 {languages: sysmlOnly, typed: true},
	"occurrence def":        {languages: sysmlOnly, definition: true},
	"occurrence":            {languages: sysmlOnly, typed: true},
	"allocation def":        {languages: sysmlOnly, definition: true},
	"binding def":           {languages: sysmlOnly, definition: true},
	"constraint def":        {languages: sysmlOnly, definition: true, body: true},
	"constraint":            {languages: sysmlOnly, typed: true, body: true},
	"requirement def":       {languages: sysmlOnly, definition: true},
	"requirement":           {languages: sysmlOnly, typed: true},
	"case def":              {languages: sysmlOnly, definition: true, body: true},
	"case":                  {languages: sysmlOnly, typed: true, body: true},
	"analysis def":          {languages: sysmlOnly, definition: true, body: true},
	"analysis":              {languages: sysmlOnly, typed: true, body: true},
	"verification def":      {languages: sysmlOnly, definition: true, body: true},
	"verification":          {languages: sysmlOnly, typed: true, body: true},
	"use case def":          {languages: sysmlOnly, definition: true, body: true},
	"use case":              {languages: sysmlOnly, typed: true, body: true},
	"subject":               {languages: sysmlOnly, typed: true},
	"actor":                 {languages: sysmlOnly, typed: true},
	"stakeholder":           {languages: sysmlOnly, typed: true},
	"objective":             {languages: sysmlOnly, typed: true},
	"fork":                  {languages: sysmlOnly},
	"join":                  {languages: sysmlOnly},
	"merge":                 {languages: sysmlOnly},
	"decide":                {languages: sysmlOnly},
	"class":                 {languages: kermlOnly, definition: true},
	"struct":                {languages: kermlOnly, definition: true},
	"datatype":              {languages: kermlOnly, definition: true},
	"classifier":            {languages: kermlOnly, definition: true},
	"feature":               {languages: kermlOnly, typed: true},
	"step":                  {languages: kermlOnly, typed: true},
	"expr":                  {languages: kermlOnly, typed: true},
	"bool":                  {languages: kermlOnly, typed: true},
	"behavior":              {languages: kermlOnly, definition: true},
	"function":              {languages: kermlOnly, definition: true},
	"predicate":             {languages: kermlOnly, definition: true},
	"metaclass":             {languages: kermlOnly, definition: true},
}

// MemberKinds lists the member kinds legal in a language, sorted.
func MemberKinds(lang source.Kind) []string {
	return legalKinds(lang, func(name string) map[source.Kind]bool { return memberKinds[name].languages },
		mapKeys(memberKinds))
}

// MemberKindTyped reports whether an OpAddMember of kind may carry a Type.
func MemberKindTyped(kind string) bool {
	return memberKinds[kind].typed
}

// MemberKindOwnerBound reports whether only some bodies offer a member of kind:
// `subject` belongs in a requirement or case, `part` anywhere.
func MemberKindOwnerBound(kind string) bool {
	return parser.MemberOwner(kind) != ""
}

// MemberKindAdmittedBy reports whether the body of owner, a declaration an
// OpAddMember may name, offers a member of kind.
func MemberKindAdmittedBy(owner ast.Node, kind string) bool {
	return parser.BodyAdmitsMember(owner, kind)
}

func legalKinds(lang source.Kind, languages func(string) map[source.Kind]bool, names []string) []string {
	out := []string{}
	for _, name := range names {
		if languages(name)[lang] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

type addMemberDetails struct {
	splice          splice
	owner           ast.Node
	ownerPath       []string
	insertion       insertion
	memberText      string
	indent          string
	takenName       string
	introducedNames []string
}

func (m Model) addMemberSplice(i int, op Operation) (splice, error) {
	details, err := m.addMemberSpliceDetails(i, op)
	return details.splice, err
}

// addMemberSpliceDetails resolves an insertion and its segment-analysis metadata.
func (m Model) addMemberSpliceDetails(i int, op Operation) (addMemberDetails, error) {
	kind, err := m.addMemberKind(i, op)
	if err != nil {
		return addMemberDetails{}, err
	}
	referenceName, assertReference, err := checkMemberName(i, op)
	if err != nil {
		return addMemberDetails{}, err
	}
	if err := m.checkMemberValue(i, op, kind); err != nil {
		return addMemberDetails{}, err
	}
	if err := checkMemberPrefixes(i, op, kind); err != nil {
		return addMemberDetails{}, err
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return addMemberDetails{}, e
	}
	if err := checkMemberOwner(i, op, owner); err != nil {
		return addMemberDetails{}, err
	}
	ownerPath := []string(nil)
	if ownerScope != nil && ownerScope.Owner() != nil {
		ownerPath = symbols.NameChain(ownerScope.Owner())
	}
	takenName := op.MemberName
	if assertReference {
		takenName = ""
	} else if referenceName != "" {
		takenName = referenceName
	}
	if takenName != "" && nameTaken(ownerScope, takenName) {
		return addMemberDetails{}, &Error{
			Failure:        FailureMemberNameTaken,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s already declares %q", op.Owner, takenName),
		}
	}
	indent := m.ownerMemberIndent(owner)
	unit := m.memberIndentUnit(owner)
	docIndent := indent + unit
	doc := ""
	if op.Doc != "" {
		doc, err = documentationText(i, "", "", op.Doc, docIndent)
		if err != nil {
			return addMemberDetails{}, err
		}
	}
	memberText := writeMember(op, kind, indent, unit, doc, docIndent)
	ins := m.memberInsertion(owner, memberText)
	sp := splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}
	introduced := []string{}
	if takenName != "" {
		introduced = append(introduced, symbolName(takenName))
	}
	for _, target := range op.Redefines {
		if names, ok := source.QualifiedNameSegments(target); ok && len(names) > 0 {
			introduced = append(introduced, names[len(names)-1])
		}
	}
	return addMemberDetails{
		splice: sp, owner: owner, ownerPath: ownerPath, insertion: ins, memberText: memberText,
		indent: indent, takenName: takenName, introducedNames: introduced,
	}, nil
}

func illegalKind(i int, message string) *Error {
	return &Error{Failure: FailureIllegalKind, OperationIndex: i, Message: message}
}

// addMemberKind is the member kind the operation writes, checked legal in the
// source's language with the prefixes it carries.
func (m Model) addMemberKind(i int, op Operation) (memberKind, error) {
	kind, ok := memberKinds[op.MemberKind]
	if op.MemberKind == "" {
		// An empty kind writes a directed usage with no kind keyword (`in x : T;`);
		// it is a typed usage like `ref`, and SysML alone writes it.
		kind, ok = memberKinds["ref"], true
	}
	if !ok || !kind.languages[m.Source.Kind()] {
		return memberKind{}, illegalKind(i, fmt.Sprintf("kind %q is not legal in %s source %q",
			op.MemberKind, m.Source.Kind(), m.Source.Name()))
	}
	if op.MemberKind == "" && op.Direction == "" {
		return memberKind{}, illegalKind(i, "an empty member kind needs a direction (in, out or inout)")
	}
	if op.MemberKind == "" && op.IsAbstract {
		return memberKind{}, illegalKind(i, "an implicit directed usage cannot be abstract: `in abstract x` does not parse")
	}
	if op.MemberKind == "return" && len(op.MetadataPrefixes) > 0 {
		return memberKind{}, illegalKind(i, "return parameters cannot carry prefix metadata")
	}
	for _, prefix := range op.MetadataPrefixes {
		if err := checkQualifiedReference(i, "metadata prefix", prefix); err != nil {
			return memberKind{}, err
		}
	}
	return kind, nil
}

// checkMemberName checks the member's name as its kind reads it: the asserted
// constraint's reference, the performed action's or exhibited state's reference
// (whose last segment is the name taken), an identifier, or none.
func checkMemberName(i int, op Operation) (referenceName string, assertReference bool, err error) {
	assertReference = op.MemberKind == "assert" || op.MemberKind == kindAssertNot
	switch {
	case assertReference:
		if _, err := checkFeatureReference(i, "asserted constraint", op.MemberName); err != nil {
			return "", true, err
		}
		if op.Value != "" {
			return "", true, illegalKind(i, fmt.Sprintf("kind %q cannot carry a value", op.MemberKind))
		}
	case op.MemberName == "":
		return "", false, checkUnnamedMember(i, op)
	case op.MemberKind == "perform" || op.MemberKind == "exhibit":
		role := "performed action"
		if op.MemberKind == "exhibit" {
			role = "exhibited state"
		}
		last, err := checkFeatureReference(i, role, op.MemberName)
		if err != nil {
			return "", false, err
		}
		referenceName = last
	default:
		if err := checkName(i, op.MemberName); err != nil {
			e := err.(*Error)
			e.Failure = FailureInvalidName
			e.Message = fmt.Sprintf("member name %q is not an identifier", op.MemberName)
			return "", false, e
		}
	}
	return referenceName, assertReference, nil
}

// checkUnnamedMember admits an empty member name for a return parameter with
// a type or multiplicity, an objective, a constraint kind with a type or body
// expression, or a member with redefines targets.
func checkUnnamedMember(i int, op Operation) error {
	constraintKind := op.MemberKind == "constraint" || op.MemberKind == kindAssertConstraint || op.MemberKind == kindAssertNotConstraint
	bodied := constraintKind && (op.Type != "" || op.BodyExpression != "")
	switch {
	case op.MemberKind == "return" && op.Type == "" && op.Multiplicity == "":
		return illegalKind(i, "an unnamed return parameter needs a type or multiplicity")
	case op.MemberKind == "objective", bodied:
		return nil
	case op.MemberKind != "return" && len(op.Redefines) == 0:
		return &Error{
			Failure: FailureInvalidName, OperationIndex: i,
			Message: "an empty member name requires redefines targets, kind return or objective, or a constraint kind with a type or body expression",
		}
	}
	return nil
}

// checkMemberValue checks the value and body expression the member states.
func (m Model) checkMemberValue(i int, op Operation, kind memberKind) error {
	if op.MemberKind == "metadata" && op.Value != "" {
		return illegalKind(i, "kind \"metadata\" cannot carry a value")
	}
	if op.Value != "" {
		valueOp := op
		valueOp.Target = op.MemberName
		if err := m.checkValue(i, valueOp); err != nil {
			return err
		}
	}
	if op.BodyExpression == "" {
		return nil
	}
	if !kind.body {
		message := fmt.Sprintf("kind %q cannot state a body expression", op.MemberKind)
		if refusal, ok := noResultBodyKinds[op.MemberKind]; ok {
			message = fmt.Sprintf("kind %q cannot state a body expression: %s", op.MemberKind, refusal)
		}
		return illegalKind(i, message)
	}
	target := op.MemberName
	if target == "" {
		target = op.Owner
	}
	return m.checkExpression(i, "body expression", target, op.BodyExpression)
}

// checkMemberPrefixes checks the prefixes, typing, multiplicity and
// relationship targets the member carries against what its kind admits.
func checkMemberPrefixes(i int, op Operation, kind memberKind) error {
	prefixExcluded := kind.definition || !kind.typed || memberPrefixExcluded(op.MemberKind)
	switch {
	case op.IsDefault && prefixExcluded:
		return illegalKind(i, fmt.Sprintf("kind %q cannot carry a default value", op.MemberKind))
	case op.IsDefault && op.Value == "":
		return &Error{Failure: FailureInvalidValue, OperationIndex: i, Message: "default requires a nonempty value expression"}
	case op.Direction != "" && op.Direction != "in" && op.Direction != "out" && op.Direction != "inout":
		return &Error{Failure: FailureInvalidValue, OperationIndex: i, Message: fmt.Sprintf("direction %q is not in, out or inout", op.Direction)}
	case !kind.typed && op.Type != "":
		return illegalKind(i, fmt.Sprintf("kind %q cannot carry a typing target", op.MemberKind))
	case !kind.typed && !kind.definition && (op.Multiplicity != "" || op.Value != ""):
		return illegalKind(i, fmt.Sprintf("kind %q takes a name alone", op.MemberKind))
	case kind.definition && op.Value != "":
		return illegalKind(i, fmt.Sprintf("definition kind %q cannot carry a value", op.MemberKind))
	case !kind.definition && len(op.Specializes) > 0:
		return illegalKind(i, fmt.Sprintf("kind %q is a usage and cannot carry specializes targets", op.MemberKind))
	case op.IsAbstract && (op.MemberKind == "enum def" || (!kind.definition && (!kind.typed || memberPrefixExcluded(op.MemberKind)))):
		return illegalKind(i, fmt.Sprintf("kind %q cannot be abstract", op.MemberKind))
	case op.Direction != "" && prefixExcluded:
		return illegalKind(i, fmt.Sprintf("kind %q cannot carry a direction", op.MemberKind))
	case len(op.Redefines) > 0 && kind.definition:
		return illegalKind(i, fmt.Sprintf("definition kind %q cannot carry redefines targets; use specializes", op.MemberKind))
	case len(op.Redefines) > 0 && noRedefinesKinds[op.MemberKind]:
		return illegalKind(i, fmt.Sprintf("kind %q cannot carry redefines targets", op.MemberKind))
	case op.MemberKind == "return" && (op.IsAbstract || op.Direction != "" || len(op.Redefines) > 0):
		return illegalKind(i, "return parameters cannot be abstract, directional or redefining")
	}
	for _, target := range op.Redefines {
		if err := checkEnd(i, "redefines", target); err != nil {
			return err
		}
	}
	return nil
}

var noRedefinesKinds = map[string]bool{
	"metadata": true, "perform": true, "exhibit": true, "assert": true, kindAssertNot: true,
}

// checkMemberOwner checks that the owner's body admits the member's kind.
func checkMemberOwner(i int, op Operation, owner ast.Node) error {
	// An empty kind is admitted wherever an explicit directed usage is: the
	// direction it writes is the member notation a plain `ref` spells anyway.
	admitKind := op.MemberKind
	if admitKind == "" {
		admitKind = "ref"
	}
	if !parser.BodyAdmitsMember(owner, admitKind) {
		return illegalKind(i, fmt.Sprintf("kind %q is only declared in a %s body, which %s does not open",
			admitKind, parser.MemberOwner(admitKind), ownerName(op.Owner)))
	}
	switch op.MemberKind {
	case "assert", kindAssertNot, kindAssertConstraint, kindAssertNotConstraint, kindExhibitState, "exhibit":
		if !parser.BodyAdmitsBehaviorUsage(owner) {
			return illegalKind(i, fmt.Sprintf("%s is not admitted in the body of %s", op.MemberKind, ownerName(op.Owner)))
		}
	case kindEntryAction, kindDoAction, kindExitAction:
		if !stateBodyOwner(owner) {
			return illegalKind(i, fmt.Sprintf("%s is only admitted in a state body, which %s does not open",
				op.MemberKind, ownerName(op.Owner)))
		}
		if hasStateSubaction(owner, memberSubactionKind(op.MemberKind)) {
			return illegalKind(i, fmt.Sprintf("%s already has a %s action", ownerName(op.Owner), memberSubactionKind(op.MemberKind)))
		}
	case "return":
		if !parser.BodyIsCalculation(owner) {
			return illegalKind(i, "return parameters are only admitted in calculation, constraint and case bodies")
		}
		if hasResultParameter(owner) {
			return illegalKind(i, "a calculation, constraint or case body already has a return parameter")
		}
	}
	return nil
}

func hasResultParameter(owner ast.Node) bool {
	for _, member := range ast.DeclMembers(owner) {
		if membership, ok := member.(*ast.Membership); ok && membership != nil {
			member = membership.Member
		}
		if usage, ok := member.(*ast.Usage); ok && usage.IsResult {
			return true
		}
	}
	return false
}

func memberPrefixExcluded(kind string) bool {
	switch kind {
	case "package", "subject", "actor", "stakeholder", "objective",
		"fork", "join", "merge", "decide", "metadata", "return",
		"perform", "perform action", "assert", kindAssertNot, kindAssertConstraint, kindAssertNotConstraint,
		kindExhibitState, "exhibit", kindEntryAction, kindDoAction, kindExitAction:
		return true
	default:
		return false
	}
}

const noResultRequirementBody = "a requirement body has no result expression; add a require constraint instead"

var noResultBodyKinds = map[string]string{
	"requirement def": noResultRequirementBody,
	"requirement":     noResultRequirementBody,
	"concern def":     noResultRequirementBody,
	"concern":         noResultRequirementBody,
	"viewpoint def":   noResultRequirementBody,
	"viewpoint":       noResultRequirementBody,
	"objective":       noResultRequirementBody,
}

// nameTaken reports whether scope declares a member named name. A `first x`
// label borrows its name from the member it starts at and so takes none; a
// body may sequence `first g;` before it declares g.
func nameTaken(scope *symbols.Scope, name string) bool {
	if scope == nil {
		return false
	}
	for _, sym := range scope.LookupLocalAll(symbolName(name)) {
		if !isStartLabel(sym.Decl) {
			return true
		}
	}
	return false
}

// isStartLabel reports whether decl is the `first x` of a body, which the index
// registers under x as a label: it borrows the name of the member it starts at
// and declares no node of its own, so it neither takes the name nor makes it
// visible.
func isStartLabel(decl ast.Node) bool {
	_, label := decl.(*ast.InitialNode)
	return label
}

func (m Model) addOwner(fqn string) (ast.Node, *symbols.Scope, error) {
	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if fqn == "" {
		return m.Root, rootScope, nil
	}
	var local *symbols.Symbol
	for _, sym := range m.declared(fqn) {
		if sym.DocName == m.Source.Name() {
			if local != nil {
				return nil, nil, &Error{Failure: FailureAmbiguousTarget,
					Message: fmt.Sprintf("%q names several declarations", fqn)}
			}
			local = sym
		}
	}
	if local == nil {
		return nil, nil, &Error{Failure: FailureOwnerUnknown,
			Message: fmt.Sprintf("no namespace named %q in this model", fqn)}
	}
	if local.Scope == nil {
		return nil, nil, &Error{Failure: FailureOwnerNotNamespace,
			Message: fmt.Sprintf("%q cannot contain members", fqn)}
	}
	switch local.Decl.(type) {
	case *ast.Package, *ast.Namespace, *ast.Definition, *ast.Usage, *ast.SubstateMember:
		return local.Decl, local.Scope, nil
	default:
		return nil, nil, &Error{Failure: FailureOwnerNotNamespace,
			Message: fmt.Sprintf("%q cannot contain members", fqn)}
	}
}

// ownerName names an add-member owner for a message: the document for "".
func ownerName(fqn string) string {
	if fqn == "" {
		return "the document"
	}
	return fmt.Sprintf("%q", fqn)
}

func writeMember(op Operation, kind memberKind, indent, unit, doc, docIndent string) string {
	prefix := make([]string, 0, 3)
	if op.Direction != "" {
		prefix = append(prefix, op.Direction)
	}
	if op.IsAbstract {
		prefix = append(prefix, "abstract")
	}
	metadata := make([]string, len(op.MetadataPrefixes))
	for i, name := range op.MetadataPrefixes {
		metadata[i] = "#" + name
	}
	extensionKeyword := op.MemberKind == "ref" || op.MemberKind == "individual" ||
		op.MemberKind == kindIndividualDef || op.MemberKind == "subject" ||
		op.MemberKind == "actor" || op.MemberKind == "stakeholder" ||
		op.MemberKind == "objective" || op.MemberKind == kindEntryAction ||
		op.MemberKind == kindDoAction || op.MemberKind == kindExitAction
	if !extensionKeyword {
		prefix = append(prefix, metadata...)
	}
	switch op.MemberKind {
	case "":
		// An empty kind writes no keyword: the direction itself spells the
		// usage (`in x : T;`).
	case "return":
		prefix = append(prefix, "return")
	case "ref":
		prefix = append(prefix, "ref")
		prefix = append(prefix, metadata...)
	case "individual", "subject", "actor", "stakeholder", "objective":
		prefix = append(prefix, op.MemberKind)
		prefix = append(prefix, metadata...)
	case kindIndividualDef:
		prefix = append(prefix, "individual")
		prefix = append(prefix, metadata...)
		prefix = append(prefix, "def")
	case kindEntryAction, kindDoAction, kindExitAction:
		keyword, _, _ := strings.Cut(op.MemberKind, " ")
		prefix = append(prefix, keyword)
		prefix = append(prefix, metadata...)
		prefix = append(prefix, "action")
	default:
		prefix = append(prefix, op.MemberKind)
	}
	header := strings.Join(prefix, " ")
	if op.MemberName != "" {
		header += " " + op.MemberName
	}
	if kind.definition && len(op.Specializes) > 0 {
		header += " specializes " + strings.Join(op.Specializes, ", ")
	} else if !kind.definition && op.Type != "" {
		header += " : " + op.Type
	}
	if len(op.Redefines) > 0 {
		header += " :>> " + strings.Join(op.Redefines, ", ")
	}
	if op.Multiplicity != "" {
		header += " " + op.Multiplicity
	}
	if op.Value != "" {
		if op.IsDefault {
			header += " default = " + op.Value
		} else {
			header += " = " + op.Value
		}
	}
	if op.BodyExpression != "" {
		return writeBodyExpression(header, op.BodyExpression, indent, unit, doc, docIndent)
	}
	if doc != "" {
		return header + " {\n" + docIndent + doc + "\n" + indent + "}"
	}
	return header + ";"
}

func writeBodyExpression(header, expression, indent, unit, doc, docIndent string) string {
	lines := strings.Split(strings.ReplaceAll(expression, "\r\n", "\n"), "\n")
	if len(lines) == 1 && doc == "" {
		return header + " { " + strings.TrimSpace(lines[0]) + " }"
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}
	common := ""
	hasCommon := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if !hasCommon {
			common = leading
			hasCommon = true
			continue
		}
		for !strings.HasPrefix(leading, common) && common != "" {
			common = common[:len(common)-1]
		}
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = strings.TrimPrefix(line, common)
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var text strings.Builder
	text.WriteString(header)
	text.WriteString(" {\n")
	if doc != "" {
		text.WriteString(docIndent)
		text.WriteString(doc)
		text.WriteByte('\n')
	}
	for i, line := range lines {
		if line != "" {
			text.WriteString(indent)
			text.WriteString(unit)
			text.WriteString(line)
		}
		if i+1 < len(lines) {
			text.WriteByte('\n')
		}
	}
	text.WriteByte('\n')
	text.WriteString(indent)
	text.WriteByte('}')
	return text.String()
}

func (m Model) memberIndentUnit(owner ast.Node) string {
	ownerIndent := lineIndent(m.Source.Bytes(), owner.Span().Offset)
	memberIndent := m.memberIndent(owner.Span())
	if strings.HasPrefix(memberIndent, ownerIndent) && len(memberIndent) > len(ownerIndent) {
		return memberIndent[len(ownerIndent):]
	}
	return m.memberIndentStyle()
}

// insertion is the splice adding a member to an owner: text replaces span, and
// the member's own notation starts at offset at within text.
type insertion struct {
	span source.Span
	text string
	at   int
}

// ownerMemberIndent is the indentation a member of owner is written at.
func (m Model) ownerMemberIndent(owner ast.Node) string {
	if owner == m.Root {
		return ""
	}
	return m.memberIndent(owner.Span())
}

// memberInsertion places text, one member's notation with its later lines
// already indented for owner, where a new member of owner goes: before the
// closing brace of its body, or in the body opened for a bodyless owner.
func (m Model) memberInsertion(owner ast.Node, text string) insertion {
	if owner == m.Root {
		prefix := ""
		if len(m.Source.Bytes()) > 0 && m.Source.Bytes()[len(m.Source.Bytes())-1] != '\n' {
			prefix = "\n"
		}
		return insertion{span: source.Span{Offset: m.Source.Len()}, text: prefix + text + "\n", at: len(prefix)}
	}
	body, hasBody := bodyInfo(owner)
	ownerIndent := lineIndent(m.Source.Bytes(), owner.Span().Offset)
	indent := m.ownerMemberIndent(owner)
	if hasBody {
		if parser.BodyIsCalculation(owner) {
			members := ast.DeclMembers(owner)
			if len(members) > 0 && isCalculationResultMember(members[len(members)-1]) {
				return m.memberInsertionBeforeResult(members[len(members)-1], text)
			}
		}
		rbrace := m.lastToken(body, lexer.RBrace)
		closeOffset := rbrace.Span.Offset
		lineStart := closeOffset
		for lineStart > 0 && m.Source.Bytes()[lineStart-1] != '\n' {
			lineStart--
		}
		closeIndent := string(m.Source.Bytes()[lineStart:closeOffset])
		if closeIndent != "" && !onlyWhitespace([]byte(closeIndent)) {
			lineStart = closeOffset
			for lineStart > 0 && (m.Source.Bytes()[lineStart-1] == ' ' || m.Source.Bytes()[lineStart-1] == '\t') {
				lineStart--
			}
			closeIndent = ownerIndent
		}
		prefix := "\n"
		if lineStart > 0 && m.Source.Bytes()[lineStart-1] == '\n' {
			prefix = ""
		}
		return insertion{
			span: source.Span{Offset: lineStart, Len: closeOffset - lineStart},
			text: prefix + indent + text + "\n" + closeIndent,
			at:   len(prefix) + len(indent),
		}
	}
	semi := m.lastToken(owner.Span(), lexer.Semicolon)
	open := " {\n" + indent
	return insertion{
		span: source.Span{Offset: semi.Span.Offset, Len: semi.Span.Len},
		text: open + text + "\n" + ownerIndent + "}",
		at:   len(open),
	}
}

func isCalculationResultMember(member ast.Node) bool {
	if ast.IsExpression(member) {
		return true
	}
	condition, ok := member.(*ast.ConstraintMember)
	return ok && condition.Keyword == "" && condition.Expression != nil
}

func (m Model) memberInsertionBeforeResult(result ast.Node, text string) insertion {
	return m.memberInsertionBefore(result.Span().Offset, text)
}

// memberInsertionBefore places text as a member ahead of the one starting at
// offset: on its own line above that member's leading comments when the member
// opens its line, else inline before it.
func (m Model) memberInsertionBefore(offset int, text string) insertion {
	content := m.Source.Bytes()
	lineStart := offset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	leading := content[lineStart:offset]
	if onlyWhitespace(leading) {
		anchor := precedingFullLineTriviaStart(content, lineStart)
		return insertion{
			span: source.Span{Offset: anchor},
			text: string(leading) + text + "\n",
			at:   len(leading),
		}
	}
	prefix := ""
	if offset == 0 || (content[offset-1] != ' ' && content[offset-1] != '\t' && content[offset-1] != '\n') {
		prefix = " "
	}
	return insertion{
		span: source.Span{Offset: offset},
		text: prefix + text + " ",
		at:   len(prefix),
	}
}

func precedingFullLineTriviaStart(content []byte, lineStart int) int {
	s := triviaScan{anchor: lineStart, blockEndAnchor: lineStart}
	for s.anchor > 0 {
		lineEnd := s.anchor - 1
		previousLine := lineEnd
		for previousLine > 0 && content[previousLine-1] != '\n' {
			previousLine--
		}
		line := strings.TrimSpace(string(content[previousLine:lineEnd]))
		var more bool
		if s.inBlockComment {
			more = s.insideBlock(line, previousLine)
		} else {
			more = s.outsideBlock(line, previousLine)
		}
		if !more {
			break
		}
	}
	return s.anchor
}

// triviaScan walks upward over the full-line comments and blank lines above a
// member; anchor is the start of the trivia found so far.
type triviaScan struct {
	anchor         int
	inBlockComment bool
	blockEndAnchor int
}

// insideBlock reads a line while inside a block comment read from its end,
// reporting whether the scan continues above it.
func (s *triviaScan) insideBlock(line string, previousLine int) bool {
	if opening := strings.LastIndex(line, "/*"); opening >= 0 {
		if strings.TrimSpace(line[:opening]) != "" {
			s.anchor = s.blockEndAnchor
			return false
		}
		s.inBlockComment = false
	} else if closing := strings.LastIndex(line, "*/"); closing >= 0 {
		after := strings.TrimSpace(line[closing+2:])
		if after != "" && !strings.HasPrefix(after, "//") {
			s.anchor = s.blockEndAnchor
			return false
		}
	}
	s.anchor = previousLine
	return true
}

// outsideBlock reads a line of code, comment or blank, reporting whether the
// scan continues above it.
func (s *triviaScan) outsideBlock(line string, previousLine int) bool {
	if line == "" || strings.HasPrefix(line, "//") {
		s.anchor = previousLine
		return true
	}
	if opening := strings.Index(line, "/*"); opening >= 0 {
		if strings.TrimSpace(line[:opening]) != "" {
			return false
		}
		closing := strings.Index(line[opening+2:], "*/")
		if closing < 0 {
			s.enterBlock(previousLine)
			return true
		}
		after := strings.TrimSpace(line[opening+closing+4:])
		if after == "" || strings.HasPrefix(after, "//") {
			s.anchor = previousLine
			return true
		}
		return false
	}
	if closing := strings.LastIndex(line, "*/"); closing >= 0 && strings.TrimSpace(line[closing+2:]) == "" {
		s.enterBlock(previousLine)
		return true
	}
	return false
}

func (s *triviaScan) enterBlock(previousLine int) {
	s.inBlockComment = true
	s.blockEndAnchor = s.anchor
	s.anchor = previousLine
}

func bodyInfo(node ast.Node) (source.Span, bool) {
	switch d := node.(type) {
	case *ast.Package:
		return d.Span(), d.HasBody
	case *ast.Namespace:
		return d.Span(), d.HasBody
	case *ast.Definition:
		return d.Span(), d.HasBody
	case *ast.Usage:
		return d.Span(), d.HasBody
	case *ast.SubstateMember:
		return d.Span(), false
	case *ast.TransitionMember:
		return d.Span(), d.HasBody
	case *ast.SuccessionEdge:
		return d.Span(), d.HasBody
	case *ast.Dependency:
		return d.Span(), d.HasBody
	case *ast.MultiplicityDecl:
		return d.Span(), d.HasBody
	case *ast.RelationshipMember:
		return d.Span(), d.HasBody
	default:
		return source.Span{}, false
	}
}

func lineIndent(content []byte, offset int) string {
	start := offset
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	i := start
	for i < offset && (content[i] == ' ' || content[i] == '\t') {
		i++
	}
	return string(content[start:i])
}

func (m Model) memberIndent(owner source.Span) string {
	content := m.Source.Bytes()
	base := lineIndent(content, owner.Offset)
	indent := ""
	start := owner.Offset
	for start < owner.End() {
		end := start
		for end < owner.End() && content[end] != '\n' {
			end++
		}
		i := start
		for i < end && (content[i] == ' ' || content[i] == '\t') {
			i++
		}
		if i < end && i > start {
			prefix := string(content[start:i])
			if len(prefix) > len(base) {
				indent = prefix
				break
			}
		}
		if end == owner.End() {
			break
		}
		start = end + 1
	}
	if indent == "" {
		indent = base + m.memberIndentStyle()
	}
	return indent
}

func (m Model) memberIndentStyle() string {
	if m.tokenData().hasTabs {
		return "\t"
	}
	return "    "
}
