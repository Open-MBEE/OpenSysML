package parser

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// definitionKindKeywords maps a single kind keyword to its DefinitionKind.
// The two-word `use case` is handled separately in parseDefUsage.
var definitionKindKeywords = map[string]ast.DefinitionKind{
	"part":      ast.DefPart,
	"attribute": ast.DefAttribute,
	"datatype":  ast.DefAttribute,
	"feature":   ast.DefAttribute,
	// Tier A.
	"item":       ast.DefItem,
	"occurrence": ast.DefOccurrence,
	"individual": ast.DefIndividual,
	"metaclass":  ast.DefMetaclass,
	"metadata":   ast.DefMetadata,
	"enum":       ast.DefEnumeration,
	"view":       ast.DefView,
	"viewpoint":  ast.DefViewpoint,
	"rendering":  ast.DefRendering,
	"concern":    ast.DefConcern,
	// Tier B.
	"connection": ast.DefConnection,
	"flow":       ast.DefFlow,
	"message":    ast.DefFlow, // message is synonym for flow
	"port":       ast.DefPort,
	"interface":  ast.DefInterface,
	"allocation": ast.DefAllocation,
	"binding":    ast.DefBinding,
	// Tier C.
	"action":       ast.DefAction,
	"state":        ast.DefState,
	"calc":         ast.DefCalc,
	"function":     ast.DefCalc, // synonym for calc
	"constraint":   ast.DefConstraint,
	"requirement":  ast.DefRequirement,
	"case":         ast.DefCase,
	"analysis":     ast.DefAnalysisCase,
	"verification": ast.DefVerificationCase,
	// KerML structural.
	"behavior":      ast.DefBehavior,
	"assoc":         ast.DefAssoc,
	"struct":        ast.DefStruct,
	"class":         ast.DefClass,
	"classifier":    ast.DefClass, // synonym for class
	"subclassifier": ast.DefClass, // subtyping context synonym for classifier
	"predicate":     ast.DefPredicate,
	"bool":          ast.DefBool,
}

// compoundDefKinds are the two-keyword kinds, where the second keyword names
// the kind rather than the declaration. Every other pair of kind keywords is a
// kind followed by a name. The two-word `use case` is handled separately in
// parseDefUsage.
var compoundDefKinds = map[[2]string]bool{
	{"assoc", "struct"}:      true,
	{"analysis", "case"}:     true,
	{"verification", "case"}: true,
}

// kindPrefixKeywords are keywords that qualify a following kind keyword rather
// than naming the declaration, even when no name follows the kind:
// `assert constraint { ... }` is an asserted anonymous constraint, not a
// declaration named `constraint`.
var kindPrefixKeywords = map[string]bool{
	"assert":  true,
	"assume":  true,
	"require": true,
	"var":     true,
}

// notKindPrefixKeywords are keywords that are never a prefix of a following
// kind keyword, because they are a kind of their own with dedicated parsing
// (`satisfy requirement r by x`) or a direction that the kind carries
// (`in item x`). A following kind keyword belongs to their own declaration.
var notKindPrefixKeywords = map[string]bool{
	"subject": true, "objective": true, "succession": true, "inv": true,
	"connector": true, "satisfy": true, "verify": true, "step": true,
	"expr": true, "interaction": true, "stakeholder": true, "frame": true,
	"actor": true, "expose": true, "render": true, "perform": true,
	"include": true, "exhibit": true, "variant": true, "event": true,
	"timeslice": true, "snapshot": true, "transition": true, "bind": true,
	"binding": true, "member": true,
	// `not satisfy r by p` negates the satisfaction; the prefix path would drop
	// the `not` (SysML.xtext:2118, SatisfyRequirementUsage isNegated).
	"not": true,
	// `individual part p` keeps the modifier; the prefix path would drop it.
	"individual": true,
	"in":         true, "out": true, "inout": true,
	// An annotation ends with its comment body, so a kind keyword after
	// `doc /* … */` opens the next member rather than being qualified by it.
	"doc": true, "comment": true, "rep": true, "locale": true,
}

// usageKindKeywords maps a single kind keyword to its UsageKind.
var usageKindKeywords = map[string]ast.UsageKind{
	"part":      ast.UsagePart,
	"attribute": ast.UsageAttribute,
	"datatype":  ast.UsageAttribute,
	"feature":   ast.UsageAttribute,
	// Tier A.
	"item":       ast.UsageItem,
	"occurrence": ast.UsageOccurrence,
	"event":      ast.UsageOccurrence, // event creates occurrence usage (event-driven)
	"individual": ast.UsageIndividual,
	"snapshot":   ast.UsageOccurrence, // snapshot occurrence usage
	"timeslice":  ast.UsageOccurrence, // timeslice occurrence usage (temporal slice)
	"metadata":   ast.UsageMetadata,
	"enum":       ast.UsageEnumeration,
	"view":       ast.UsageView,
	"viewpoint":  ast.UsageViewpoint,
	"rendering":  ast.UsageRendering,
	"concern":    ast.UsageConcern,
	// Tier B.
	"connection":  ast.UsageConnection,
	"connector":   ast.UsageConnector,
	"succession":  ast.UsageSuccession,
	"flow":        ast.UsageFlow,
	"message":     ast.UsageFlow, // message is synonym for flow
	"port":        ast.UsagePort,
	"interface":   ast.UsageInterface,
	"interaction": ast.UsageInteraction,
	"allocation":  ast.UsageAllocation,
	// `allocate` introduces a ConnectorPart, never a definition
	// (AllocationUsageDeclaration, SysML.xtext:1219-1222).
	"allocate": ast.UsageAllocation,
	"binding":  ast.UsageBinding,
	"actor":    ast.UsageActor,         // actor of a requirement, use case or viewpoint
	"render":   ast.UsageViewRendering, // rendering a view body names
	"bind":     ast.UsageBinding,       // shorthand for binding
	// Tier C.
	"action":       ast.UsageAction,
	"perform":      ast.UsageAction, // perform keyword creates action usage
	"state":        ast.UsageState,
	"exhibit":      ast.UsageState, // exhibit references state usage (state exhibition)
	"transition":   ast.UsageTransition,
	"step":         ast.UsageStep,
	"calc":         ast.UsageCalc,
	"expr":         ast.UsageExpr, // expression parameter (lambda/closure)
	"function":     ast.UsageCalc, // synonym for calc
	"constraint":   ast.UsageConstraint,
	"inv":          ast.UsageConstraint, // synonym for constraint (invariant)
	"require":      ast.UsageConstraint, // synonym for constraint (required condition)
	"assert":       ast.UsageConstraint, // assert creates constraint usage (assertion)
	"assume":       ast.UsageConstraint, // assume creates constraint usage (assumption)
	"requirement":  ast.UsageRequirement,
	"satisfy":      ast.UsageSatisfy,
	"verify":       ast.UsageSatisfy, // verify is alias for satisfy
	"include":      ast.UsageUseCase, // include creates use case usage with includes relationship
	"subject":      ast.UsageSubject,
	"objective":    ast.UsageObjective,
	"stakeholder":  ast.UsageStakeholder,
	"frame":        ast.UsageFramedConcern,
	"case":         ast.UsageCase,
	"analysis":     ast.UsageAnalysisCase,
	"verification": ast.UsageVerificationCase,
	"variant":      ast.UsagePart, // variant keyword creates variant membership
	// KerML structural.
	"behavior":  ast.UsageBehavior,
	"assoc":     ast.UsageAssoc,
	"struct":    ast.UsageStruct,
	"class":     ast.UsageClass,
	"predicate": ast.UsagePredicate,
	"bool":      ast.UsageBool,
}

var featureModifierKeywords = map[string]bool{
	"abstract":   true,
	"variation":  true,
	"ref":        true,
	"end":        true,
	"constant":   true,
	"const":      true, // KerML spelling (KerML.xtext BasicFeaturePrefix)
	"event":      true, // event-driven occurrence modifier
	"individual": true, // individual occurrence/part modifier
	"snapshot":   true, // snapshot occurrence/part modifier
	"in":         true,
	"out":        true,
	"inout":      true,
	"composite":  true,
	"portion":    true,
	"derived":    true,
	"ordered":    true,
	"nonunique":  true,
	"public":     true,
	"protected":  true,
	"private":    true,
	"readonly":   true,
}

// relationshipKeywords maps a spelled-out relationship keyword to its kind.
var relationshipKeywords = map[string]ast.RelationshipKind{
	"specializes": ast.RelSpecializes,
	"subsets":     ast.RelSubsets,
	"redefines":   ast.RelRedefines,
	"references":  ast.RelReferences,
	"crosses":     ast.RelCrosses,
	"intersects":  ast.RelIntersects,
	"disjoint":    ast.RelDisjoint, // followed by 'from' keyword
	"unions":      ast.RelUnions,
	"differences": ast.RelDifferences,
	"chains":      ast.RelChains,
}

type featureMods struct {
	isAbstract    bool
	isVariation   bool
	isVariant     bool
	isReference   bool
	isVariable    bool
	isEnd         bool
	isChain       bool
	isConstant    bool
	isEvent       bool            // event modifier for occurrences
	isIndividual  bool            // individual modifier for individuals/snapshots
	portion       ast.PortionKind // 'snapshot' / 'timeslice' portion prefix
	isNegated     bool            // `not` of `assert not <kind>`: the conditions are asserted to be false
	prefixKeyword string          // keyword qualifying the kind that follows it: the `assert` of `assert constraint c`
	visibility    ast.Visibility
	direction     ast.FeatureDirection
	isComposite   bool
	isPortion     bool
	isDerived     bool
	isReadonly    bool
	isOrdered     bool
	isNonunique   bool
	cross         *ast.CrossFeatureMember // the cross feature declared right after `end`
	usageOnly     lexer.Token             // first prefix keyword only a usage prefix admits (`ref`, a direction, …)
}

// checkConstantSpelling rejects the constant prefix of the other notation:
// KerML spells it `const` (BasicFeaturePrefix), SysML `constant` (RefPrefix).
func (p *Parser) checkConstantSpelling(t lexer.Token) {
	kerml := p.src.Kind() == source.KindKerML
	switch {
	case kerml && t.KeywordID == "constant":
		p.error(t.Span, "`constant` is SysML notation: the KerML grammar spells the prefix `const`, "+
			"so write `const` here or move the declaration to a .sysml file")
	case !kerml && t.KeywordID == "const":
		p.error(t.Span, "`const` is KerML notation: the SysML grammar spells the prefix `constant`, "+
			"so write `constant` here or move the declaration to a .kerml file")
	}
}

// prefixConflict reports t written after a prefix keyword the grammar makes it an
// alternative of (SysML.xtext BasicDefinitionPrefix/RefPrefix, KerML.xtext BasicFeaturePrefix).
func (p *Parser) prefixConflict(t lexer.Token, prior, alternatives string) {
	p.error(t.Span, fmt.Sprintf("'%s' cannot follow '%s': a prefix says %s, not both", t.KeywordID, prior, alternatives))
}

// noteUsageOnly remembers the first prefix keyword no DefinitionPrefix admits,
// so a definition written after it can be rejected at that keyword.
func (m *featureMods) noteUsageOnly(t lexer.Token) {
	if m.usageOnly.Span.Len == 0 {
		m.usageOnly = t
	}
}

// abstractOrVariation names the prefix keyword of the pair already read.
func (m *featureMods) abstractOrVariation() string {
	if m.isVariation {
		return "variation"
	}
	return "abstract"
}

// directionOf maps a FeatureDirection keyword to its kind.
func directionOf(kw string) ast.FeatureDirection {
	switch kw {
	case "in":
		return ast.DirIn
	case "out":
		return ast.DirOut
	}
	return ast.DirInOut
}

// tryParseCrossFeature parses the cross feature an end declares ahead of its kind
// keyword, `end var x1 : Sub1 [0..1] :> g feature x` (KerML.xtext OwnedCrossingFeature),
// consuming nothing otherwise; a bare prefix (`end ref attribute e`) is the end's.
func (p *Parser) tryParseCrossFeature() *ast.CrossFeatureMember {
	cp := p.checkpoint()
	defer p.release()
	start := p.peek().Span.Offset
	cross := &ast.CrossFeatureMember{}
	p.parseCrossFeaturePrefix(cross)
	if p.atName() || p.at(lexer.Lt) {
		cross.Ident = p.parseIdentification()
	}
	cross.Relationships = p.parseRelationships(true)
	if p.parseCrossMultiplicityPart(cross) {
		cross.Relationships = append(cross.Relationships, p.parseRelationships(true)...)
	}
	declared := cross.Ident.Name != "" || cross.Ident.ShortName != "" ||
		len(cross.Relationships) > 0 || cross.Multiplicity != nil || cross.IsOrdered || cross.IsNonunique
	if !declared || !p.isKindKeyword(p.peek()) {
		p.restore(cp)
		// `end [2] nonunique ref e : C`: the multiplicity part alone is still the crossing one.
		if p.at(lexer.LBracket) {
			cross = &ast.CrossFeatureMember{}
			p.parseCrossMultiplicityPart(cross)
			cross.NodeSpan = source.Span{Offset: start, Len: p.lastEnd() - start}
			return cross
		}
		return nil
	}
	cross.NodeSpan = p.spanFrom(start)
	return cross
}

// parseCrossMultiplicityPart reads `[mult]` and the `ordered`/`nonunique` after it
// onto cross (KerML.xtext MultiplicityPart), reporting whether any was present.
func (p *Parser) parseCrossMultiplicityPart(cross *ast.CrossFeatureMember) bool {
	present := false
	if p.at(lexer.LBracket) {
		cross.Multiplicity = p.parseMultiplicity()
		present = true
	}
	for {
		switch {
		case p.atKeyword("ordered") && !cross.IsOrdered:
			cross.IsOrdered = true
		case p.atKeyword("nonunique") && !cross.IsNonunique:
			cross.IsNonunique = true
		default:
			return present
		}
		p.advance()
		present = true
	}
}

// parseCrossFeaturePrefix reads the modifiers a cross feature is declared with
// (KerML.xtext BasicFeaturePrefix, SysML.xtext BasicUsagePrefix) onto cross.
func (p *Parser) parseCrossFeaturePrefix(cross *ast.CrossFeatureMember) {
	for {
		t := p.peek()
		if p.atVarWord() {
			// KerML reserves `var`; in SysML it is the cross feature's name
			// unless a name or another modifier follows (`end var [1] item x`).
			next := p.peekN(1)
			if p.src.Kind() != source.KindKerML &&
				next.Kind != lexer.Identifier && next.Kind != lexer.UnrestrictedName && next.Kind != lexer.Lt &&
				!(next.Kind == lexer.Keyword && (featureModifierKeywords[next.KeywordID] || !p.reservedWord(next.KeywordID))) {
				return
			}
			cross.IsVariable = true
			p.advance()
			continue
		}
		if t.Kind != lexer.Keyword {
			return
		}
		switch t.KeywordID {
		case "in":
			cross.Direction = ast.DirIn
		case "out":
			cross.Direction = ast.DirOut
		case "inout":
			cross.Direction = ast.DirInOut
		case "derived":
			cross.IsDerived = true
		case "abstract":
			cross.IsAbstract = true
		case "variation":
			cross.IsVariation = true
		case "composite":
			cross.IsComposite = true
		case "portion":
			cross.IsComposite = true
			cross.IsPortion = true
		case "constant", "const":
			p.checkConstantSpelling(t)
			cross.IsConstant = true
		case "ref":
			cross.IsReference = true
		default:
			return
		}
		p.advance()
	}
}

// applyFeatureMods transfers modifiers that were consumed before a declaration's
// kind keyword onto the declaration itself, as in `ref part a : V`. Only
// modifiers that were present are applied, so a flag the declaration parsed for
// itself is never cleared.
func applyFeatureMods(decl ast.Node, mods featureMods) {
	switch d := decl.(type) {
	case *ast.Usage:
		if mods.isAbstract {
			d.IsAbstract = true
		}
		if mods.isVariation {
			d.IsVariation = true
		}
		if mods.isVariant {
			d.IsVariant = true
		}
		if mods.isReference {
			d.IsReference = true
		}
		if mods.isVariable {
			d.IsVariable = true
		}
		if mods.isEnd {
			d.IsEnd = true
		}
		if mods.isChain {
			d.IsChain = true
		}
		if mods.isConstant {
			d.IsConstant = true
		}
		if mods.isEvent {
			d.IsEvent = true
		}
		if mods.isIndividual {
			d.IsIndividual = true
		}
		if mods.portion != ast.PortionNone {
			d.Portion = mods.portion
		}
		if mods.isNegated {
			d.IsNegated = true
		}
		if mods.isComposite {
			d.IsComposite = true
		}
		if mods.isPortion {
			d.IsPortion = true
		}
		if mods.isDerived {
			d.IsDerived = true
		}
		if mods.isOrdered {
			d.IsOrdered = true
		}
		if mods.isNonunique {
			d.IsNonunique = true
		}
		if mods.direction != ast.DirNone {
			d.Direction = mods.direction
		}
		if mods.visibility != ast.VisibilityDefault {
			d.Visibility = mods.visibility
		}
		if mods.cross != nil && d.CrossFeature == nil {
			d.CrossFeature = mods.cross
		}
		if mods.prefixKeyword != "" && d.PrefixKeyword == "" {
			d.PrefixKeyword = mods.prefixKeyword
		}
	case *ast.Definition:
		if mods.isAbstract {
			d.IsAbstract = true
		}
		if mods.isVariation {
			d.IsVariation = true
		}
		if mods.isConstant {
			d.IsConstant = true
		}
		if mods.isEvent {
			d.IsEvent = true
		}
		if mods.visibility != ast.VisibilityDefault {
			d.Visibility = mods.visibility
		}
	}
}

// varPrefixWord marks a variable feature (KerML.xtext BasicFeaturePrefix,
// `isVariable ?= 'var'`). That is the only position where it is a keyword, so it
// is matched contextually like `point` and names a feature everywhere else.
const varPrefixWord = "var"

// useCaseKind is the declaration kind a use case definition or usage carries.
const useCaseKind = "use case"

// atVarWord reports whether the cursor is at the word `var`.
func (p *Parser) atVarWord() bool {
	t := p.peek()
	return t.Kind == lexer.Identifier && p.src.Text(t.Span) == varPrefixWord
}

// atVarPrefix reports whether the cursor is at the `var` prefix of a declaration
// rather than at a feature named `var`.
func (p *Parser) atVarPrefix() bool {
	if !p.atVarWord() {
		return false
	}
	next := p.peekN(1)
	return p.isKindKeyword(next) || next.Kind == lexer.Hash ||
		(next.Kind == lexer.Keyword && featureModifierKeywords[next.KeywordID])
}

// atVarDeclaration reports whether the cursor is at a `var`-prefixed
// declaration, whose kind keyword may be left out (`var x : Integer;`). A
// following name or `#M` prefix cannot continue an expression, so `var` there
// is the prefix; `var#(1)` indexes a feature named `var`.
func (p *Parser) atVarDeclaration() bool {
	if !p.atVarWord() {
		return false
	}
	next := p.peekN(1).Kind
	return p.isKindKeyword(p.peekN(1)) || next == lexer.Identifier || next == lexer.UnrestrictedName ||
		(next == lexer.Hash && p.peekN(2).Kind != lexer.LParen)
}

// kindPrefixWord returns the word at the cursor that may qualify a following
// kind keyword, or "" when the cursor is at no such word.
func (p *Parser) kindPrefixWord() string {
	if p.at(lexer.Keyword) {
		return p.peek().KeywordID
	}
	if p.atVarPrefix() {
		return varPrefixWord
	}
	return ""
}

// atKindPrefix reports whether the current keyword qualifies the kind keyword
// after it instead of being the kind itself, as in `var feature x` or
// `item part Shape`. When it does not, the second keyword names the declaration
// (`action flow { ... }` is an action named `flow`).
func (p *Parser) atKindPrefix() bool {
	kw := p.kindPrefixWord()
	if kw == "" || notKindPrefixKeywords[kw] {
		return false
	}
	// A feature modifier qualifies the declaration itself (`variation part v`),
	// so it is parsed as a modifier rather than dropped as a kind prefix.
	if featureModifierKeywords[kw] {
		return false
	}
	// `use case` is one kind spelled in two words, not a prefix and a kind.
	if p.atUseCase() {
		return false
	}
	if kw == varPrefixWord && (p.peekN(1).Kind == lexer.Hash ||
		p.peekN(1).Kind == lexer.Keyword && featureModifierKeywords[p.peekN(1).KeywordID]) {
		return true
	}
	if !p.isKindKeyword(p.peekN(1)) {
		return false
	}
	return kindPrefixKeywords[kw] || !namesDeclaration(p.peekN(2))
}

// atSecondaryKind reports whether the current kind keyword belongs to the kind
// of the declaration whose first kind keyword was already consumed, rather than
// being that declaration's name.
func (p *Parser) atSecondaryKind(firstKeyword string) bool {
	if notKindPrefixKeywords[firstKeyword] || !p.isKindKeyword(p.peek()) {
		return false
	}
	if compoundDefKinds[[2]string{firstKeyword, p.peek().KeywordID}] || kindPrefixKeywords[firstKeyword] {
		return true
	}
	return !namesDeclaration(p.peekN(1))
}

// atPortionedKind reports whether the current token is the kind keyword of the
// usage a portion prefix portions (`timeslice item : Cargo`). A kind keyword
// there is always the kind, never the portion's name, since the portion keyword
// itself is the only kind a bare portion usage declares.
func (p *Parser) atPortionedKind() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	_, ok := p.usageKind(p.peek().KeywordID)
	return ok
}

// isKindKeyword reports whether the token is a def or usage kind keyword.
func (p *Parser) isKindKeyword(t lexer.Token) bool {
	if t.Kind != lexer.Keyword {
		return false
	}
	_, isDef := p.definitionKind(t.KeywordID)
	_, isUsage := p.usageKind(t.KeywordID)
	return isDef || isUsage
}

// namesDeclaration reports whether a kind keyword followed by this token is the
// name of the declaration rather than its kind. A declaration that ends there
// (`;`), opens a body (`{`) or is typed (`:`) has nothing else to take its name
// from, so the kind keyword before it is the name; anything else — a name, a
// `def`, a redefinition — belongs to a declaration of that kind.
func namesDeclaration(t lexer.Token) bool {
	switch t.Kind {
	case lexer.Semicolon, lexer.LBrace, lexer.Colon:
		return true
	}
	return false
}

// atAssertedReference reports whether an `assert` names an existing constraint,
// as against stating a condition of its own: the name is the whole declaration,
// so only a body or a terminator may follow it.
func (p *Parser) atAssertedReference() bool {
	if p.isKindKeyword(p.peek()) || !p.namesReference(0) {
		return false
	}
	for i := 1; ; i += 2 {
		switch sep := p.peekN(i).Kind; sep {
		case lexer.Dot, lexer.ColonColon:
			if next := p.peekN(i + 1); next.Kind != lexer.Identifier &&
				next.Kind != lexer.UnrestrictedName && next.Kind != lexer.Keyword {
				return false
			}
		case lexer.Semicolon, lexer.LBrace, lexer.LBracket:
			return true
		default:
			return false
		}
	}
}

// namesReference reports whether the token at n can name a referenced usage —
// `assert c;`, `assert not c;` — rather than beginning an expression.
func (p *Parser) namesReference(n int) bool {
	t := p.peekN(n)
	switch t.Kind {
	case lexer.Identifier, lexer.UnrestrictedName:
		return true
	case lexer.Keyword:
		return !p.reservedWord(t.KeywordID)
	}
	return false
}

// atFeatureSpecialization reports whether the current token begins a feature
// specialization — typing, subsetting, reference, crossing or redefinition
// (SysML.xtext FeatureSpecialization). Both spellings of each clause answer
// here (`:`/`defined by`, `:>`/`subsets`, `::>`/`references`, `=>`/`crosses`,
// `:>>`/`redefines`), so neither can be read differently from the other.
// `specializes` is excluded: it relates two types (SubclassificationPart).
func (p *Parser) atFeatureSpecialization() bool { return p.featureSpecializationAt(0) }

// featureSpecializationAt is atFeatureSpecialization at the token i ahead.
func (p *Parser) featureSpecializationAt(i int) bool {
	t := p.peekN(i)
	switch t.Kind {
	case lexer.Colon, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt, lexer.EqGt:
		return true
	case lexer.Keyword:
		switch t.KeywordID {
		case "subsets", "references", "crosses", "redefines", "default":
			return true
		case "defined", "typed":
			n := p.peekN(i + 1)
			return n.Kind == lexer.Keyword && n.KeywordID == "by"
		}
	}
	return false
}

// beginsDeclarationTail reports whether the token after a name continues a
// keyword-less usage declaration (SysML.xtext DefaultReferenceUsage): a
// specialization or a feature value, e.g. `T1 = 10.0;`, `x :> y = e;`.
func beginsDeclarationTail(t, t2 lexer.Token) bool {
	switch t.Kind {
	case lexer.Eq, lexer.ColonEq, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt, lexer.EqGt:
		return true
	case lexer.Colon:
		// A typing (`kpl : DerivedUnit = km / L;`) names its type next.
		return t2.Kind == lexer.Identifier || t2.Kind == lexer.UnrestrictedName
	case lexer.Keyword:
		switch t.KeywordID {
		case "subsets", "references", "crosses", "redefines", "default":
			return true
		case "defined", "typed":
			return t2.Kind == lexer.Keyword && t2.KeywordID == "by"
		}
	}
	return false
}

// keywordlessFeatureAt reports whether the tokens at offset off declare a feature with
// no kind keyword: an identification (`<s>`? name?) followed by a declaration tail.
func (p *Parser) keywordlessFeatureAt(off int) bool {
	if p.peekN(off).Kind == lexer.Lt {
		if !p.atNameAt(off+1) || p.peekN(off+2).Kind != lexer.Gt {
			return false
		}
		off += 3
		if !p.atNameAt(off) {
			return p.keywordlessDeclarationTailAt(off)
		}
	}
	t := p.peekN(off)
	if !p.atNameAt(off) {
		return false
	}
	// KerML reserves `var`, so it prefixes a declaration and never names one.
	if t.Kind == lexer.Identifier && p.src.Kind() == source.KindKerML &&
		p.src.Text(t.Span) == varPrefixWord {
		return false
	}
	return p.keywordlessDeclarationTailAt(off + 1)
}

// keywordlessDeclarationTailAt reports whether the token off positions ahead
// continues a keyword-less feature declaration after its identification.
func (p *Parser) keywordlessDeclarationTailAt(off int) bool {
	switch next := p.peekN(off); next.Kind {
	case lexer.LBracket, lexer.Semicolon, lexer.LBrace:
		return true
	default:
		return beginsDeclarationTail(next, p.peekN(off+1))
	}
}

// atKindlessFeatureTyping reports whether a name is declared with no kind
// keyword and states a specialization or multiplicity (`mass : MassValue;`),
// the unambiguous half of a keyword-less declaration.
func (p *Parser) atKindlessFeatureTyping() bool {
	if t := p.peek(); t.Kind != lexer.Identifier && t.Kind != lexer.UnrestrictedName {
		return false
	}
	next := p.peekN(1)
	if next.Kind == lexer.LBracket {
		return true
	}
	if next.Kind == lexer.Eq || next.Kind == lexer.ColonEq || next.Kind == lexer.EqGt {
		return false
	}
	return beginsDeclarationTail(next, p.peekN(2))
}

// atEnumeratedValueDeclaration reports whether the enum-body member ahead is an
// EnumeratedValue the body loop reads itself: prefixed, anonymous or keywordless.
func (p *Parser) atEnumeratedValueDeclaration() bool {
	off := 0
	if t := p.peek(); t.Kind == lexer.Keyword {
		switch t.KeywordID {
		case "public", "private", "protected":
			off = 1
		}
	}
	end := p.prefixMetadataEndAt(off)
	prefixed := end > off
	off = end
	if t := p.peekN(off); t.Kind == lexer.Keyword && t.KeywordID == "enum" {
		if prefixed {
			return true
		}
		off++
		return p.peekN(off).Kind == lexer.Eq || p.peekN(off).Kind == lexer.ColonEq
	}
	switch t := p.peekN(off); t.Kind {
	case lexer.Eq, lexer.ColonEq, lexer.Lt:
		return true
	case lexer.Semicolon, lexer.LBrace:
		return prefixed
	}
	if p.atNameAt(off) {
		off++
		if endsEnumeratedValueName(p.peekN(off)) {
			return true
		}
	}
	return p.atFeatureSpecializationPartAt(off)
}

// atNameAt reports whether the token off positions ahead can begin a name
// segment, as atName does for the current token.
func (p *Parser) atNameAt(off int) bool {
	t := p.peekN(off)
	switch t.Kind {
	case lexer.Identifier, lexer.UnrestrictedName:
		return true
	case lexer.Keyword:
		return !p.reservedWord(t.KeywordID)
	}
	return false
}

// endsEnumeratedValueName reports whether t follows the name of an enumerated
// value that states no specialization: its value, body or `;`.
func endsEnumeratedValueName(t lexer.Token) bool {
	switch t.Kind {
	case lexer.Semicolon, lexer.LBrace, lexer.Eq, lexer.ColonEq:
		return true
	case lexer.Keyword:
		return t.KeywordID == "default"
	}
	return false
}

// prefixMetadataEndAt returns the offset just past the `#Name` prefixes that
// begin off tokens ahead, reading each name as parseQualifiedName does.
func (p *Parser) prefixMetadataEndAt(off int) int {
	for p.peekN(off).Kind == lexer.Hash {
		off++
		if p.peekN(off).Kind == lexer.Dollar && p.peekN(off+1).Kind == lexer.ColonColon {
			off += 2
		}
		for {
			if !p.atNameAt(off) {
				return off
			}
			off++
			if p.peekN(off).Kind != lexer.ColonColon {
				break
			}
			off++
		}
	}
	return off
}

// atFeatureSpecializationPartAt reports whether the token off positions ahead
// opens a FeatureSpecializationPart (KerML.xtext): a specialization or a
// multiplicity. Only such tokens are looked past, since reading beyond a `doc`
// or `;` would pull the trivia that follows into the current member.
func (p *Parser) atFeatureSpecializationPartAt(off int) bool {
	t := p.peekN(off)
	switch t.Kind {
	case lexer.LBracket, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt, lexer.EqGt:
		return true
	case lexer.Colon:
		off++
		if p.peekN(off).Kind == lexer.Dollar && p.peekN(off+1).Kind == lexer.ColonColon {
			off += 2
		}
		return p.atNameAt(off)
	case lexer.Keyword:
		switch t.KeywordID {
		case "subsets", "references", "crosses", "redefines", "default":
			return true
		case "defined", "typed":
			n := p.peekN(off + 1)
			return n.Kind == lexer.Keyword && n.KeywordID == "by"
		}
	}
	return false
}

// atKeywordlessFeature reports whether the cursor is at such a feature, either
// directly or behind a `var` prefix (`var p : Real;`).
func (p *Parser) atKeywordlessFeature() bool {
	if p.atVarPrefixedFeature() {
		return true
	}
	return p.keywordlessFeatureAt(0)
}

// atVarPrefixedFeature reports whether `var` prefixes a keyword-less feature
// rather than naming one. `var` is a prefix only in KerML (KerML.xtext:520).
func (p *Parser) atVarPrefixedFeature() bool {
	return p.src.Kind() == source.KindKerML && p.atVarWord() && p.keywordlessFeatureAt(1)
}

// atTextualRepresentationStart matches the KerML TextualRepresentation head
// (`rep` Identification? `language` String), including its language-only spelling.
func (p *Parser) atTextualRepresentationStart() bool {
	if p.atKeyword("rep") {
		next := p.peekN(1)
		return next.Kind == lexer.Identifier ||
			next.Kind == lexer.UnrestrictedName ||
			next.Kind == lexer.Lt || // a short name, `rep <ocl> language "ocl"`
			(next.Kind == lexer.Keyword && next.KeywordID == "language")
	}
	return p.atKeyword("language") && p.peekN(1).Kind == lexer.String
}

// atDefUsageStart reports whether the current token begins a def/usage
// declaration: a feature specialization stated in place of a name, a
// conjugation, a feature-modifier keyword or a kind keyword.
func (p *Parser) atDefUsageStart() bool {
	t := p.peek()
	if p.atFeatureSpecialization() || t.Kind == lexer.Tilde {
		return true
	}
	if p.atTextualRepresentationStart() {
		return true
	}
	if p.atVarPrefixedFeature() || p.atVarPrefix() {
		return true
	}
	if t.Kind != lexer.Keyword {
		return false
	}
	if featureModifierKeywords[t.KeywordID] {
		return true
	}
	if t.KeywordID == "use" {
		return p.atUseCase()
	}
	// `connect` starts an anonymous connection usage without being a kind
	// keyword: `connection c connect a to b` states it after the kind.
	if t.KeywordID == "connect" {
		return true
	}
	if t.KeywordID == "not" {
		return p.atNegatedSatisfy()
	}
	_, isDef := p.definitionKind(t.KeywordID)
	_, isUsage := p.usageKind(t.KeywordID)
	return isDef || isUsage
}

// atNegatedSatisfy reports whether the cursor is at `not satisfy`, the
// satisfaction negated without an `assert` before it. `verify` has no negated
// form (SysML.xtext:2118 is the only production spelling `isNegated`).
func (p *Parser) atNegatedSatisfy() bool {
	if !p.atKeyword("not") {
		return false
	}
	n := p.peekN(1)
	return n.Kind == lexer.Keyword && n.KeywordID == "satisfy"
}

// atUseCase reports whether the current token is `use` immediately followed by
// `case` (the two-word use-case kind keyword).
func (p *Parser) atUseCase() bool {
	if !p.atKeyword("use") {
		return false
	}
	n := p.peekN(1)
	return n.Kind == lexer.Keyword && n.KeywordID == "case"
}

// isModifierOrKindKeyword checks if keyword is a modifier or def/usage kind keyword
func (p *Parser) isModifierOrKindKeyword(kw string) bool {
	_, isMod := featureModifierKeywords[kw]
	_, isDef := p.definitionKind(kw)
	_, isUsage := p.usageKind(kw)
	return isMod || isDef || isUsage
}

// atDirectionKeyword reports whether the cursor is at a feature direction.
func (p *Parser) atDirectionKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	switch p.peek().KeywordID {
	case "in", "out", "inout":
		return true
	}
	return false
}

// chainWord marks a feature chain declaration (`attribute chain a.b;`). It is
// a modifier only when a name follows it; otherwise it names the feature.
const chainWord = "chain"

// atChainWord reports whether the cursor is at the word `chain`.
func (p *Parser) atChainWord() bool {
	t := p.peek()
	return t.Kind == lexer.Identifier && p.src.Text(t.Span) == chainWord
}

// atChainModifier reports whether the cursor is at the `chain` modifier of a
// declaration rather than at a feature named `chain`: what follows must spell a
// name (atName), so a word this grammar reserves — `ordered`, `specializes`,
// `default`, `about`, … — leaves `chain` as the name.
func (p *Parser) atChainModifier() bool {
	if !p.atChainWord() {
		return false
	}
	next := p.peekN(1)
	switch next.Kind {
	case lexer.Identifier, lexer.UnrestrictedName, lexer.ColonColon, lexer.Lt:
		return true
	case lexer.Keyword:
		return !p.reservedWord(next.KeywordID)
	}
	return false
}

func (p *Parser) parseFeatureModifiers() featureMods {
	var m featureMods
	p.parseMoreFeatureModifiers(&m)
	return m
}

// parseMoreFeatureModifiers adds the modifier keywords at the cursor to m.
func (p *Parser) parseMoreFeatureModifiers(m *featureMods) {
	for {
		t := p.peek()
		if p.atChainWord() {
			if p.atChainModifier() {
				m.isChain = true
				p.advance()
				continue
			}
			return
		}
		if t.Kind == lexer.Identifier && p.src.Text(t.Span) == varPrefixWord {
			next := p.peekN(1)
			// KerML FeaturePrefix puts prefix metadata after `var`: `var #M feature f`.
			isModifier := p.isKindKeyword(next) || next.Kind == lexer.Hash ||
				(next.Kind == lexer.Keyword && featureModifierKeywords[next.KeywordID]) ||
				p.atVarPrefixedFeature()
			if isModifier {
				m.isVariable = true
				m.prefixKeyword = varPrefixWord
				p.advance()
				continue
			}
			return
		}
		if t.Kind != lexer.Keyword {
			return
		}
		switch t.KeywordID {
		case "abstract":
			if m.isAbstract || m.isVariation {
				p.prefixConflict(t, m.abstractOrVariation(), "'abstract' or 'variation'")
			}
			m.isAbstract = true
		case "variation":
			if m.isAbstract || m.isVariation {
				p.prefixConflict(t, m.abstractOrVariation(), "'abstract' or 'variation'")
			}
			m.isVariation = true
		case "ref":
			m.isReference = true
			m.noteUsageOnly(t)
		case "end":
			m.isEnd = true
			m.noteUsageOnly(t)
			p.advance() // consume "end"
			m.cross = p.tryParseCrossFeature()
			continue
		case "constant", "const":
			p.checkConstantSpelling(t)
			m.isConstant = true
		case "event":
			// Check if standalone usage: event <name>; (no typing/body)
			// If followed by identifier/qualified name (not keyword), it's usage keyword
			nextTok := p.peekN(1)
			if nextTok.Kind == lexer.Identifier || (nextTok.Kind == lexer.Keyword && !p.isModifierOrKindKeyword(nextTok.KeywordID)) {
				// Treat as usage keyword, stop consuming modifiers
				return
			}
			m.isEvent = true
		case "individual":
			// `individual` is a modifier orthogonal to the kind keyword (SysML v2
			// §8.3.9.11), except before `def` or a typing/specialization token,
			// where it is the declaration's own keyword.
			nextTok := p.peekN(1)
			if nextTok.Kind == lexer.Colon || nextTok.Kind == lexer.ColonGt || nextTok.Kind == lexer.ColonGtGt {
				// individual : Type → anonymous usage
				return
			}
			if nextTok.Kind == lexer.Keyword && nextTok.KeywordID == "def" {
				// individual def → DefIndividual keyword
				return
			}
			m.isIndividual = true
		case "snapshot":
			// The portion loop in parseDefUsage reads a portion prefix the same way
			// for either keyword; only another modifier after it is handled here.
			nextTok := p.peekN(1)
			if nextTok.Kind != lexer.Keyword || !featureModifierKeywords[nextTok.KeywordID] {
				return
			}
			m.portion = ast.PortionSnapshot
		case "public":
			m.visibility = ast.VisibilityPublic
		case "protected":
			m.visibility = ast.VisibilityProtected
		case "private":
			m.visibility = ast.VisibilityPrivate
		case "in", "out", "inout":
			if m.direction != ast.DirNone {
				p.prefixConflict(t, m.direction.String(), "one direction ('in', 'out' or 'inout')")
			}
			m.direction = directionOf(t.KeywordID)
			m.noteUsageOnly(t)
		case "composite":
			if m.isPortion {
				p.prefixConflict(t, "portion", "'composite' or 'portion'")
			}
			m.isComposite = true
			m.noteUsageOnly(t)
		case "portion":
			if m.isComposite {
				p.prefixConflict(t, "composite", "'composite' or 'portion'")
			}
			// A portion is composite in addition to being a portion.
			m.isComposite = true
			m.isPortion = true
			m.noteUsageOnly(t)
		case "readonly":
			m.isReadonly = true
			m.noteUsageOnly(t)
		case "derived":
			m.isDerived = true
			m.noteUsageOnly(t)
		case "ordered":
			m.isOrdered = true
			m.noteUsageOnly(t)
		case "nonunique":
			m.isNonunique = true
			m.noteUsageOnly(t)
		default:
			return
		}
		p.advance()
	}
}

// parsePostModifiers parses feature modifiers that appear after typing/multiplicity.
// Currently only 'ordered' and 'nonunique' are allowed in this position.
// isPostModifierKeyword checks if token is a post-multiplicity modifier keyword
func isPostModifierKeyword(tok lexer.Token) bool {
	if tok.Kind != lexer.Keyword {
		return false
	}
	return tok.KeywordID == "ordered" || tok.KeywordID == "nonunique"
}

func (p *Parser) parsePostModifiers() featureMods {
	var m featureMods
	for {
		t := p.peek()
		if t.Kind != lexer.Keyword {
			return m
		}
		switch t.KeywordID {
		case "ordered":
			m.isOrdered = true
			p.advance()
		case "nonunique":
			m.isNonunique = true
			p.advance()
		case "terminate":
			// Consume terminate keyword - marks terminal action node
			// For now just consume it (no AST field, behavioral semantics)
			p.advance()
		default:
			return m
		}
	}
}

// modifierImpliedKind gives the kind an occurrence modifier declares with no kind
// keyword after it (SysML.xtext IndividualUsage, PortionUsage, EventOccurrenceUsage).
func modifierImpliedKind(mods featureMods) (ast.UsageKind, string) {
	switch {
	case mods.isIndividual:
		return ast.UsageIndividual, "individual"
	case mods.portion == ast.PortionSnapshot:
		return ast.UsageOccurrence, "snapshot"
	case mods.portion == ast.PortionTimeslice:
		return ast.UsageOccurrence, "timeslice"
	case mods.isEvent:
		return ast.UsageOccurrence, "event"
	}
	return ast.UsageAttribute, ""
}

// parsePortionPrefix reads a `snapshot`/`timeslice` prefix (SysML v2 8.3.9.11) into
// mods and returns the usage itself when no kind keyword follows (`timeslice t;`).
func (p *Parser) parsePortionPrefix(start int, mods *featureMods, prefixes *[]*ast.PrefixMetadata) *ast.Usage {
	for p.atKeyword("snapshot") || p.atKeyword("timeslice") {
		tok := p.advance()
		portion := ast.PortionSnapshot
		if tok.KeywordID == "timeslice" {
			portion = ast.PortionTimeslice
		}
		if mods.portion != ast.PortionNone {
			p.error(tok.Span, "a usage declares at most one portion kind ('snapshot' or 'timeslice')")
		}
		mods.portion = portion
		// The portion kind is part of the usage prefix, so prefix metadata may
		// still follow it: `snapshot #Classified part s;`.
		*prefixes = append(*prefixes, p.parsePrefixMetadata()...)
		if !p.atPortionedKind() {
			isAll := p.acceptSufficientAll()
			return p.parseUsage(start, ast.UsageOccurrence, tok.KeywordID, *mods, isAll)
		}
	}
	return nil
}

// parseDefUsage parses a definition or usage declaration. The caller has
// already established (via atDefUsageStart) that a def/usage begins here.
func (p *Parser) parseDefUsage(start int) ast.Node {
	// Parse optional `#MetadataType` prefixes (user-defined keywords)
	prefixes := p.parsePrefixMetadata()

	// Helper to apply prefixes to result node
	applyPrefixes := func(node ast.Node) ast.Node {
		if len(prefixes) == 0 {
			return node
		}
		if u, ok := node.(*ast.Usage); ok {
			u.Prefixes = append(prefixes, u.Prefixes...)
		} else if d, ok := node.(*ast.Definition); ok {
			d.Prefixes = append(prefixes, d.Prefixes...)
		}
		return node
	}

	// A specialization where the name would go declares an unnamed usage of the
	// default kind (`:>> x`, `redefines x`), whichever spelling it was written in.
	if p.atFeatureSpecialization() {
		u := p.parseUsage(start, ast.UsageAttribute, "", featureMods{}, false)
		return applyPrefixes(u)
	}

	mods := p.parseFeatureModifiers()

	// A satisfaction is negated with no `assert` before it: `not satisfy r by p;`
	// (SysML.xtext:2118, SatisfyRequirementUsage `'assert'? isNegated ?= 'not'?`).
	if p.atNegatedSatisfy() {
		mods.isNegated = true
		p.advance() // 'not'
	}

	// UsagePrefix ends in UsageExtensionKeyword* (SysML.xtext:582), so prefix
	// metadata may follow the modifiers: `abstract #Classified z;`.
	prefixes = append(prefixes, p.parsePrefixMetadata()...)

	if u := p.parsePortionPrefix(start, &mods, &prefixes); u != nil {
		return applyPrefixes(u)
	}

	// Two-word `use case` kind keyword.
	if p.atUseCase() {
		p.advance() // 'use'
		p.advance() // 'case'
		if p.atKeyword("def") {
			p.advance() // 'def'
			return applyPrefixes(p.parseDefinition(start, ast.DefUseCase, useCaseKind, mods, false, true))
		}
		return applyPrefixes(p.parseUsage(start, ast.UsageUseCase, useCaseKind, mods, false))
	}

	t := p.peek()
	kw := ""
	if t.Kind == lexer.Keyword {
		kw = t.KeywordID
	}

	// Special case: perform action <name> (declaration form)
	// Pattern: perform action generateTorque: GenerateTorque;
	// Skip "perform" and parse as regular "action" usage
	if kw == "perform" && p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "action" {
		p.advance() // consume 'perform'
		// The prefix says the action is performed by whatever declares it, which
		// the kind keyword alone would lose (PerformActionUsage, SysML v2 §7.17.6).
		mods.prefixKeyword = "perform"
		kw = "action" // treat as regular action keyword
		// Continue to dual-keyword path (don't enter usage-only block)
	} else if kw == "subject" || kw == "objective" || kw == "succession" || kw == "inv" || kw == "connect" || kw == "connector" || kw == "bind" || kw == "satisfy" || kw == "verify" || kw == "include" || kw == "step" || kw == "expr" || kw == "interaction" || kw == "require" || kw == "transition" || kw == "perform" || kw == "exhibit" || kw == "variant" || kw == "assert" || kw == "assume" || kw == "event" || kw == "stakeholder" || kw == "frame" || kw == "actor" || kw == "expose" || kw == "render" || kw == "allocate" {
		// Check for usage-only keywords that never have def forms

		// Special case: perform <ref>; (shorthand without action keyword)
		// Must check BEFORE consuming keyword token
		if kw == "perform" && !p.atKeyword("action") {
			p.advance() // consume 'perform'
			return applyPrefixes(p.parsePerformedActionReference(start, mods, "perform"))
		}

		// A member keyword can also be an ordinary name: KerML has no `frame` or
		// `render` keyword, and the Kernel Semantic Library writes `in frame :
		// SpatialFrame[1]`. Read the keyword as the declaration's own name
		// unless what follows can begin the member form.
		if memberKeywordNames[kw] && !p.atMemberKeywordUsedAsKeyword(kw) {
			return applyPrefixes(p.parseUsage(start, ast.UsageAttribute, "", mods, false))
		}

		// `connect a to b { … }` is a connection usage whose ends the keyword
		// introduces; it declares nothing of its own (SysML.xtext ConnectionUsage).
		if kw == "connect" {
			p.advance() // consume 'connect'
			return applyPrefixes(p.parseUsage(start, ast.UsageConnection, "connect", mods, false))
		}

		// `inv true v { … }` and `inv false v { … }`: the truth keyword is the
		// invariant's polarity, not its name (KerML.xtext Invariant, isNegated ?= 'false').
		if kw == "inv" && (p.peekN(1).KeywordID == "true" || p.peekN(1).KeywordID == "false") {
			p.advance() // 'inv'
			mods.isNegated = p.peek().KeywordID == "false"
			p.advance()
			return applyPrefixes(p.parseUsage(start, ast.UsageConstraint, "inv", mods, false))
		}

		if len(prefixes) > 0 && prefixMetadataFollowsKeyword(kw) {
			p.reportMisplacedPrefixMetadata(prefixes, t)
		}

		// A subject, actor, stakeholder, objective or rendering the body does
		// not own is parsed as itself and then replaced by an ErrorNode.
		if !p.bodyAdmitsMember(kw) {
			en := p.misplacedMember(t)
			applyPrefixes = func(ast.Node) ast.Node {
				en.NodeSpan = p.spanFrom(start)
				return en
			}
		}

		p.advance() // consume the kind keyword
		if prefixMetadataFollowsKeyword(kw) {
			prefixes = append(prefixes, p.parsePrefixMetadata()...)
		}
		// `variant x` declares a variant of the variation that owns it
		// (VariantMembership, SysML v2 §7.20).
		if kw == "variant" {
			mods.isVariant = true
			// The variant element carries its own usage prefix (SysML.xtext
			// VariantUsageElement → OccurrenceUsagePrefix): `variant ref port a;`.
			p.parseMoreFeatureModifiers(&mods)
			prefixes = append(prefixes, p.parsePrefixMetadata()...)
			if u := p.parsePortionPrefix(start, &mods, &prefixes); u != nil {
				return applyPrefixes(u)
			}
		}
		isAll := p.acceptSufficientAll()
		// `render` names the rendering a view uses (ViewRenderingMember) and
		// `frame` the concern a requirement frames (FramedConcernMember). Each
		// owns a usage that either references an existing element —
		// `render asTreeDiagram;`, `frame 'system breakdown';` — or declares one
		// after the kind keyword the notation spells out, `render rendering r`
		// and `frame concern c` (SysML.xtext ViewRenderingUsage,
		// FramedConcernUsage; SysML v2 §8.3.20, §8.3.26). Only the declaration
		// form states a name.
		if kw == "render" || kw == "frame" {
			declKeyword, noun, body := "rendering", "rendering", p.parseDefUsageBodyMembers
			if kw == "frame" {
				declKeyword, noun, body = "concern", "concern", p.parseRequirementBody
			}
			if p.acceptKeyword(declKeyword) {
				return applyPrefixes(p.parseUsage(start, p.usageKindOf(kw), kw, mods, isAll))
			}
			return applyPrefixes(p.parseReferenceMemberUsage(start, p.usageKindOf(kw), kw, noun, mods, body, false))
		}

		// Only `exhibit state` declares a state; every other exhibit names an
		// existing one through an OwnedReferenceSubsetting (SysML.xtext
		// ExhibitStateUsage; SysML v2 §8.3.17).
		if kw == "exhibit" {
			if p.acceptKeyword("state") {
				return applyPrefixes(p.parseUsage(start, ast.UsageState, "exhibit state", mods, isAll))
			}
			return applyPrefixes(p.parseReferenceMemberUsage(
				start, ast.UsageState, kw, "state", mods, p.parseStateBody, true))
		}

		// Special case: include use case <name> (full form)
		// If include is followed by "use case", consume them and parse as use case with includes relationship
		if kw == "include" && p.atUseCase() {
			p.advance() // consume 'use'
			p.advance() // consume 'case'
			// The prefix says the use case is included, which the kind keyword alone
			// would lose (IncludeUseCaseUsage, SysML v2 §7.23.4).
			mods.prefixKeyword = "include"
			u := p.parseUsage(start, ast.UsageUseCase, useCaseKind, mods, isAll)
			if u != nil {
				// Add includes relationship to first typing target
				// Actually, include use case <name> : Type means: create use case usage <name> typed by Type, with includes semantics
				// The includes relationship is implicit in the 'include' keyword context
				// For now, we'll add an includes relationship with nil target (self-referential)
				// Or use a special flag. But spec may expect includes to target the typing.
				// Simplest: add includes relationship AFTER typing parsed
				if len(u.Relationships) > 0 && u.Relationships[0].Kind == ast.RelTyping {
					// Insert includes relationship pointing to typing target
					typing := u.Relationships[0].Target
					u.Relationships = append([]*ast.Relationship{
						{Kind: ast.RelIncludes, Target: typing},
					}, u.Relationships...)
				}
			}
			return applyPrefixes(u)
		}

		// A guard between the ends makes the succession a transition
		// (SysML.xtext:1719 GuardedSuccession returns TransitionUsage):
		// `succession S first a if g then b;`.
		if kw == "succession" && p.atGuardedSuccession() {
			node := p.parseTransitionMember(start)
			if tm, ok := node.(*ast.TransitionMember); ok {
				tm.IsSuccession = true
			}
			return applyPrefixes(node)
		}

		// `succession flow from X to Y` is a flow that is also a succession
		// (SysML.xtext:1278 SuccessionFlowUsage); the prefix keeps it apart.
		if kw == "succession" && p.atKeyword("flow") {
			p.advance()
			mods.prefixKeyword = kw
			return applyPrefixes(p.parseUsage(start, ast.UsageFlow, "flow", mods, isAll))
		}

		// `event m.start;` names an existing occurrence rather than declaring
		// one, and takes a value part like any usage (SysML.xtext
		// EventOccurrenceUsage: `'event' ( OwnedReferenceSubsetting
		// FeatureSpecializationPart? | OccurrenceUsageKeyword UsageDeclaration? )
		// UsageCompletion`).
		if kw == "event" && !p.atKeyword("occurrence") && !p.at(lexer.Colon) {
			u := p.parseReferenceMemberUsage(
				start, ast.UsageOccurrence, kw, "occurrence", mods, p.parseDefUsageBodyMembers, true)
			u.IsEvent = true
			return applyPrefixes(u)
		}

		// Special case: include <ref>; (shorthand for use case with includes relationship)
		// If include is NOT followed by "use case", parse as use case usage with includes
		// Pattern: include <ref>[mult] { body };
		if kw == "include" && !p.atUseCase() {
			u := &ast.Usage{
				Kind:        ast.UsageUseCase,
				IsAbstract:  mods.isAbstract,
				IsReference: mods.isReference,
				IsVariable:  mods.isVariable,
				IsEnd:       mods.isEnd,
				Visibility:  mods.visibility,
				Direction:   mods.direction,
				IsComposite: mods.isComposite,
				IsPortion:   mods.isPortion,
			}
			u.NodeBase.NodeSpan = p.spanFrom(start)

			// Parse reference target
			target := p.parseRelationshipTarget()
			if target != nil {
				// Add as includes relationship
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelIncludes,
					Target: target,
				})
			} else {
				// The reference form of `include` subsets an existing use case, so it
				// names one (SysML.xtext:2300 IncludeUseCaseUsage).
				p.error(p.peek().Span, "expected a use case reference after 'include'")
			}

			// Optional multiplicity after reference
			if p.at(lexer.LBracket) {
				u.Multiplicity = p.parseMultiplicity()
			}

			// Expect semicolon or body
			if p.accept2(lexer.Semicolon) {
				u.HasBody = false
			} else if p.at(lexer.LBrace) {
				p.advance() // consume '{'
				// Parse body members
				var members []ast.Node
				leave := p.pushBodyContext(usageBodyContext(ast.UsageUseCase))
				for !p.at(lexer.RBrace) && !p.atEOF() {
					m := p.parseBodyMember()
					if m != nil {
						members = append(members, m)
					}
				}
				leave()
				p.expect(lexer.RBrace, msgExpectedBodyClose)
				u.Members = members
				u.HasBody = true
			}

			u.NodeSpan = p.spanFrom(start)
			return applyPrefixes(u)
		}

		// `assert constraint { ... }` (likewise `assume`/`require`) spells the
		// kind after the prefix: the second keyword is the kind, so the
		// declaration is an anonymous constraint rather than one named
		// `constraint`.
		// `variant` likewise prefixes a kind when a name follows it
		// (`variant attribute diameterSmall = 70[mm];`); with no name, the
		// second keyword is the variant's own name.
		// `assert not constraint { … }` and `assert not satisfy … by …` negate the
		// declaration the prefix qualifies (Invariant::isNegated), so the `not`
		// belongs to it rather than to an expression. `assert not c { … }` negates
		// the reference form the same way, where `assert not (x > 1);` does negate
		// an expression (SysML.xtext:2008, AssertConstraintUsage).
		if kindPrefixKeywords[kw] && p.atKeyword("not") &&
			(p.isKindKeyword(p.peekN(1)) || p.namesReference(1)) {
			mods.isNegated = true
			p.advance()
		}

		// A `not` with nothing after it negates neither a declaration nor an
		// expression, so the assertion states no condition at all.
		if kindPrefixKeywords[kw] && p.atKeyword("not") {
			switch p.peekN(1).Kind {
			case lexer.Semicolon, lexer.RBrace, lexer.EOF:
				p.advance() // 'not'
				return p.errorNodeSkip(start, "expected a condition after 'not'")
			}
		}

		// `assert c;` and `assert not c { … }` name an existing constraint rather
		// than declaring one named `c` (SysML.xtext:2009, AssertConstraintUsage's
		// OwnedReferenceSubsetting).
		if kw == "assert" && p.atAssertedReference() {
			u := p.parseReferenceMemberUsage(
				start, ast.UsageConstraint, kw, "constraint", mods, p.parseConstraintBody, true)
			u.IsNegated = mods.isNegated
			return applyPrefixes(u)
		}

		// A prefix keyword qualifies a two-word kind as well as a one-word one:
		// `variant use case uc11;` is a use case usage (SysML.xtext:700,
		// VariantUsageElement).
		if (kindPrefixKeywords[kw] || kw == "variant") && p.atUseCase() {
			p.advance() // 'use'
			p.advance() // 'case'
			if kindPrefixKeywords[kw] {
				mods.prefixKeyword = kw
			}
			return applyPrefixes(p.parseUsage(start, ast.UsageUseCase, useCaseKind, mods, isAll))
		}

		kindKeyword := kw
		if p.isKindKeyword(p.peek()) &&
			(kindPrefixKeywords[kw] || (kw == "variant" && !namesDeclaration(p.peekN(1)))) {
			kindKeyword = p.peek().KeywordID
			// `variant` is a modifier of the declaration it prefixes, recorded
			// as isVariant; a prefix keyword says what the declaration is for.
			if kindPrefixKeywords[kw] {
				mods.prefixKeyword = kw
			}
			p.advance()
		}
		return applyPrefixes(p.parseUsage(start, p.usageKindOf(kindKeyword), kindKeyword, mods, isAll))
	}

	// Special case: if current token is 'def' (after prefixes/modifiers), parse as generic definition
	// This handles patterns like `#scenario def X` where prefix acts as semantic annotation
	if p.atKeyword("def") {
		p.advance() // consume 'def'
		// Use generic definition kind (or could extract from prefix)
		return applyPrefixes(p.parseDefinition(start, ast.DefClass, "", mods, false, true))
	}

	defKind, ok := p.definitionKind(kw)
	if !ok {
		// Fallback: if we have modifiers but no kind keyword, assume it's a generic usage (e.g., "in x: Integer;")
		// This is common for parameters in calc/action bodies.
		// Also check if name + multiplicity/modifiers follow (e.g., "in seq[1..*] ordered;")
		// An `end` whose declaration is omitted entirely is only an interface
		// default end or an `end ref` (SysML v2 8.2.2.14.1).
		if mods.isEnd && (p.at(lexer.Semicolon) || p.at(lexer.LBrace)) {
			return applyPrefixes(p.parseAnonymousEndUsage(start, mods))
		}

		hasModifiers := mods.direction != ast.DirNone || mods.isReference || mods.isEnd ||
			mods.isComposite || mods.isDerived || mods.isIndividual || mods.isEvent ||
			mods.portion != ast.PortionNone
		hasNameWithMultOrMods := p.atNameOrKeyword() && (p.peekN(1).Kind == lexer.LBracket || p.peekN(1).Kind == lexer.Colon || isPostModifierKeyword(p.peekN(1)))
		// A DefaultReferenceUsage needs no keyword at all (SysML.xtext:632):
		// `T1 = 10.0;`, `distancePerVolume :> scalarQuantities = d / v;`.
		keywordlessDecl := p.atKeywordlessFeature()
		// SysML v2 §7.27.4: a user-defined keyword may declare a usage without
		// any language-defined keyword (`#cause 'battery old' { ... }`). The
		// kind of such a usage comes from the metadata, not the syntax.
		keywordOnlyUsage := len(prefixes) > 0 &&
			(p.at(lexer.Identifier) || p.at(lexer.UnrestrictedName) || p.atFeatureSpecialization())
		if hasModifiers || hasNameWithMultOrMods || keywordOnlyUsage || keywordlessDecl {
			kind, keyword := modifierImpliedKind(mods)
			return applyPrefixes(p.parseUsage(start, kind, keyword, mods, false))
		}
		return applyPrefixes(nil)
	}
	p.advance() // consume the kind keyword

	// `individual : T` is an anonymous individual occurrence usage; `snapshot` is
	// handled with the usage-only keywords above.
	if kw == "individual" {
		mods.isIndividual = true
	}

	// Parse 'all' modifier if present (appears after keyword, before name)
	isAll := p.acceptSufficientAll()

	if p.atChainModifier() {
		mods.isChain = true
		p.advance()
	}

	// Parse a secondary kind keyword, which happens either in a compound kind
	// (`assoc struct`) or when the first keyword prefixes the second
	// (`item part Shape`). A kind keyword that is not one of those is the
	// declaration's name (`attribute item : Integer` names the attribute
	// `item`), and consuming it here would silently discard that name.
	kindKeyword := kw
	if p.atSecondaryKind(kw) {
		kindKeyword = p.peek().KeywordID
		defKind = p.definitionKindOf(kindKeyword)
		p.advance() // consume secondary keyword
	}

	if p.atKeyword("def") {
		p.advance() // consume 'def'
		return applyPrefixes(p.parseDefinition(start, defKind, kindKeyword, mods, isAll, true))
	}

	// Check if this is a definition-only keyword (not in usageKindKeywords)
	// Examples: metaclass, struct, class, predicate, bool
	// These keywords don't require "def" suffix and can't be used as usages
	_, hasUsageForm := p.usageKind(kw)
	if !hasUsageForm {
		// Definition-only keyword - parse as definition directly
		return applyPrefixes(p.parseDefinition(start, defKind, kindKeyword, mods, isAll, false))
	}

	// Note: 'datatype' is treated uniformly as a usage keyword by the parser.
	// Semantic classification (def vs usage) is deferred to the symbol builder
	// and semantics passes, which have full context (relationships, body structure).
	// This follows Phase 4 principle: parse syntax uniformly, classify semantically.

	// A secondary keyword refines a definition's kind (`individual item def`),
	// but a usage keeps the kind of the first keyword (`item part Shape` is an
	// item usage), so the keyword recorded here is that first one; the compound
	// `assoc struct` keeps both words since it declares an association structure.
	keyword := kw
	if kw == "assoc" && kindKeyword == "struct" {
		keyword = "assoc struct"
	}
	return applyPrefixes(p.parseUsage(start, p.usageKindOf(kw), keyword, mods, isAll))
}

// parseDefinition parses a definition. keyword is the kind keyword as consumed
// from the token stream, kept so a synonym spelling can be told from the
// canonical one. defKeywordConsumed distinguishes SysML <kind> def from
// KerML classifier declarations, which share DefinitionKind values.
func (p *Parser) parseDefinition(start int, kind ast.DefinitionKind, keyword string, mods featureMods, isAll bool, defKeywordConsumed bool) *ast.Definition {
	// DefinitionPrefix admits only `abstract`/`variation` (SysML.xtext:498),
	// TypePrefix only `abstract` (KerML.xtext:313); `ref`, a direction, … are usage prefixes.
	if mods.usageOnly.Span.Len > 0 {
		admitted := "'abstract' or 'variation'"
		if p.src.Kind() == source.KindKerML {
			admitted = "'abstract'"
		}
		p.error(mods.usageOnly.Span, fmt.Sprintf("'%s' is a usage prefix: a definition prefix admits only %s", mods.usageOnly.KeywordID, admitted))
	}
	def := &ast.Definition{
		Kind:          kind,
		Keyword:       keyword,
		HasDefKeyword: defKeywordConsumed,
		IsAbstract:    mods.isAbstract,
		IsVariation:   mods.isVariation,
		IsAll:         isAll,
		IsConstant:    mods.isConstant,
		IsEvent:       mods.isEvent,
		IsIndividual:  mods.isIndividual,
		Visibility:    mods.visibility,
		Ident:         p.parseIdentification(),
	}
	if !defKeywordConsumed && isKerMLClassifierDefinitionKeyword(keyword) && p.at(lexer.LBracket) {
		def.Multiplicity = p.parseMultiplicity()
	}
	def.Relationships = p.parseRelationships(false)

	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
	defer p.pushBodyContext(defBodyContext(kind))()
	switch kind {
	case ast.DefAction, ast.DefOccurrence:
		// Action/occurrence def bodies: mixed (declarations + behavioral statements)
		// Occurrence defs support temporal ordering of messages/events (interactions)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseActionBodyMixed()
			hasBody = true
		}
	case ast.DefCalc:
		// Calculation def bodies: mixed (parameters + return statements)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseCalcBody()
			hasBody = true
		}
	case ast.DefConstraint:
		// Constraint def bodies: always use parseConstraintBody (handles assert/assume/bare expressions)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseConstraintBody()
			hasBody = true
		}
	case ast.DefRequirement, ast.DefConcern, ast.DefViewpoint:
		// Requirement def bodies: a requirement member may appear anywhere in the
		// body, not only first, so the whole body is parsed as a requirement body
		// (which falls back to general body members). A concern definition is a
		// requirement definition and a viewpoint definition a concern definition
		// (SysML v2 §7.19), so all three carry require/assume/subject/actor.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseRequirementBody()
			hasBody = true
		}
	case ast.DefState:
		// State def bodies are state bodies, like state usage bodies: what the
		// first member happens to be does not change what the rest may be, and
		// the generic body member parser knows nothing of regions or transitions.
		// `parallel` marks the substates orthogonal, and only a body may follow it
		// (SysML.xtext StateDefBody).
		if p.atKeyword("parallel") {
			def.IsParallel = true
			p.advance()
			if _, ok := p.expect(lexer.LBrace, "expected '{' after 'parallel'"); ok {
				members = p.parseStateBody()
				hasBody = true
			}
			break
		}
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseStateBody()
			hasBody = true
		}
	case ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
		// Case bodies may end in a ResultExpressionMember (SysML.xtext
		// CalculationBodyPart): `vehicle.mass` as the last member.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseCaseBody()
			hasBody = true
		}
	case ast.DefEnumeration:
		// Enumeration bodies hold EnumeratedValues, whose keyword and
		// declaration are both optional (SysML.xtext EnumeratedValue): `= 60.0;`.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseEnumBody(def)
			hasBody = true
		}
	default:
		members, hasBody = p.parseDefUsageBody()
	}

	def.Members = members
	def.HasBody = hasBody
	def.NodeSpan = p.spanFrom(start)
	return def
}

// isKerMLClassifierDefinitionKeyword identifies keyword forms using KerML's
// ClassifierDeclaration (and the shared TypeDeclaration) multiplicity slot.
func isKerMLClassifierDefinitionKeyword(keyword string) bool {
	switch keyword {
	case "type", "classifier", "class", "datatype", "struct", "assoc", "behavior",
		"function", "predicate", "interaction", "metaclass":
		return true
	default:
		return false
	}
}

// defBodyContext returns the body notation a definition of the given kind
// declares its members in.
func defBodyContext(kind ast.DefinitionKind) bodyContext {
	switch kind {
	case ast.DefInterface:
		return bodyInterface
	case ast.DefAction, ast.DefBehavior:
		return bodyAction
	case ast.DefState:
		return bodyState
	case ast.DefCalc:
		return bodyCalc
	case ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
		return bodyCase
	case ast.DefRequirement, ast.DefConcern, ast.DefViewpoint:
		return bodyRequirement
	case ast.DefView:
		return bodyViewDef
	}
	return bodyOther
}

// usageBodyContext returns the body notation a usage of the given kind declares
// its members in.
func usageBodyContext(kind ast.UsageKind) bodyContext {
	switch kind {
	case ast.UsageInterface:
		return bodyInterface
	case ast.UsageAction, ast.UsageTransition, ast.UsageStep, ast.UsageBehavior:
		return bodyAction
	case ast.UsageState:
		return bodyState
	case ast.UsageCalc:
		return bodyCalc
	case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
		return bodyCase
	case ast.UsageRequirement, ast.UsageConcern, ast.UsageViewpoint,
		ast.UsageFramedConcern, ast.UsageObjective, ast.UsageSatisfy:
		return bodyRequirement
	case ast.UsageView:
		return bodyView
	}
	return bodyOther
}

// bodyAdmitsMember reports whether the innermost body offers the member kw
// introduces; a word the file's language does not reserve is a name instead.
func (p *Parser) bodyAdmitsMember(kw string) bool {
	m, ok := ownedMembers[kw]
	if !ok || !p.reservedWord(kw) {
		return true
	}
	return slices.Contains(m.bodies, p.bodyContext())
}

// ownedMember describes a member notation that only certain body kinds offer.
type ownedMember struct {
	role   string // what the member declares, for the diagnostic
	owner  string // the kind of element whose body offers it
	bodies []bodyContext
}

var ownedMembers = map[string]ownedMember{
	"subject":     {"the subject of a requirement or case", "requirement or case", []bodyContext{bodyRequirement, bodyCase}},
	"actor":       {"an actor of a requirement or case", "requirement or case", []bodyContext{bodyRequirement, bodyCase}},
	"stakeholder": {"a stakeholder of a requirement", "requirement", []bodyContext{bodyRequirement}},
	"objective":   {"the objective of a case", "case", []bodyContext{bodyCase}},
	"entry":       {"the entry action of a state", "state", []bodyContext{bodyState}},
	"do":          {"the do action of a state", "state", []bodyContext{bodyState}},
	"exit":        {"the exit action of a state", "state", []bodyContext{bodyState}},
	"render":      {"the rendering of a view", "view", []bodyContext{bodyViewDef, bodyView}},
	// TransitionUsageMember is a StateBodyItem only (SysML.xtext); a transition between
	// action nodes is an extension the notation pass reports, so those bodies read it too.
	"transition": {"a transition between states", "state", []bodyContext{bodyState, bodyAction, bodyCalc, bodyCase}},
	// Expose is a ViewBodyItem only (SysML.xtext); in a view def body it is an
	// extension OpenSysML resolves, which the notation pass reports.
	"expose": {"what a view usage exposes", "view usage", []bodyContext{bodyView, bodyViewDef}},
}

// parseMisplacedStateSubaction reads an entry/do/exit member outside a state body
// into one ErrorNode; nil when the cursor is not on one. A state body reads them itself.
func (p *Parser) parseMisplacedStateSubaction(start int, trivia []ast.Trivia) ast.Node {
	tok := p.peek()
	if tok.Kind != lexer.Keyword || !isStateSubactionKeyword(tok.KeywordID) || p.bodyAdmitsMember(tok.KeywordID) {
		return nil
	}
	kind := stateSubactionKind(tok.KeywordID)
	en := p.misplacedMember(tok)
	p.advance()
	p.parseStateSubaction(start, kind)
	en.NodeSpan = p.spanFrom(start)
	en.SetLeadingTrivia(trivia)
	return en
}

// misplacedMember reports a member written outside the body kind that owns it and
// returns the ErrorNode standing in for it; the caller parses the member and spans the node.
func (p *Parser) misplacedMember(kw lexer.Token) *ast.ErrorNode {
	m := ownedMembers[kw.KeywordID]
	msg := fmt.Sprintf("'%s' declares %s and is only allowed in a %s body; move it into the %s it belongs to",
		kw.KeywordID, m.role, m.owner, m.owner)
	p.error(kw.Span, msg)
	return &ast.ErrorNode{Message: msg}
}

// isBehavioralKeyword checks if next token is a behavioral keyword
func (p *Parser) isBehavioralKeyword() bool {
	if !p.at(lexer.Keyword) {
		// `done;` and the other unreserved node words, in the node shape only.
		_, ok := p.atActionNodeWord()
		return ok
	}
	kw := p.peek().KeywordID
	switch kw {
	case "first", "fork", "join", "merge", "decide", "action", "then",
		"assign", "perform", "while", "loop", "if", "send", "terminate", "for",
		// `else <target>;` is a DefaultTargetSuccession member (SysML.xtext).
		"else":
		return true
	}
	return false
}

// isResultKeyword checks if next token is 'return'
func (p *Parser) isResultKeyword() bool {
	return p.at(lexer.Keyword) && p.peek().KeywordID == "return"
}

// stepNameKeyword is `do`, which names a step (`step do[1] subsets middle;` in
// StatePerformances.kerml) though it continues every other declaration.
const stepNameKeyword = "do"

// metadataStopKeyword is `about`, which ends a metadata declaration (SysML.xtext
// MetadataUsage), so an unnamed `metadata : M about x;` is not named "about".
const metadataStopKeyword = "about"

// parseUsageIdentification parses the identification of a usage of kind: `do`
// names a step and nothing else, `about` names nothing in a metadata usage.
func (p *Parser) parseUsageIdentification(kind ast.UsageKind) ast.Identification {
	if kind == ast.UsageStep && p.atKeyword(stepNameKeyword) {
		tok := p.advance()
		return ast.Identification{
			Name: tok.KeywordID,
		}
	}
	if kind == ast.UsageMetadata {
		return p.parseIdentificationStopping(metadataStopKeyword)
	}
	return p.parseIdentification()
}

// atGuardedSuccession reports whether the succession being read states a guard
// between its ends (`succession [name] first a if g then b`), which is the
// GuardedSuccession production and so a transition, not a connector.
func (p *Parser) atGuardedSuccession() bool {
	i := 0
	if p.peek().Kind == lexer.Identifier || p.peek().Kind == lexer.UnrestrictedName {
		i = 1
	}
	if !p.peekIsKeyword(i, "first") {
		return false
	}
	for depth := 0; i < 60; i++ {
		tok := p.peekN(i)
		switch tok.Kind {
		case lexer.EOF, lexer.Semicolon, lexer.LBrace, lexer.RBrace:
			return false
		case lexer.LParen, lexer.LBracket:
			depth++
		case lexer.RParen, lexer.RBracket:
			depth--
		case lexer.Keyword:
			if depth == 0 && tok.KeywordID == "then" {
				return false
			}
			if depth == 0 && tok.KeywordID == "if" {
				return true
			}
		}
	}
	return false
}

// parseUsage parses a usage. keyword is the kind keyword as consumed from the
// token stream, kept for the same reason as in parseDefinition.
func (p *Parser) parseUsage(start int, kind ast.UsageKind, keyword string, mods featureMods, isAll bool) *ast.Usage {
	u := &ast.Usage{
		Kind:          kind,
		Keyword:       keyword,
		PrefixKeyword: mods.prefixKeyword,
		IsNegated:     mods.isNegated,
		IsAbstract:    mods.isAbstract,
		IsVariation:   mods.isVariation,
		IsVariant:     mods.isVariant,
		IsReference:   mods.isReference,
		IsVariable:    mods.isVariable,
		IsAll:         isAll,
		IsEnd:         mods.isEnd,
		IsChain:       mods.isChain,
		IsConstant:    mods.isConstant,
		IsEvent:       mods.isEvent,
		IsIndividual:  mods.isIndividual,
		Portion:       mods.portion,
		Visibility:    mods.visibility,
		Direction:     mods.direction,
		IsComposite:   mods.isComposite,
		IsPortion:     mods.isPortion,
		IsDerived:     mods.isDerived,
		IsOrdered:     mods.isOrdered,
		IsNonunique:   mods.isNonunique,
	}

	u.CrossFeature = mods.cross

	// Handle UsageSatisfy special syntax:
	// Full form: satisfy [requirement] <name> by <name> { body }
	// Short form: satisfy/verify <name>;
	if kind == ast.UsageSatisfy {
		// The declaration form is a full UsageDeclaration, so it may state a
		// type as well as a name (`satisfy requirement r : Req1 by v;`), while
		// the reference form names an existing requirement usage and declares
		// nothing.
		if p.acceptKeyword("requirement") {
			u.DeclaresRequirement = true
			// The UsageDeclaration is optional, and `by` introduces the subject
			// rather than naming the satisfaction (`satisfy requirement by v;`).
			if !p.atKeyword("by") {
				u.Ident = p.parseUsageIdentification(kind)
			}
			declRels := p.parseRelationships(true)
			u.Relationships = append(u.Relationships, declRels...)
		} else if reqName := p.parseChainedName(); reqName != nil {
			// SysML.xtext:2119's ReferenceSubsetting reaches a nested feature through a '.'
			// chain (KerML.xtext:699); kept a plain subsetting until its readers migrate.
			u.Relationships = append(u.Relationships, &ast.Relationship{
				Kind:   ast.RelSubsets,
				Target: reqName,
			})
			// The reference form takes specializations of its own:
			// `verify r :>> massRequirement;` (SysML.xtext:2272,
			// RequirementVerificationUsage's `FeatureSpecialization*`).
			u.Relationships = append(u.Relationships, p.parseRelationships(true)...)
		}

		// ValuePart? — a satisfy usage may bind a value like any usage.
		p.parseUsageValue(u)

		// Check for optional "by" clause. Per SatisfyRequirementUsage the `by`
		// operand names the subject of the satisfaction, never the usage itself,
		// so it is always recorded as a subject relationship.
		if p.acceptKeyword("by") {
			if subjTarget := p.parseRelationshipTarget(); subjTarget != nil {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelSubject,
					Target: subjTarget,
				})
			}
		}

		// A satisfy usage is a requirement usage (SysML v2 §7.20), so its body
		// carries requirement members.
		if p.accept2(lexer.Semicolon) {
			u.HasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			leave := p.pushBodyContext(usageBodyContext(kind))
			u.Members = p.parseRequirementBody()
			leave()
			u.HasBody = true
		}
		u.NodeSpan = p.spanFrom(start)
		return u
	}

	// A binding connector states an optional declaration and two connector ends,
	// each read by parseConnectorEnd like a succession's or a connector's.
	if kind == ast.UsageBinding {
		p.parseBindingDeclaration(u, keyword)
		leave := p.pushBodyContext(usageBodyContext(kind))
		members, hasBody := p.parseDefUsageBody()
		leave()
		u.Members = members
		u.HasBody = hasBody
		u.NodeSpan = p.spanFrom(start)
		return u
	}

	// Handle succession/connector/flow with multiplicity before name/first keyword
	// Pattern: `succession [mult] name first [mult] x then [mult] y`
	// Pattern: `succession [mult] first [mult] x then [mult] y` (anonymous)
	// Pattern: `connector [mult] name from [mult] x to [mult] y`
	// Check for anonymous succession BEFORE consuming multiplicity
	var earlyMultiplicity *ast.Multiplicity
	var isAnonymous bool
	switch kind {
	case ast.UsageSuccession:
		// A name may only precede `first` (KerML.xtext:891), so `first` or an end
		// followed by `then` — past any leading multiplicity — means there is none.
		isAnonymous = p.peekIsKeyword(p.pastBracketed(0), "first") || p.atConnectorBinaryEnds("then")
	case ast.UsageConnector:
		isAnonymous = p.atConnectorBinaryEnds("to")
	}
	anonymousConnector := kind == ast.UsageConnector && isAnonymous
	// `succession [mult] a then b` declares no connector, so the multiplicity is the
	// first end's (KerML.xtext:891); `succession [mult] first a then b` keeps its own.
	leadingEndMultiplicity := kind == ast.UsageSuccession && p.at(lexer.LBracket) && p.atConnectorBinaryEnds("then")
	if (kind == ast.UsageSuccession || kind == ast.UsageConnector || kind == ast.UsageFlow) && !anonymousConnector && !leadingEndMultiplicity && p.at(lexer.LBracket) {
		earlyMultiplicity = p.parseMultiplicity()
	}

	// A declaration stating a specialization before its name has no name to state
	// (SysML.xtext FeatureDeclaration): `part redefines wheel` is the same unnamed
	// usage as `part :>> wheel`. The name it answers to is its redefinition's, and
	// the symbol layer derives that (KerML 7.3.4.5, symbols.effectiveIdent).
	preRels := p.parsePreNameRelationships(true)
	// A bare flow shorthand `flow x to y` and anonymous succession `succession x then y` have no declaration name
	// Anonymous connector starts with 'from' keyword (e.g., `connector : X from y to z`)
	// A connection or interface stating ends where its name would go declares
	// nothing of its own either: `interface b1.p to b2.p`.
	skipIdentification := (kind == ast.UsageFlow && (p.atFlowShorthand() || p.atKeyword("from"))) ||
		(kind == ast.UsageSuccession && isAnonymous) ||
		(kind == ast.UsageAllocation && p.atAllocateShorthand()) ||
		(kind == ast.UsageConnector && p.atKeyword("from")) || anonymousConnector ||
		((kind == ast.UsageConnection || kind == ast.UsageInterface) &&
			(keyword == "connect" || p.atConnectorShorthandEnds()))
	atGlobalName := p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon
	if kind == ast.UsageMetadata && !skipIdentification && (atGlobalName || p.atName() && !p.atMetadataIdentification()) {
		// `metadata M about x;` states the typing M and no name of its own
		// (SysML.xtext MetadataUsageDeclaration).
		skipIdentification = true
		if metaType := p.parseQualifiedName(); metaType != nil {
			preRels = append(preRels, &ast.Relationship{Kind: ast.RelTyping, Target: metaType})
		}
	}
	if !skipIdentification {
		u.Ident = p.parseUsageIdentification(kind)
	}

	// Parse post-identification relationships (e.g., : Type)
	postIdRels := p.parseRelationships(true)
	u.Relationships = append(preRels, postIdRels...)

	// For anonymous succession/flow, skip multiplicity parsing - it belongs to connector ends
	// UNLESS earlyMultiplicity was already parsed (e.g., `succession [mult] first ...`)
	// In the shorthand forms a multiplicity here is the first end's.
	// A succession stating a specialization instead of a name still has a
	// declaration of its own, so the multiplicity is the declaration's
	// (KerML.xtext:891, SuccessionDeclaration's FeatureDeclaration alternative).
	declared := u.Ident.Name != "" || u.Ident.ShortName != "" || len(u.Relationships) > 0
	skipMultiplicity := ((kind == ast.UsageSuccession || kind == ast.UsageFlow) && !declared && earlyMultiplicity == nil) ||
		((kind == ast.UsageConnection || kind == ast.UsageInterface) && skipIdentification) ||
		anonymousConnector
	if !skipMultiplicity {
		if earlyMultiplicity != nil {
			u.Multiplicity = earlyMultiplicity
		} else {
			u.Multiplicity = p.parseMultiplicity()
		}
	}

	p.parseSpecializationsAfterMultiplicity(u)
	p.checkTypeDeclarationSpecialization(u, keyword)

	p.parseUsageValue(u)
	p.parseTierBEnds(u, kind)

	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
	defer p.pushBodyContext(usageBodyContext(kind))()
	switch kind {
	case ast.UsageAction:
		// Action usage bodies: mixed (declarations + behavioral statements)
		// Support THREE forms:
		// 1. action name; (no body)
		// 2. action name { in item x; action nested {...}; first ...; } (braced mixed body)
		// 3. action name \n statements (inline behavioral body without braces)
		if p.atTransitionEffectStatement(start) && p.atEffectEnd() {
			// 4. action written as a transition's effect: no body of its own, and
			// the transition owns the ';' (`do action alarm : Alarm;`).
			hasBody = false
		} else if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.at(lexer.LBrace) {
			// Braced body - use mixed parser (handles declarations + behavioral)
			_, ok := p.expect(lexer.LBrace, "expected '{'")
			if ok {
				members = p.parseActionBodyMixed()
				hasBody = true
			}
		} else if p.isBehavioralKeyword() {
			// Inline behavioral body without braces: action name\n assign ...;
			// The body is a single statement plus any 'then'-chained continuations;
			// a following statement that is not chained belongs to the enclosing body.
			// EXCEPT: a 'then' chaining to a declaration is namespace-level
			// succession, not behavioral succession - stop parsing body
			//
			// An unbraced body written as a transition's effect ends where the
			// transition does, so the transition owns its statement's ';'.
			inEffect := p.atTransitionEffectStatement(start)
			savedEffectStmtStart := p.effectStmtStart
			for !p.atEOF() && !p.atNamespaceSuccession() {
				if inEffect {
					p.effectStmtStart = p.peek().Span.Offset
				}
				members = append(members, p.parseActionMember())
				// Only an inline statement continues the body; a succession names
				// members of the enclosing body.
				if !p.atKeyword("then") || !startsInlineSuccessionStatement(p.peekN(1)) {
					break
				}
			}
			p.effectStmtStart = savedEffectStmtStart
			hasBody = true
			u.IsActionNode = true
		} else {
			p.expectBodyOrEnd("action declaration")
		}
	case ast.UsageCalc:
		// Calculation usage bodies: mixed (parameters + return statements)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseCalcBody()
			hasBody = true
		}
	case ast.UsageBool:
		// Bool usage bodies: can be calc-style (with return) OR constraint-style (single expression)
		// Lookahead: if body starts with 'in' or 'return' → calcBody, otherwise → constraint-style expression
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			// Peek at first token in body
			firstTok := p.peek()
			if firstTok.Kind == lexer.Keyword && (firstTok.KeywordID == "in" || firstTok.KeywordID == "return") {
				// Structured calc body with parameters/return
				members = p.parseCalcBody()
			} else {
				// Single expression body (constraint-style)
				members = p.parseConstraintBody()
			}
			hasBody = true
		}
	case ast.UsageConstraint, ast.UsagePredicate:
		// Constraint bodies: { assert/assume expr; ... }
		// Bool and predicate usages also use constraint-style bodies with expressions
		// Special case: if body starts with 'in' or 'return' keyword, parse as calc body (structured parameters)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.at(lexer.LBrace) {
			// Check if this is a typed predicate with input/return parameters
			// Peek ahead past the '{' and any 'doc' keywords to see if body has 'in' or 'return'
			hasCalcBody := false
			for i := 1; i < 10; i++ { // Look ahead up to 10 tokens
				tok := p.peekN(i)
				if tok.KeywordID == "doc" {
					continue // Skip doc keywords
				}
				if tok.KeywordID == "in" || tok.KeywordID == "return" {
					hasCalcBody = true
				}
				break // Stop at first non-doc keyword
			}

			p.advance() // {
			if hasCalcBody {
				// Parse as calc body with structured parameters
				members = p.parseConstraintCalcBody()
			} else {
				members = p.parseConstraintBody()
			}
			hasBody = true
		} else {
			p.expectBodyOrEnd("declaration")
		}
	case ast.UsageRequirement, ast.UsageConcern, ast.UsageViewpoint, ast.UsageFramedConcern, ast.UsageObjective:
		// Requirement bodies: { subject/assume/require/actor ... }. A concern
		// usage is a requirement usage and a viewpoint usage a concern usage
		// (SysML v2 §7.19), so they carry the same members; a framed concern is
		// a concern usage and its declaration form ends in a RequirementBody
		// (SysML.xtext FramedConcernUsage). An objective is a requirement usage
		// too (SysML.xtext ObjectiveRequirementUsage).
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseRequirementBody()
			hasBody = true
		}
	case ast.UsageExpr, ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
		// Expression bodies end in the expression they compute (KerML.xtext Expression);
		// case bodies may end in a ResultExpressionMember (SysML.xtext CalculationBodyPart).
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseCaseBody()
			hasBody = true
		}
	case ast.UsageState:
		// State usage bodies: always use parseStateBody (it handles both state-specific and generic members)
		// `parallel` marks the substates orthogonal, and only a body may follow it
		// (SysML.xtext StateUsageBody).
		if p.atKeyword("parallel") {
			u.IsParallel = true
			p.advance()
			if _, ok := p.expect(lexer.LBrace, "expected '{' after 'parallel'"); ok {
				members = p.parseStateBody()
				hasBody = true
			}
			break
		}
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.expectBodyOrEnd("declaration") {
			members = p.parseStateBody()
			hasBody = true
		}
	default:
		members, hasBody = p.parseDefUsageBody()
	}

	// A flow's `of name : Type` clause contributes a member before the body is
	// parsed, so the body members are appended rather than replacing it.
	u.Members = append(u.Members, members...)
	u.HasBody = hasBody
	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseSuccessionAsUsage parses a succession stated without the `succession`
// keyword (SysML v2 8.2.2.13.3): `first a then b;`.
func (p *Parser) parseSuccessionAsUsage(start int) ast.Node {
	u := &ast.Usage{Kind: ast.UsageSuccession}
	p.parseConnectorEnds(u, "")
	u.Members, u.HasBody = p.parseGenericBody()
	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseGenericBody parses the body of a member whose kind offers no members of
// its own (SysML.xtext UsageBody), so it does not inherit the body around it.
func (p *Parser) parseGenericBody() (members []ast.Node, hasBody bool) {
	defer p.pushBodyContext(bodyOther)()
	return p.parseDefUsageBody()
}

// parseDefUsageBody parses a definition/usage body: `;` (no body) or
// `{ member* }`. Body members may be nested def/usage declarations or ordinary
// namespace members, each carrying optional visibility.
func (p *Parser) parseDefUsageBody() (members []ast.Node, hasBody bool) {
	if p.accept2(lexer.Semicolon) || !p.expectBodyOrEnd("declaration") {
		return nil, false
	}
	return p.parseDefUsageBodyMembers(), true
}

// parseDefUsageBodyMembers parses the members of a definition/usage body up to
// and including its closing brace, with the opening brace already consumed.
func (p *Parser) parseDefUsageBodyMembers() []ast.Node {
	body := p.newBodyBuilder()
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		// A member-attached `then` sequences the members either side of it, so
		// the keyword is taken here and the member it prefixes read next time
		// round (see succession.go).
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		// `succession first a then b;` names two members of this body, which is
		// the form used when writing a converted model back. DefinitionBodyItem
		// (SysML.xtext:516-524) has no TargetSuccessionMember, so it takes no body.
		if p.atKeyword("then") {
			body.add(p.parseSuccessionEdge(p.advance(), false))
			continue
		}
		body.add(p.parseBodyMember())
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, msgExpectedBodyClose)
	return body.finish()
}

// parseCaseBody parses a case body's members, which may end in a result
// expression member (SysML.xtext CalculationBodyPart): `vehicle.mass`.
func (p *Parser) parseCaseBody() []ast.Node {
	body := p.newBodyBuilder()
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		if p.atKeyword("then") {
			body.add(p.parseSuccessionEdge(p.advance(), true))
			continue
		}
		// A case body carries an action body's items — control nodes and behavioural
		// statements among them (SysML.xtext:2191 CaseBodyItem → … → ActionBodyItem).
		if p.atActionNodeMember() || p.atCalcStatement() {
			body.add(p.parseActionMember())
			continue
		}
		if p.atResultExpression() {
			body.add(p.ParseExpression())
			continue
		}
		body.add(p.parseBodyMember())
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, msgExpectedBodyClose)
	return body.finish()
}

// atResultExpression reports whether the current token begins a bare result
// expression rather than a member declaration. Keyword-led members belong to
// the member parser, and a name whose next token continues a declaration is a
// declaration.
func (p *Parser) atResultExpression() bool {
	t := p.peek()
	if p.atPrefixOperator() {
		return true
	}
	if (t.Kind == lexer.Keyword && !exprStartKeywords[t.KeywordID]) || t.Kind == lexer.LBrace {
		return false
	}
	if !p.atExprStart() || p.atVarDeclaration() {
		return false
	}
	if !p.atName() {
		return true
	}
	next := p.peekN(1)
	if next.Kind == lexer.Keyword && wordBinaryOpKeywords[next.KeywordID] {
		return true
	}
	isDecl := next.Kind == lexer.Colon || next.Kind == lexer.Semicolon ||
		next.Kind == lexer.Keyword || next.Kind == lexer.LBracket ||
		next.Kind == lexer.LBrace ||
		beginsDeclarationTail(next, p.peekN(2))
	return !isDecl
}

// wordBinaryOpKeywords are the keyword binary operators (binaryOpFor); a name
// followed by one continues an expression, not a declaration.
var wordBinaryOpKeywords = map[string]bool{
	"implies": true,
	"or":      true,
	"xor":     true,
	"and":     true,
	"hastype": true,
	"istype":  true,
	"as":      true,
	"meta":    true,
}

// parseEnumBody parses an enumeration body, whose enumerated values may be
// anonymous (SysML.xtext EnumeratedValue): `= 60.0;`.
func (p *Parser) parseEnumBody(def *ast.Definition) []ast.Node {
	body := p.newBodyBuilder()
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		// `= 60.0;`, `uncl : Level = 0;`, `<u> uncl;`, `: Level;` and `#M a;` are
		// enumerated values (SysML.xtext EnumeratedValue), not keyword-less attributes.
		if p.atEnumeratedValueDeclaration() {
			start := p.peek().Span.Offset
			trivia := p.takeTrivia()
			vis := p.parseVisibility()
			prefixes := p.parsePrefixMetadata()
			keyword := ""
			if p.acceptKeyword("enum") {
				keyword = "enum"
			}
			var u *ast.Usage
			if p.at(lexer.Eq) || p.at(lexer.ColonEq) {
				u = &ast.Usage{Kind: ast.UsageEnumeration, Keyword: keyword, Visibility: vis}
				p.parseUsageValue(u)
				p.expectSemicolon("enumerated value")
				u.NodeSpan = p.spanFrom(start)
			} else {
				u = p.parseUsage(start, ast.UsageEnumeration, keyword, featureMods{visibility: vis}, false)
			}
			u.Prefixes = prefixes
			m := &ast.Membership{Visibility: vis, Member: u}
			m.NodeSpan = p.spanFrom(start)
			m.SetLeadingTrivia(trivia)
			body.add(m)
			continue
		}
		outer := p.inEnumBody
		p.inEnumBody = true
		member := p.parseBodyMember()
		p.inEnumBody = outer
		p.checkEnumerationMember(def, member)
		body.add(member)
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, msgExpectedBodyClose)
	return body.finish()
}

// checkEnumerationMember warns on a member EnumerationBody (SysML.xtext) does not
// admit; the member still reads, so the analysis escalates it without gating tiers.
func (p *Parser) checkEnumerationMember(def *ast.Definition, member ast.Node) {
	node := memberNode(member)
	switch node.(type) {
	case *ast.Usage, // a non-enumerated usage is the variation-membership constraint's finding
		*ast.Comment, *ast.Documentation, *ast.TextualRepresentation, *ast.PrefixMetadata, *ast.ErrorNode:
		return
	}
	owner, label := enumerationName(def), enumerationMemberLabel(node)
	p.warn(member.Span(), fmt.Sprintf("enumeration definition %s cannot own %s: an enumeration body holds only enumerated values and annotations (comments, documentation, textual representations, metadata); move %s out of %s",
		owner, label, label, owner), codeEnumerationBodyMember)
}

// enumerationName names def for a diagnostic.
func enumerationName(def *ast.Definition) string {
	if def.Ident.Name != "" {
		return "`" + def.Ident.Name + "`"
	}
	if def.Ident.ShortName != "" {
		return "`" + def.Ident.ShortName + "`"
	}
	return "this enumeration"
}

// enumerationMemberLabel describes an illegal enumeration member for a diagnostic.
func enumerationMemberLabel(node ast.Node) string {
	switch n := node.(type) {
	case *ast.Definition:
		kw := n.Keyword
		if kw == "" {
			kw = n.Kind.String()
		}
		if n.HasDefKeyword {
			kw += " def"
		}
		return labelNamed(kw, n.Ident)
	case *ast.Package:
		return labelNamed("package", n.Ident)
	case *ast.Namespace:
		return labelNamed("namespace", n.Ident)
	case *ast.Alias:
		return labelNamed("alias", n.Ident)
	case *ast.Import:
		if n.IsExpose {
			return "this expose"
		}
		return "this import"
	case *ast.Dependency:
		return "this dependency"
	case *ast.FilterMember:
		return "this filter"
	}
	return "this member"
}

// labelNamed is "<kind> `<name>`", or "this <kind>" when unnamed.
func labelNamed(kind string, id ast.Identification) string {
	if id.Name != "" {
		return kind + " `" + id.Name + "`"
	}
	if id.ShortName != "" {
		return kind + " `" + id.ShortName + "`"
	}
	return "this " + kind
}

func (p *Parser) parseTypeFeatureMember(start int, vis ast.Visibility, trivia []ast.Trivia) ast.Node {
	p.advance() // consume 'member'
	inner := p.parseDeclaration(start)
	if inner == nil {
		en := p.errorNodeSkip(start, "expected a body member after 'member'")
		en.SetLeadingTrivia(trivia)
		return en
	}
	m := &ast.Membership{
		Visibility:    vis,
		IsTypeFeature: true,
		Member:        inner,
	}
	m.NodeBase.NodeSpan = p.spanFrom(start)
	m.SetLeadingTrivia(trivia)
	return m
}

// parseBodyMember parses one body member: an optional visibility prefix
// followed by a declaration (which may be a nested def/usage). Import/Alias
// carry their own visibility and are returned directly; other declarations are
// wrapped in a Membership. Mirrors parseMember.
func (p *Parser) parseBodyMember() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()
	vis := p.parseVisibility()
	// The enumeration context names this member only; bodies nested in it hold
	// ordinary members, so it is consumed here rather than left set while they parse.
	enumValue := p.inEnumBody
	p.inEnumBody = false

	// A member-attached `then` is taken by the body loop, which owns the member
	// list the succession it desugars to is synthesised into (see
	// succession.go). One reaching here belongs to a body that keeps no such
	// list, so there is nowhere to put the succession: report it rather than
	// parse the member with the keyword dropped, which would silently sequence
	// nothing.
	if p.atKeyword("then") {
		tok := p.advance()
		p.error(tok.Span, "`then` cannot prefix a member here: a succession sequences two members of a definition, usage, action, state, calculation or requirement body")
		p.inEnumBody = enumValue
		return p.parseBodyMember()
	}

	if en := p.parseMisplacedStateSubaction(start, trivia); en != nil {
		return en
	}

	// Check for `#MetadataType` prefix (user-defined keyword)
	// Parse prefixes and then parse def/usage declaration
	if p.at(lexer.Hash) {
		// Delegate to parseDefUsage which handles prefixes
		inner := p.parseDefUsage(start)
		if inner == nil {
			return nil
		}
		// Wrap in membership if not already wrapped
		if m, ok := inner.(*ast.Membership); ok {
			m.SetLeadingTrivia(trivia)
			return m
		}
		m := &ast.Membership{Visibility: vis, Member: inner}
		m.NodeSpan = p.spanFrom(start)
		m.SetLeadingTrivia(trivia)
		return m
	}

	// A metadata usage: `@Type;` or `@Type { prop = value; }`.
	if p.at(lexer.At) {
		pm := p.parseMetadataUsage(start)
		if pm == nil {
			return nil
		}
		pm.SetLeadingTrivia(trivia)
		m := &ast.Membership{Visibility: vis, Member: pm}
		m.NodeSpan = p.spanFrom(start)
		m.SetLeadingTrivia(trivia)
		return m
	}

	// A direction prefixes the feature it applies to, so a member that is only a
	// direction is that feature missing (SysML.xtext FeatureDirection).
	if dir := p.peek(); p.atDirectionKeyword() && p.peekN(1).Kind == lexer.Semicolon {
		p.advance() // consume the direction
		p.error(p.peek().Span, "expected a feature after '"+dir.KeywordID+"': write `"+dir.KeywordID+" <name> : <Type>`")
		p.advance() // consume ';'
		return nil
	}

	if p.atKeyword("import") {
		imp := p.parseImport(start, vis)
		imp.SetLeadingTrivia(trivia)
		return imp
	}
	if p.atKeyword("alias") {
		al := p.parseAlias(start, vis)
		al.SetLeadingTrivia(trivia)
		return al
	}
	if p.atTextualRepresentationStart() {
		inner := p.parseTextualRepresentation(start)
		m := &ast.Membership{Visibility: vis, Member: inner}
		m.NodeBase.NodeSpan = p.spanFrom(start)
		m.SetLeadingTrivia(trivia)
		return m
	}
	if p.atKeyword("member") {
		return p.parseTypeFeatureMember(start, vis, trivia)
	}

	// Check for timeslice usage keyword
	// Creates occurrence usage (temporal slice)
	if p.atKeyword("timeslice") {
		inner := p.parseDefUsage(start)
		if inner != nil {
			m := &ast.Membership{
				Visibility: vis,
				Member:     inner,
			}
			m.NodeSpan = p.spanFrom(start)
			m.SetLeadingTrivia(trivia)
			return m
		}
	}

	// Check for snapshot usage keyword
	// Creates occurrence usage (temporal instant)
	if p.atKeyword("snapshot") {
		inner := p.parseDefUsage(start)
		if inner != nil {
			m := &ast.Membership{
				Visibility: vis,
				Member:     inner,
			}
			m.NodeSpan = p.spanFrom(start)
			m.SetLeadingTrivia(trivia)
			return m
		}
	}

	// Check for behavioral statements in structural contexts (occurrence/part with temporal ordering)
	// These include: first/then succession edges for snapshot ordering
	if p.atKeyword("first") {
		// A structural body has no token flow, so `first a then b;` is a
		// SuccessionAsUsage over its members (SysML v2 8.2.2.13.3), never an
		// InitialNodeMember — which would declare a member shadowing end `a`.
		// The one-ended `first a;` stays an initial node (extension notation).
		if p.atChainedFirstSuccession() ||
			(!p.bodyContext().carriesActions() && p.atTwoEndedFirst()) {
			return p.parseSuccessionAsUsage(start)
		}
		firstTok := p.advance()
		return p.parseInitialNode(firstTok)
	}

	// Check for return statement (result member)
	// Can appear in calc body, constraint body, or requirement body
	if p.isResultKeyword() {
		return p.parseResultMember()
	}

	// A KerML relationship written keyword-first: `subset X subsets Y;`,
	// `disjoint X from Y;`.
	if p.atRelationshipMember() {
		return p.parseRelationshipMember(start, vis, trivia)
	}

	// Check for expose statement: expose <path>[::*|::**][filter];
	// Per SysML v2 8.3.26.2, an Expose is an Import: MembershipExpose
	// specializes MembershipImport and NamespaceExpose specializes
	// NamespaceImport, so the wildcard tail selects the import kind exactly as
	// it does for `import`. An Expose always imports all elements regardless of
	// visibility (isImportAll = true) and always has protected visibility.
	if p.atKeyword("expose") {
		// Only a view usage body offers Expose (SysML.xtext ViewBodyItem); the
		// member is still read so recovery resumes after its `;`.
		var misplaced *ast.ErrorNode
		if !p.bodyAdmitsMember("expose") {
			misplaced = p.misplacedMember(p.peek())
		}
		p.advance() // consume 'expose'

		path := p.parseQualifiedName()
		if path == nil {
			p.error(p.peek().Span, "expected namespace path after 'expose'")
			return &ast.ErrorNode{Message: "expected namespace path"}
		}

		imp := &ast.Import{
			Visibility: ast.VisibilityProtected,
			IsAll:      true,
			Kind:       ast.ImportMembership,
			Imported:   path,
			IsExpose:   true,
		}
		p.parseImportTail(imp)
		imp.NodeBase.NodeSpan = p.spanFrom(start)
		imp.SetLeadingTrivia(trivia)

		p.expectSemicolon("expose statement")

		if misplaced != nil {
			misplaced.NodeSpan = imp.NodeSpan
			misplaced.SetLeadingTrivia(trivia)
			return misplaced
		}
		return imp
	}

	// An accept node (SysML.xtext `AcceptNode`):
	//
	//	action <name>? accept <payload> ('via' <port>)? (';' | '{' … '}')
	//
	// The payload says what the action waits for — a type (`accept scene :
	// Scene`), an event feature (`accept :> shutDown`) or a trigger expression
	// (`accept when x > 1`) — and is parsed by the one payload parser triggers
	// also use, so every spelling reaches lowering the same way.
	if p.atAcceptNode() {
		return p.parseAcceptNode(start, vis, trivia)
	}

	// A transition usage stating its ends (SysML.xtext `TransitionUsage`):
	//
	//	transition <name>? first <source> … then <target>;
	//	transition <name>? <source> to <target> …;
	//
	// One parser reads every transition, wherever it is written, so a named
	// transition carries the same trigger, guard and effect a nameless one does
	// and lowering sees one representation. A `transition` declaring no ends
	// (`transition t : Signalling;`) is an ordinary usage and parsed below.
	if p.atKeyword("transition") && p.atTransitionEnds() {
		// Only a state body offers TransitionUsageMember (SysML.xtext StateBodyItem).
		var misplaced *ast.ErrorNode
		if !p.bodyAdmitsMember("transition") {
			misplaced = p.misplacedMember(p.peek())
		}
		p.advance() // consume 'transition'
		node := p.parseTransitionMember(start)
		if misplaced != nil {
			misplaced.NodeSpan = node.Span()
			misplaced.SetLeadingTrivia(trivia)
			return misplaced
		}
		if tr, ok := node.(interface{ SetLeadingTrivia([]ast.Trivia) }); ok {
			tr.SetLeadingTrivia(trivia)
		}
		m := &ast.Membership{Visibility: vis, Member: node}
		m.NodeSpan = node.Span()
		m.SetLeadingTrivia(trivia)
		return m
	}

	// Check for anonymous feature pattern: [modifiers] [name] : Type OR [modifiers] :>> relationships
	// Examples: private thisClock : Clock :>> self; or ref stateSpace: StateSpace; or ref :>> x
	// This handles features with visibility but no usage kind keyword
	nextKind := p.peekN(1).Kind

	// Check for (visibility OR modifier) + (name + colon OR relationship) pattern
	hasVisibility := vis != ast.VisibilityDefault
	hasModifier := p.atKeyword("ref") || p.atKeyword("readonly") || p.atKeyword("derived") || p.atKeyword("composite") || p.atKeyword("portion") || p.atKeyword("end")

	if hasVisibility || hasModifier {
		mods := p.parseFeatureModifiers()
		// Merge visibility into mods if it was parsed earlier
		if hasVisibility {
			mods.visibility = vis
		}

		// An `end` with no declaration at all is an interface body's default
		// end (SysML v2 8.2.2.14 DefaultInterfaceEnd, `isEnd ?= 'end' Usage`
		// over an optional UsageDeclaration).
		if mods.isEnd && (p.at(lexer.Semicolon) || p.at(lexer.LBrace)) {
			return p.parseAnonymousEnd(start, trivia, vis, mods)
		}

		// Special case: end shortname [mult] feature name pattern
		// Example: end self2 [1] feature sameThing: Anything
		// Also: end [1] feature transferSource (no short name)
		// Also: end ref source; (no definition keyword, just anonymous feature)
		// This declares a feature with 'end' modifier, optional short name, and multiplicity
		// Prefix metadata may stand where the kind keyword would, after the
		// modifiers (SysML.xtext ExtendedUsage): `end #original r1 : Req1;`.
		if p.at(lexer.Hash) {
			if inner := p.parseDefUsage(start); inner != nil {
				applyFeatureMods(inner, mods)
				m := &ast.Membership{Visibility: vis, Member: inner}
				m.NodeSpan = p.spanFrom(start)
				m.SetLeadingTrivia(trivia)
				return m
			}
		}

		// The cross feature was read with the modifiers, so only the end's own
		// declaration remains; `end ref name;` is the anonymous form handled below.
		if mods.isEnd && p.isKindKeyword(p.peek()) {
			decl := p.parseDeclaration(start)
			if u, ok := decl.(*ast.Usage); ok {
				applyFeatureMods(u, mods)
				u.IsEnd = true
				u.Visibility = mods.visibility
			}
			mem := &ast.Membership{Visibility: vis, Member: decl}
			mem.NodeSpan = p.spanFrom(start)
			mem.SetLeadingTrivia(trivia)
			return mem
		}

		// A feature modifier can precede the kind keyword: `ref part a : V`,
		// `composite item i`, `derived attribute c`. The declaration is parsed
		// as usual and the modifiers already consumed are applied to it.
		if p.isKindKeyword(p.peek()) || p.atKindPrefix() {
			// A keyword that only qualifies the kind after it is consumed first:
			// `derived var feature x` declares a feature, not a `var`.
			for p.atKindPrefix() && !p.isKindKeyword(p.peek()) {
				// A prefix saying what the declaration is for is part of it, whether
				// or not a modifier was written before it (`derived var feature x`).
				if w := p.kindPrefixWord(); kindPrefixKeywords[w] {
					mods.prefixKeyword = w
				}
				if p.kindPrefixWord() == varPrefixWord {
					mods.isVariable = true
				}
				p.advance()
			}
			decl := p.parseDeclaration(start)
			if decl == nil {
				en := p.errorNodeSkip(start, "expected a declaration after a feature modifier")
				en.SetLeadingTrivia(trivia)
				return en
			}
			applyFeatureMods(decl, mods)
			mem := &ast.Membership{Visibility: mods.visibility, Member: decl}
			mem.NodeSpan = p.spanFrom(start)
			mem.SetLeadingTrivia(trivia)
			return mem
		}

		// Check for name + colon (typed) OR direct relationship (anonymous) OR name + relationship OR name + semicolon OR name + multiplicity
		hasNameAndType := p.atName() && p.peekN(1).Kind == lexer.Colon
		hasRelationship := p.at(lexer.ColonGt) || p.at(lexer.ColonGtGt) || p.at(lexer.ColonColonGt) || p.atRelationshipKeyword()
		// Either spelling of a specialization, or a value, continues the
		// declaration: `ref x :> y`, `ref x subsets y`, `ref x = 5`, `ref x default = 5`.
		hasNameAndRelationship := p.atName() && beginsDeclarationTail(p.peekN(1), p.peekN(2))
		hasNameOnly := p.atName() && (p.peekN(1).Kind == lexer.Semicolon || p.peekN(1).Kind == lexer.RBrace)
		hasNameAndBody := p.atName() && p.peekN(1).Kind == lexer.LBrace
		hasNameAndMult := p.atName() && p.peekN(1).Kind == lexer.LBracket // name with multiplicity (e.g., ref payload [0..*])
		// `end [1] : A;` — an unnamed feature declaring only its type.
		hasTypeOnly := p.at(lexer.Colon)

		if hasNameAndType || hasTypeOnly || hasRelationship || hasNameAndRelationship || hasNameOnly || hasNameAndBody || hasNameAndMult {
			var id ast.Identification

			// Parse optional name
			if hasNameAndType || hasNameAndRelationship || hasNameOnly || hasNameAndBody || hasNameAndMult {
				if seg, ok := p.parseNameSegmentRelaxed(); ok {
					id.Name = seg.Text
					id.NameSpan = seg.Span
				}
				if hasNameAndType {
					p.advance() // consume ':'
				}
			}

			// Parse as anonymous usage: the kind an occurrence modifier implies
			// (`ref individual v : V`), else an attribute.
			kind, keyword := modifierImpliedKind(mods)
			if keyword == "" {
				kind = p.anonymousUsageKind(mods)
			}
			u := &ast.Usage{
				Kind:         kind,
				Keyword:      keyword,
				Ident:        id,
				Visibility:   mods.visibility,
				IsAbstract:   mods.isAbstract,
				IsVariation:  mods.isVariation,
				IsVariant:    mods.isVariant,
				IsReference:  mods.isReference,
				IsIndividual: mods.isIndividual,
				IsEvent:      mods.isEvent,
				Portion:      mods.portion,
				IsVariable:   mods.isVariable,
				IsConstant:   mods.isConstant,
				IsDerived:    mods.isDerived,
				IsComposite:  mods.isComposite,
				IsPortion:    mods.isPortion,
				IsEnd:        mods.isEnd,
				IsChain:      mods.isChain,
				Direction:    mods.direction,
				IsOrdered:    mods.isOrdered,
				IsNonunique:  mods.isNonunique,
			}

			if hasTypeOnly {
				p.advance() // consume ':'
			}

			// If we consumed a colon, parse typing relationship(s)
			// Support comma-separated types: : Type1, Type2, Type3
			if hasNameAndType || hasTypeOnly {
				u.Relationships = append(u.Relationships, p.parseTypingRelationships()...)
			}

			u.CrossFeature = mods.cross
			p.parseFeatureSpecializationPart(u)

			// Parse optional value (= expr or default expr)
			p.parseUsageValue(u)

			// Parse body or semicolon
			members, hasBody := p.parseGenericBody()
			u.Members = members
			u.HasBody = hasBody

			u.NodeSpan = p.spanFrom(start)
			mem := &ast.Membership{Visibility: vis, Member: u}
			mem.NodeSpan = p.spanFrom(start)
			mem.SetLeadingTrivia(trivia)
			return mem
		}
		// If not anonymous feature pattern, fallback to parseDeclaration below
	}

	// Check for anonymous feature pattern without modifiers: name : Type
	if p.atName() && nextKind == lexer.Colon {
		var id ast.Identification
		if seg, ok := p.parseNameSegmentRelaxed(); ok {
			id.Name = seg.Text
			id.NameSpan = seg.Span
		}

		// Parse as anonymous usage (attribute by default)
		u := &ast.Usage{
			Kind:  ast.UsageAttribute,
			Ident: id,
		}

		// Parse typing/relationships
		p.advance() // consume ':'
		u.Relationships = append(u.Relationships, p.parseTypingRelationships()...)
		p.parseFeatureSpecializationPart(u)

		// Parse optional value (= expr or default expr)
		p.parseUsageValue(u)

		// Parse body or semicolon
		members, hasBody := p.parseGenericBody()
		u.Members = members
		u.HasBody = hasBody

		u.NodeSpan = p.spanFrom(start)
		mem := &ast.Membership{Visibility: vis, Member: u}
		mem.NodeSpan = p.spanFrom(start)
		mem.SetLeadingTrivia(trivia)
		return mem
	}

	// Check for the bare-name pattern: identifier = expr; OR identifier; OR identifier { body }
	// In an enumeration body it is an enumerated value (low = 0.25; pass; open { doc });
	// in any other body a default reference usage (SysML.xtext DefaultReferenceUsage).
	// But exclude usage-only keywords (inv, subject, etc.) - they're declarations, not enum literal names
	// Also exclude constraint (has both def/usage forms but shouldn't be enum literal name)
	isUsageOnlyKwForEnum := p.at(lexer.Keyword) && (p.peek().KeywordID == "subject" || p.peek().KeywordID == "objective" ||
		p.peek().KeywordID == "succession" || p.peek().KeywordID == "inv" || p.peek().KeywordID == "connector" ||
		// `connect` introduces a connection usage's ends, so `connect;` is that
		// usage missing them rather than a literal named `connect`.
		p.peek().KeywordID == "connect" ||
		p.peek().KeywordID == "satisfy" || p.peek().KeywordID == "verify" || p.peek().KeywordID == "step" || p.peek().KeywordID == "expr" || p.peek().KeywordID == "constraint" ||
		p.peek().KeywordID == "interaction" || p.peek().KeywordID == "bool" || p.peek().KeywordID == "assoc" || p.peek().KeywordID == "struct" ||
		p.peek().KeywordID == "class" || p.peek().KeywordID == "predicate" ||
		// A view or viewpoint member keyword names no literal either: `render;`
		// and `frame;` are members missing the reference they are written with
		// (ViewRenderingUsage, FramedConcernUsage), diagnosed as such.
		p.peek().KeywordID == "render" || p.peek().KeywordID == "frame" ||
		// `include` states the use case it includes, so `include;` is that
		// member missing its reference (SysML.xtext IncludeUseCaseUsage).
		p.peek().KeywordID == "include" ||
		p.peek().KeywordID == "stakeholder" || p.peek().KeywordID == "actor")
	// A relationship keyword is not a literal's name either: `redefines;` and
	// `redefines = 5;` are specializations missing their target, diagnosed as
	// such, exactly as `:>>;` and `:>> = 5;` are.
	// A kind keyword before a body declares an anonymous usage of that kind
	// (`action { … }`), not a literal named by it: a name spelling a reserved
	// keyword must be written as an unrestricted name (KerML §7.2.4).
	anonKindDecl := nextKind == lexer.LBrace && p.isKindKeyword(p.peek())
	if !isUsageOnlyKwForEnum && !anonKindDecl && !p.atFeatureSpecialization() && p.atNameOrKeyword() && (nextKind == lexer.Eq || nextKind == lexer.Semicolon || nextKind == lexer.LBrace) {
		seg, _ := p.parseNameSegmentRelaxed()
		id := ast.Identification{Name: seg.Text, NameSpan: seg.Span}

		var value ast.Node
		var valueOperatorSpan source.Span
		if op, ok := p.accept(lexer.Eq); ok {
			valueOperatorSpan = op.Span
			value = p.ParseExpression()
		}

		kind := ast.UsageAttribute
		if enumValue {
			kind = ast.UsageEnumeration
		}
		members, hasBody := p.parseGenericBody()

		u := &ast.Usage{
			Kind:              kind,
			Ident:             id,
			Value:             value,
			ValueOperatorSpan: valueOperatorSpan,
			Members:           members,
			HasBody:           hasBody,
		}
		u.NodeSpan = p.spanFrom(start)

		mem := &ast.Membership{Visibility: vis, Member: u}
		mem.NodeSpan = p.spanFrom(start)
		mem.SetLeadingTrivia(trivia)
		return mem
	}

	// A keyword before a kind keyword qualifies the declaration rather than
	// naming it (`var feature x`, `assert constraint { ... }`), so it is consumed
	// and the declaration keeps the name it declares for itself. A keyword that
	// is not such a prefix is the declaration's own kind, and the keyword after
	// it is its name (`action flow { ... }` is an action named `flow`); that is
	// parsed below, which reads the name instead of dropping it.
	if p.atKindPrefix() {
		prefix := p.kindPrefixWord()
		p.advance() // consume the prefix keyword
		inner := p.parseDeclaration(start)
		if inner == nil {
			en := p.errorNodeSkip(start, msgExpectedBodyMember)
			en.SetLeadingTrivia(trivia)
			return en
		}
		// A prefix that says what the declaration is for is part of it
		// (`assert constraint c` is an AssertConstraintUsage).
		if u, ok := inner.(*ast.Usage); ok && kindPrefixKeywords[prefix] && u.PrefixKeyword == "" {
			u.PrefixKeyword = prefix
		}
		if u, ok := inner.(*ast.Usage); ok && prefix == varPrefixWord {
			u.IsVariable = true
		}
		mem := &ast.Membership{Visibility: vis, Member: inner}
		mem.NodeSpan = p.spanFrom(start)
		mem.SetLeadingTrivia(trivia)
		return mem
	}

	inner := p.parseDeclaration(start)
	if inner == nil {
		en := p.errorNodeSkip(start, p.noBodyMemberMessage())
		en.SetLeadingTrivia(trivia)
		return en
	}
	mem := &ast.Membership{Visibility: vis, Member: inner}
	mem.NodeSpan = p.spanFrom(start)
	mem.SetLeadingTrivia(trivia)
	return mem
}

// noBodyMemberMessage reports why the current token starts no body member,
// naming the missing declaration when a relationship keyword that is not a
// feature specialization stands in place of one (SysML.xtext
// SubclassificationPart, TypeRelationshipPart, FeatureRelationshipPart).
func (p *Parser) noBodyMemberMessage() string {
	const base = msgExpectedBodyMember
	t := p.peek()
	if t.Kind != lexer.Keyword {
		return base
	}
	switch t.KeywordID {
	case "specializes":
		return base + ": 'specializes' relates two types; a member refines an inherited feature by subsetting it, written 'subsets' or ':>'"
	case "unions", "intersects", "chains", "inverse", "featured":
		return base + ": '" + t.KeywordID + "' relates the declaration written before it, so a member cannot begin with it"
	case "disjoint":
		if p.peekIsKeyword(1, "from") {
			return base + ": expected the disjoined type before 'from'"
		}
	}
	return base
}

// parsePerformedActionReference parses the reference form of a performed action
// usage — a feature reference with an optional body — which SysML.xtext spells
// `OwnedReferenceSubsetting FeatureSpecializationPart? ValuePart?` followed by
// an `ActionBody` (PerformActionUsageDeclaration; SysML v2 §7.17.6). The keyword
// that introduced it is already consumed; kw carries it for errors and to record
// which synonym was written.
func (p *Parser) parsePerformedActionReference(start int, mods featureMods, kw string) *ast.Usage {
	return p.parseReferenceMemberUsage(start, ast.UsageAction, kw, "action", mods, p.parseActionBodyMixed, true)
}

// memberKeywordNames are the SysML member keywords KerML does not reserve, so a
// KerML declaration may carry one as its name (`in frame : SpatialFrame[1]`).
var memberKeywordNames = map[string]bool{
	"frame": true, "render": true, "subject": true, "actor": true,
	"stakeholder": true, "objective": true,
}

// atMemberKeywordUsedAsKeyword reports whether the member keyword at the cursor
// introduces a member (a name or its kind keyword follows) or names a KerML feature.
func (p *Parser) atMemberKeywordUsedAsKeyword(kw string) bool {
	if p.src.Kind() != source.KindKerML {
		return true
	}
	next := p.peekN(1)
	switch next.Kind {
	case lexer.Identifier, lexer.UnrestrictedName:
		return true
	case lexer.Keyword:
		switch kw {
		case "frame":
			return next.KeywordID == "concern"
		case "render":
			return next.KeywordID == "rendering"
		}
	}
	return false
}

// prefixMetadataFollowsKeyword reports whether kw takes its prefix metadata after itself
// (`subject #M s;`, SysML.xtext `'keyword' UsageExtensionKeyword* …`), unlike `#B assert …`.
func prefixMetadataFollowsKeyword(kw string) bool {
	switch kw {
	case "subject", "actor", "stakeholder", "objective", "variant", "assume", "require":
		return true
	}
	return false
}

// reportMisplacedPrefixMetadata reports prefix metadata written ahead of the keyword
// at the cursor that it must follow, with the edit that moves it there.
func (p *Parser) reportMisplacedPrefixMetadata(prefixes []*ast.PrefixMetadata, kw lexer.Token) {
	content := p.src.Bytes()
	texts := make([]string, 0, len(prefixes))
	edits := make([]quickfix.Edit, 0, len(prefixes)+1)
	var span source.Span
	for i, pm := range prefixes {
		// A node span runs to the next token, past any comment; the name's does not.
		sp := pm.Span()
		end := sp.Offset + len(strings.TrimRight(p.src.Text(sp), " \t\r\n"))
		if n := len(pm.Type.Parts); n > 0 {
			end = pm.Type.Parts[n-1].Span.End()
		}
		texts = append(texts, p.src.Text(source.Span{Offset: sp.Offset, Len: end - sp.Offset}))
		if i == 0 {
			span.Offset = sp.Offset
		}
		span.Len = end - span.Offset
		for end < kw.Span.Offset && (content[end] == ' ' || content[end] == '\t') {
			end++
		}
		edits = append(edits, quickfix.Replace(source.Span{Offset: sp.Offset, Len: end - sp.Offset}, ""))
	}
	run := strings.Join(texts, " ")
	edits = append(edits, quickfix.Insert(kw.Span.End(), " "+run))

	example := kw.KeywordID + " " + run
	switch next := p.peekN(1); next.Kind {
	case lexer.Identifier, lexer.UnrestrictedName, lexer.Keyword:
		example += " " + p.src.Text(next.Span)
	}
	p.errorWithFixes(span, "prefix metadata follows '"+kw.KeywordID+"': write `"+example+"`", quickfix.Fix{
		Title:     "move the prefix metadata after '" + kw.KeywordID + "'",
		Edits:     edits,
		Preferred: true,
	})
}

// parseReferenceMemberUsage parses the reference form that SysML.xtext spells
// `ownedRelationship += OwnedReferenceSubsetting FeatureSpecializationPart?
// ValuePart?` followed by the member's body: a performed action
// (PerformActionUsageDeclaration), the rendering a view names
// (ViewRenderingUsage) and the concern a requirement frames
// (FramedConcernUsage) are all written that way. Such a member names an
// existing feature and declares no name of its own, so the reference is
// recorded as a ReferenceSubsetting and the identification left empty; the name
// the member answers to is its reference's (KerML 7.3.4.5, ast.EffectiveName).
//
// The introducing keyword is already consumed; kw records which synonym was
// written, noun names the referenced element in diagnostics, parseBody parses
// the body members once '{' is consumed, and allowValue states whether the
// notation admits a `ValuePart`: a performed action does, the rendering a view
// names and the concern a requirement frames do not.
func (p *Parser) parseReferenceMemberUsage(start int, kind ast.UsageKind, kw, noun string, mods featureMods, parseBody func() []ast.Node, allowValue bool) *ast.Usage {
	u := &ast.Usage{
		Kind:        kind,
		Keyword:     kw,
		IsAbstract:  mods.isAbstract,
		IsReference: mods.isReference,
		IsVariable:  mods.isVariable,
		IsEnd:       mods.isEnd,
		Visibility:  mods.visibility,
		Direction:   mods.direction,
		IsComposite: mods.isComposite,
		IsPortion:   mods.isPortion,
	}
	u.NodeBase.NodeSpan = p.spanFrom(start)

	var target ast.Node
	if p.atNameOrKeyword() {
		target = p.parseRelationshipTarget()
	}
	if target != nil {
		u.Relationships = append(u.Relationships, &ast.Relationship{
			Kind:   ast.RelReferences,
			Target: target,
		})
	} else {
		p.error(p.peek().Span, fmt.Sprintf("expected a %s reference after '%s'", noun, kw))
	}

	// FeatureSpecializationPart? ValuePart?
	if p.at(lexer.LBracket) {
		u.Multiplicity = p.parseMultiplicity()
	}
	specRels := p.parseRelationships(true)
	u.Relationships = append(u.Relationships, specRels...)
	if allowValue {
		p.parseUsageValue(u)
	}

	switch {
	case p.atTransitionEffectStatement(start) && p.atEffectEnd():
		// A performed action written as a transition's effect is ended by the
		// transition's next clause or by the ';' the transition itself consumes.
		u.HasBody = false
	case p.accept2(lexer.Semicolon):
		u.HasBody = false
	case p.at(lexer.LBrace):
		p.advance()
		leave := p.pushBodyContext(usageBodyContext(kind))
		u.Members = parseBody()
		leave()
		u.HasBody = true
	case allowValue && (p.atKeyword("then") || p.atKeyword("if") || p.atKeyword("do")):
		// A performed action written as a transition's effect is terminated by
		// the transition's next clause: `do perform notify then idle;`.
		u.HasBody = false
	case target != nil:
		p.expectBodyOrEnd(fmt.Sprintf("'%s' %s reference", kw, noun))
	}

	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseBindingDeclaration parses a binding connector from after its kind keyword
// to its body: SysML `UsageDeclaration? 'bind' end '=' end` (SysML.xtext
// BindingConnectorAsUsage) or KerML `FeatureDeclaration ('of' end '=' end)? |
// 'of'? end '=' end` (KerML.xtext BindingConnectorDeclaration); either file
// kind takes either spelling.
func (p *Parser) parseBindingDeclaration(u *ast.Usage, keyword string) {
	if keyword == "bind" {
		p.parseBindingEnds(u)
		return
	}
	switch {
	case p.at(lexer.LBrace) || p.at(lexer.Semicolon):
		// `binding { end …; end …; }` states its ends as members.
	case p.acceptKeyword("of"), p.acceptKeyword("bind"), p.atBindingEnds():
		p.parseBindingEnds(u)
	default:
		p.parseBindingFeatureDeclaration(u)
		if p.acceptKeyword("of") || p.acceptKeyword("bind") {
			p.parseBindingEnds(u)
		}
	}
}

// parseBindingFeatureDeclaration parses the declaration a binding may state
// before its ends: `Identification? FeatureSpecializationPart?` (KerML.xtext
// FeatureDeclaration), as in `binding ab : AB [1] bind a = b`.
func (p *Parser) parseBindingFeatureDeclaration(u *ast.Usage) {
	if p.atName() || p.at(lexer.Lt) {
		u.Ident = p.parseIdentification()
	}
	p.parseFeatureSpecializationPart(u)
}

// atBindingEnds reports whether the cursor is at a whole ConnectorEnd followed
// by `=`, which states a KerML binding's ends where a declaration could stand
// (KerML.xtext:875): `binding a = b`, `binding [1] e ::> a = b`.
func (p *Parser) atBindingEnds() bool {
	from := p.pastBracketed(0)
	if p.endThenAt(from, lexer.Eq, "") {
		return true
	}
	if !p.atNameAt(from) {
		return false
	}
	if p.peekN(from+1).Kind == lexer.ColonColonGt || p.peekIsKeyword(from+1, "references") {
		return p.endThenAt(from+2, lexer.Eq, "")
	}
	return false
}

// parseBindingEnds parses the two ends of a binding, `end '=' end`, recording a
// diagnostic and stopping where an end or the `=` is missing.
func (p *Parser) parseBindingEnds(u *ast.Usage) {
	first := p.parseBindingEnd()
	if first == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, first)
	if !p.accept2(lexer.Eq) {
		p.error(p.peek().Span, "expected '=' between binding ends")
		return
	}
	if second := p.parseBindingEnd(); second != nil {
		u.ConnectorEnds = append(u.ConnectorEnds, second)
	}
}

// parseBindingEnd parses one binding end. A missing end or an expression written
// there is reported and kept as an ErrorNode target, so the end still counts.
func (p *Parser) parseBindingEnd() *ast.ConnectorEnd {
	start := p.peek().Span.Offset
	global := p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon
	if p.at(lexer.LBracket) || p.atNameOrKeyword() || global {
		cp := p.checkpoint()
		end := p.parseConnectorEnd()
		expression := end != nil && p.atExpressionOperator()
		if expression {
			p.restore(cp)
		}
		p.release()
		if !expression {
			return end
		}
	}
	end := &ast.ConnectorEnd{}
	if p.at(lexer.LBracket) {
		end.Multiplicity = p.parseMultiplicity()
	}
	if p.at(lexer.Semicolon) || p.at(lexer.LBrace) || p.at(lexer.RBrace) || p.atEOF() {
		const msg = "expected a binding end"
		p.error(p.peek().Span, msg)
		en := &ast.ErrorNode{Message: msg}
		en.NodeSpan = p.spanFrom(start)
		end.Target = en
	} else if expr := p.ParseExpression(); expr != nil {
		en, failed := expr.(*ast.ErrorNode)
		if !failed {
			const msg = "a binding end names a feature, not an expression; " +
				"declare a feature with the expression as its value and bind to that"
			p.error(expr.Span(), msg)
			en = &ast.ErrorNode{Message: msg}
			en.NodeSpan = expr.Span()
		}
		end.Target = en
	}
	end.NodeSpan = p.spanFrom(start)
	return end
}

// atExpressionOperator reports whether the current token continues the name
// before it into an expression, which a connector end never is.
func (p *Parser) atExpressionOperator() bool {
	switch p.peek().Kind {
	case lexer.Question, lexer.QuestionQ, lexer.Pipe, lexer.Amp, lexer.EqEq, lexer.NotEq,
		lexer.EqEqEq, lexer.NotEqEq, lexer.Lt, lexer.Gt, lexer.Le, lexer.Ge, lexer.Plus,
		lexer.Minus, lexer.Star, lexer.Slash, lexer.Percent, lexer.StarStar, lexer.Caret,
		lexer.LParen, lexer.Arrow, lexer.DotQuestion, lexer.At, lexer.AtAt:
		return true
	case lexer.Keyword:
		switch p.peek().KeywordID {
		case "and", "or", "xor", "implies", "as", "istype", "hastype", "meta":
			return true
		}
	}
	return false
}

// namesFeature reports whether a parsed expression names a feature, as a
// binding's right end (SysML.xtext ConnectorEndMember) and an assignment's
// target (SysML.xtext TargetParameter) must.
func namesFeature(n ast.Node) bool {
	switch n.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		return true
	}
	return false
}

// parseRelationshipTarget parses a relationship target which can be either:
// - A qualified name (A::B::C)
// - A feature chain (A.B.C or A::B.C.D - mix of :: and .)
// Returns Node interface (either *QualifiedName or *FeatureChainExpr).
// Does NOT consume body expressions ({ in ... }) unlike ParseExpression().
func (p *Parser) parseRelationshipTarget() ast.Node {
	start := p.peek().Span.Offset

	// Start with qualified name (handles A::B::C)
	// Use parseQualifiedNameRelaxed to allow keywords like "do" in feature chains (e.g., do.startShot)
	base := p.parseQualifiedNameRelaxed()
	if base == nil {
		return nil
	}

	// Check for dot extensions (feature chain)
	if !p.at(lexer.Dot) && !p.at(lexer.DotDot) {
		return base // Just a qualified name
	}

	// Build feature chain expression
	var operand ast.Node = &ast.FeatureReference{Name: base}
	operand.(*ast.FeatureReference).NodeSpan = base.NodeSpan

	for p.at(lexer.Dot) || p.at(lexer.DotDot) {
		// `a..b` chains over an empty segment; report it where it is written and
		// read the rest of the chain, so recovery stays inside this reference.
		if p.at(lexer.DotDot) {
			p.error(p.peek().Span, "expected a name after '.'")
		}
		p.advance() // consume '.' or '..'
		// Each chaining feature of a chain is itself a qualified name
		// (KerML OwnedFeatureChaining: chainingFeature = [Feature|QualifiedName]).
		if !p.atNameOrKeyword() && !(p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon) {
			p.error(p.peek().Span, "expected a name after '.'")
			break
		}
		memberName := p.parseQualifiedNameRelaxed()
		if memberName == nil {
			break
		}

		chain := &ast.FeatureChainExpr{
			Operand: operand,
			Member:  memberName,
		}
		chain.NodeSpan = p.spanFrom(start)
		operand = chain
	}

	return operand
}

// parsePreNameRelationships parses the specializations a declaration may state
// where its name would go. A word the grammar does not reserve names the
// declaration instead, so only a reserved spelling begins a clause here.
func (p *Parser) parsePreNameRelationships(isUsage bool) []*ast.Relationship {
	if t := p.peek(); t.Kind == lexer.Keyword && !p.reservedWord(t.KeywordID) {
		return nil
	}
	return p.parseRelationships(isUsage)
}

// parseFeatureSpecializationPart parses a usage's
// `FeatureSpecialization* MultiplicityPart? FeatureSpecialization*` (KerML.xtext:574) onto u.
func (p *Parser) parseFeatureSpecializationPart(u *ast.Usage) {
	u.Relationships = append(u.Relationships, p.parseRelationships(true)...)
	if p.at(lexer.LBracket) {
		u.Multiplicity = p.parseMultiplicity()
	}
	p.parseSpecializationsAfterMultiplicity(u)
}

// parseSpecializationsAfterMultiplicity parses the `ordered`/`nonunique` tail of
// a MultiplicityPart and the FeatureSpecialization* that may follow it onto u.
func (p *Parser) parseSpecializationsAfterMultiplicity(u *ast.Usage) {
	post := p.parsePostModifiers()
	u.IsOrdered = u.IsOrdered || post.isOrdered
	u.IsNonunique = u.IsNonunique || post.isNonunique
	u.Relationships = append(u.Relationships, p.parseRelationships(true)...)
}

// parseRelationships parses zero or more relationship clauses. isUsage selects
// the meaning of the symbolic `:>` operator (subsets on a usage, specializes on
// a definition). Each clause may carry a comma-separated target list; every
// target becomes its own Relationship sharing the clause kind.
func (p *Parser) parseRelationships(isUsage bool) (rels []*ast.Relationship) {
	for {
		if p.atDeclarationConjugation() {
			rels = append(rels, p.parseDeclarationConjugation())
			continue
		}
		kind, ok := p.relationshipClauseKind(isUsage)
		if !ok {
			return rels
		}
		for {
			r := p.parseRelationshipClauseTarget(kind)
			rels = append(rels, r)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
	}
}

// anonymousUsageKind returns the kind of a usage declared without a kind
// keyword. An interface body's default end is a port usage (SysML v2 8.2.2.14
// DefaultInterfaceEnd); anywhere else the kind-less form is a reference usage,
// which this parser represents as an attribute usage.
func (p *Parser) anonymousUsageKind(mods featureMods) ast.UsageKind {
	if mods.isEnd && p.bodyContext() == bodyInterface {
		return ast.UsagePort
	}
	return ast.UsageAttribute
}

// parseAnonymousEnd parses an `end` whose declaration is omitted entirely
// (`end ;`, `end { ... }`) as a body member.
func (p *Parser) parseAnonymousEnd(start int, trivia []ast.Trivia, vis ast.Visibility, mods featureMods) ast.Node {
	node := p.parseAnonymousEndUsage(start, mods)
	if en, ok := node.(*ast.ErrorNode); ok {
		en.SetLeadingTrivia(trivia)
		return en
	}
	mem := &ast.Membership{Visibility: vis, Member: node}
	mem.NodeSpan = node.Span()
	mem.SetLeadingTrivia(trivia)
	return mem
}

// parseAnonymousEndUsage parses an `end` whose declaration is omitted entirely.
// Only an interface body's default end (SysML v2 8.2.2.14.1) and an explicit
// `end ref` (8.2.2.7.2 ReferenceUsage) may omit it.
func (p *Parser) parseAnonymousEndUsage(start int, mods featureMods) ast.Node {
	kind := ast.UsageAttribute
	switch {
	case p.bodyContext() == bodyInterface:
		kind = ast.UsagePort
	case mods.isReference:
	default:
		return p.errorNodeSkip(start,
			"this `end` must declare a name, type or `ref` (write `end ref;`): only an interface body may declare a bare `end;` (SysML v2 8.2.2.14.1 DefaultInterfaceEnd)")
	}
	u := &ast.Usage{
		Kind:         kind,
		IsEnd:        true,
		Visibility:   mods.visibility,
		IsReference:  mods.isReference,
		IsDerived:    mods.isDerived,
		IsComposite:  mods.isComposite,
		IsPortion:    mods.isPortion,
		Direction:    mods.direction,
		CrossFeature: mods.cross,
	}
	u.Members, u.HasBody = p.parseGenericBody()
	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseTypingRelationships parses the comma-separated target list of a typing
// clause whose ':' has already been consumed.
func (p *Parser) parseTypingRelationships() []*ast.Relationship {
	var rels []*ast.Relationship
	for {
		rels = append(rels, p.parseRelationshipClauseTarget(ast.RelTyping))
		if !p.accept2(lexer.Comma) {
			return rels
		}
	}
}

// parseRelationshipClauseTarget parses one target of a relationship clause,
// including the `~` of a conjugated port typing (SysML v2 8.2.2.12).
func (p *Parser) parseRelationshipClauseTarget(kind ast.RelationshipKind) *ast.Relationship {
	start := p.peek().Span.Offset
	tildeTok, conjugated := p.accept(lexer.Tilde)
	if conjugated && kind != ast.RelTyping {
		p.error(tildeTok.Span, "'~' conjugates a port type and is only allowed after ':' or 'defined by'")
	}
	// Handles both qualified names and feature chains, but not body expressions.
	target := p.parseRelationshipTarget()
	if target == nil && conjugated {
		p.error(p.peek().Span, "expected a port definition name after '~'")
	}
	r := &ast.Relationship{Kind: kind, Target: target, Conjugated: conjugated && kind == ast.RelTyping}
	r.NodeSpan = p.spanFrom(start)
	return r
}

// parseTierBEnds parses the distinctive Tier B usage grammar following the
// declaration head: connector ends (connection/interface/allocation) and flow
// ends + payload (flow). Other kinds contribute nothing.
func (p *Parser) parseTierBEnds(u *ast.Usage, kind ast.UsageKind) {
	switch kind {
	case ast.UsageConnection, ast.UsageInterface:
		// `connection c connect a to b` states its ends after `connect`; `connect a
		// to b` and `interface a.p to b.p` state them after the kind keyword itself.
		switch {
		case p.atKeyword("connect"):
			p.parseConnectorEnds(u, "connect")
		case u.Keyword == "connect":
			// The keyword introduced the ends, so it is the ConnectorPart of the
			// grammar and states at least one end.
			p.parseConnectorEnds(u, "")
			if len(u.ConnectorEnds) == 0 {
				p.error(p.peek().Span, "expected connector end after 'connect'")
			}
		case p.atConnectorShorthandEnds():
			p.parseConnectorEnds(u, "")
		}
	case ast.UsageConnector:
		// Connector can use four syntaxes:
		// 1. "connect X to Y" - standard connector ends
		// 2. "[name] from X to Y" - from/to syntax
		// 3. "X to Y" - anonymous binary ends (KerML.xtext:836 BinaryConnectorDeclaration)
		// 4. "(X, Y, Z)" - n-ary end list (KerML.xtext:842 NaryConnectorDeclaration)
		if p.atKeyword("connect") {
			p.parseConnectorEnds(u, "connect")
		} else if p.at(lexer.LParen) {
			p.parseNaryConnectorEnds(u)
		} else if !declaresConnector(u) && p.atConnectorBinaryEnds("to") {
			p.parseConnectorEnds(u, "")
		} else {
			p.parseConnectorFromTo(u)
		}
	case ast.UsageSuccession:
		// A succession may state its ends as body members instead
		// (KerML.xtext SuccessionDeclaration:891): `succession { end ...; }`.
		if !p.at(lexer.LBrace) && !p.at(lexer.Semicolon) {
			p.parseConnectorEnds(u, "") // succession has no intermediate keyword
		}
	case ast.UsageAllocation:
		// `allocate` states a ConnectorPart, either as the kind keyword —
		// `allocate X to Y` — or after an `allocation` declaration —
		// `allocation al allocate X to Y`. Without it, `allocation al;` declares
		// a plain allocation usage (AllocationUsageDeclaration,
		// SysML.xtext:1219-1222).
		allocateStatesEnds := u.Keyword == "allocate" || p.acceptKeyword("allocate")

		if p.atKeyword("to") {
			// Single-end form: allocate to target
			p.advance() // consume "to"
			end := p.parseConnectorEnd()
			if end != nil {
				u.ConnectorEnds = append(u.ConnectorEnds, end)
			}
		} else if !p.at(lexer.LBrace) && !p.at(lexer.Semicolon) {
			// Binary form: allocate source to target
			// Only parse connector ends if NOT at body start
			p.parseConnectorEnds(u, "") // no intermediate keyword
		}
		if allocateStatesEnds && len(u.ConnectorEnds) == 0 {
			p.error(p.peek().Span, "expected connector end after 'allocate'")
		}
	case ast.UsageFlow:
		p.parseFlowEnds(u)
	case ast.UsageMetadata:
		// `about` names the elements the usage annotates, each a qualified name
		// (SysML.xtext AnnotatedElement: [Element|QualifiedName]).
		if p.acceptKeyword("about") {
			about := p.parseQualifiedNameList()
			if len(about) == 0 {
				p.error(p.peek().Span, "expected an annotated element after 'about'")
			}
			for _, target := range about {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelAnnotates,
					Target: target,
				})
			}
		}
	case ast.UsageOccurrence:
		// Occurrence usage (message) syntax: message name of payload : Type from sender to receiver;
		// The 'from X to Y' connector ends specify sender and receiver
		if p.atKeyword("from") || p.atKeyword("to") {
			p.parseConnectorFromTo(u)
		}
	}
}

// parseConnectorEnds parses `<kw> end to end` (binary) or
// `<kw> ( end , end , ... )` (n-ary), where <kw> is `connect` or `allocate`.
// For succession, kw is empty and the pattern is directly `end then end`.
// Each end can optionally have a multiplicity: `[mult] end`.
// The connector clause is optional. On a malformed end, it records a diagnostic,
// keeps the ends parsed so far, and stops (the declaration remains a Usage).
func (p *Parser) parseConnectorEnds(u *ast.Usage, kw string) {
	// For connection/allocation, expect intermediate keyword ('connect'/'allocate')
	// For succession, no intermediate keyword (kw is empty)
	if kw != "" {
		if !p.acceptKeyword(kw) {
			return
		}
	}
	if p.at(lexer.LParen) {
		p.advance() // '('
		for {
			ce := p.parseConnectorEnd()
			if ce == nil {
				return // parseConnectorEnd recorded the diagnostic; keep partial ends
			}
			u.ConnectorEnds = append(u.ConnectorEnds, ce)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.RParen, "expected ')' to close connector ends")
		return
	}
	// Binary form: end keyword end (where keyword is "to" for connection, "then" for succession).
	// For succession, support optional "first" keyword: first end then end
	if u.Kind == ast.UsageSuccession {
		p.acceptKeyword("first") // optional "first" before first end
	}
	from := p.parseConnectorEnd()
	if from == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, from)

	// Check for optional "references" keyword after first end
	// Pattern: end X references Y to end Z
	if p.acceptKeyword("references") {
		refTarget := p.parseRelationshipTarget()
		if refTarget != nil {
			from.Reference = refTarget
		}
	}

	// Determine expected keyword based on usage kind
	var expectedKeyword string
	switch u.Kind {
	case ast.UsageSuccession:
		expectedKeyword = "then"
	default:
		expectedKeyword = "to"
	}

	if !p.acceptKeyword(expectedKeyword) {
		p.error(p.peek().Span, fmt.Sprintf("expected '%s' between connector ends", expectedKeyword))
		return
	}
	to := p.parseConnectorEnd()
	if to == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, to)

	// Check for optional "references" keyword after second end
	if p.acceptKeyword("references") {
		refTarget := p.parseRelationshipTarget()
		if refTarget != nil {
			to.Reference = refTarget
		}
	}
}

// parseConnectorEnd parses one connector end and its optional reference subsetting.
func (p *Parser) parseConnectorEnd() *ast.ConnectorEnd {
	start := p.peek().Span.Offset
	ce := &ast.ConnectorEnd{}

	// Optional multiplicity
	if p.at(lexer.LBracket) {
		ce.Multiplicity = p.parseMultiplicity()
	}

	// A connector end has one target, unlike a general relationship clause.
	ce.Target = p.parseRelationshipTarget()
	if ce.Target == nil {
		return nil
	}

	// A named end uses ReferencesKeyword to state the feature it attaches to.
	// Keep explicit end redefinitions, but never consume comma-separated targets.
	for {
		var kind ast.RelationshipKind
		switch {
		case p.accept2(lexer.ColonColonGt):
			kind = ast.RelReferences
		case p.acceptKeyword("references"):
			kind = ast.RelReferences
		case p.accept2(lexer.ColonGtGt):
			kind = ast.RelRedefines
		default:
			ce.NodeSpan = p.spanFrom(start)
			return ce
		}
		rel := p.parseRelationshipClauseTarget(kind)
		if rel.Target == nil {
			p.error(p.peek().Span, "expected a relationship target for connector end")
		}
		ce.Relationships = append(ce.Relationships, rel)
	}

}

// parseNaryConnectorEnds parses the parenthesized end list a KerML connector
// declaration states without an introducing keyword; the grammar requires at
// least two ends there (KerML.xtext:842).
func (p *Parser) parseNaryConnectorEnds(u *ast.Usage) {
	before := len(u.ConnectorEnds)
	p.parseConnectorEnds(u, "")
	if len(u.ConnectorEnds)-before == 1 {
		p.error(u.ConnectorEnds[before].Span(),
			"expected at least two connector ends in a parenthesized end list")
	}
}

// parseConnectorFromTo parses the `from x to y` pattern for connector usages.
// Pattern: `from <end> [references <target>] to <end> [references <target>]` (binary form only).
func (p *Parser) parseConnectorFromTo(u *ast.Usage) {
	if !p.acceptKeyword("from") {
		return // Optional connector clause
	}

	from := p.parseConnectorEnd()
	if from == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, from)

	// Check for optional "references" keyword after from end
	if p.acceptKeyword("references") {
		refTarget := p.parseRelationshipTarget()
		if refTarget != nil {
			from.Reference = refTarget
		}
	}

	if !p.acceptKeyword("to") {
		p.error(p.peek().Span, "expected 'to' between connector ends")
		return
	}

	to := p.parseConnectorEnd()
	if to == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, to)

	// Check for optional "references" keyword after to end
	if p.acceptKeyword("references") {
		refTarget := p.parseRelationshipTarget()
		if refTarget != nil {
			to.Reference = refTarget
		}
	}
}

// declaresConnector reports whether a connector stated a declaration of its own
// (name, specialization or multiplicity), after which ends need `from`.
func declaresConnector(u *ast.Usage) bool {
	return u.Ident.Name != "" || u.Ident.ShortName != "" || len(u.Relationships) > 0 || u.Multiplicity != nil
}

// atConnectorBinaryEnds reports whether the cursor is at a whole ConnectorEnd
// (KerML.xtext:854: `[0..1]`? (`e ::>` | `e references`)? `$::`? chain) followed by
// the delimiter kw — `to` for a connector, `then` for a succession. A name may only
// precede `from`/`first` (KerML.xtext:836, :891), so such an end has no declaration
// before it: `connector [0..1] a to b`, `succession [1] e ::> a then b`.
func (p *Parser) atConnectorBinaryEnds(kw string) bool {
	from := p.pastBracketed(0)
	if p.endThenKeywordAt(from, kw) {
		return true
	}
	if !p.atNameAt(from) {
		return false
	}
	if p.peekN(from+1).Kind == lexer.ColonColonGt || p.peekIsKeyword(from+1, "references") {
		return p.endThenKeywordAt(from+2, kw)
	}
	return false
}

// atEndThenKeyword reports whether the cursor is at a connector end — a name or
// arbitrarily deep feature chain such as `differential.leftDiffPort` — followed
// by kw.
func (p *Parser) atEndThenKeyword(kw string) bool {
	return p.endThenKeywordAt(0, kw)
}

// endThenKeywordAt is atEndThenKeyword from the token at offset from.
func (p *Parser) endThenKeywordAt(from int, kw string) bool {
	return p.endThenAt(from, lexer.Keyword, kw)
}

// endThenAt reports whether a connector end — a name, feature chain or global
// `$::` path — starts at offset from and is followed by a token of kind k, or by
// the keyword kw where k is lexer.Keyword.
func (p *Parser) endThenAt(from int, k lexer.Kind, kw string) bool {
	if p.peekN(from).Kind == lexer.Dollar && p.peekN(from+1).Kind == lexer.ColonColon {
		from += 2
	}
	if !p.atNameAt(from) {
		return false
	}
	i := from + 1
	for p.peekN(i).Kind == lexer.Dot || p.peekN(i).Kind == lexer.ColonColon {
		switch p.peekN(i + 1).Kind {
		case lexer.Identifier, lexer.UnrestrictedName, lexer.Keyword:
			i += 2
		default:
			return false
		}
	}
	if k == lexer.Keyword {
		return p.peekIsKeyword(i, kw)
	}
	return p.peekN(i).Kind == k
}

// atFlowShorthand reports whether the parser sits at a bare flow shorthand
// `x to y` (an end immediately followed by the `to` keyword), which has no
// declaration name (SysML.xtext `FlowDeclaration`, second alternative).
func (p *Parser) atFlowShorthand() bool {
	return p.atEndThenKeyword("to")
}

// atAllocateShorthand reports whether an allocation usage names its first
// connector end rather than itself: in `allocate torqueGenerator to powerTrain`
// both names are ends, while `allocate a1 : AllocDef` declares a named usage.
func (p *Parser) atAllocateShorthand() bool {
	return p.atEndThenKeyword("to")
}

// atConnectorShorthandEnds reports whether the cursor is at connector ends stated
// with no keyword of their own: `connect x.p to y.p`, `interface (a.p, b.p)`
// (SysML.xtext ConnectorPart, InterfacePart). A multiplicity written here is the
// first end's, so the ends are recognized past it.
func (p *Parser) atConnectorShorthandEnds() bool {
	if p.at(lexer.LParen) {
		return true
	}
	return p.endThenKeywordAt(p.pastBracketed(0), "to")
}

// pastBracketed returns the offset of the token after the balanced `[…]` group
// at offset from, or from itself where no group starts there.
func (p *Parser) pastBracketed(from int) int {
	if p.peekN(from).Kind != lexer.LBracket {
		return from
	}
	depth := 0
	for i := from; p.peekN(i).Kind != lexer.EOF; i++ {
		switch p.peekN(i).Kind {
		case lexer.LBracket:
			depth++
		case lexer.RBracket:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return from
}

// atPayloadDeclaration reports whether the payload after `of` declares a feature
// of its own (`of name : T`, `of name[1] : T`) rather than stating only the
// payload's type (`of T`, `of T[1]`). The declaration form is the Payload
// alternative carrying a PayloadFeatureSpecializationPart (SysML.xtext:1303),
// whose multiplicity may precede the typing.
func (p *Parser) atPayloadDeclaration() bool {
	if !p.atName() {
		return false
	}
	return p.peekN(p.pastBracketed(1)).Kind == lexer.Colon
}

// parseFlowEnds parses an optional `of <payload>` followed by either
// `from <x> to <y>` or the shorthand `<x> to <y>`. On a malformed end it records
// a diagnostic and keeps whatever ends were parsed so far.
func (p *Parser) parseFlowEnds(u *ast.Usage) {
	start := p.peek().Span.Offset
	var fe *ast.FlowEnds
	hasOf := p.acceptKeyword("of")
	if hasOf {
		fe = &ast.FlowEnds{}
		// Payload can be:
		// 1. Simple reference: of Type, of Type[1], of [1] Type
		// 2. Typed declaration: of name : Type, of name[1] : Type, of name : Type[1]
		// Check for (name + colon) pattern to distinguish
		if p.atPayloadDeclaration() {
			// Typed declaration - parse as nested member
			// Create a usage for the payload declaration
			payloadStart := p.peek().Span.Offset
			payloadUsage := &ast.Usage{
				Kind: ast.UsageAttribute, // default to attribute
			}
			payloadUsage.Ident = p.parseIdentification()

			// A PayloadFeatureSpecializationPart takes the multiplicity on either
			// side of the typing (SysML.xtext:1309).
			if p.at(lexer.LBracket) {
				payloadUsage.Multiplicity = p.parseMultiplicity()
			}

			// Parse typing relationship
			if p.accept2(lexer.Colon) {
				typeName := p.parseQualifiedName()
				if typeName != nil {
					payloadUsage.Relationships = append(payloadUsage.Relationships, &ast.Relationship{
						Kind:   ast.RelTyping,
						Target: typeName,
					})
				}
			}
			if payloadUsage.Multiplicity == nil && p.at(lexer.LBracket) {
				payloadUsage.Multiplicity = p.parseMultiplicity()
			}

			// Parse optional value assignment: = expr
			if op, ok := p.accept(lexer.Eq); ok {
				payloadUsage.ValueOperatorSpan = op.Span
				payloadUsage.Value = p.ParseExpression()
			}

			// The declaration is a member like any other, so it carries its own
			// span: the symbol built from it is what go-to-definition, hover and
			// rename identify a payload by.
			payloadUsage.NodeSpan = p.spanFrom(payloadStart)

			// Store payload usage as member (nested in flow)
			u.Members = append(u.Members, payloadUsage)
			fe.PayloadDecl = payloadUsage
			// Also store reference in FlowEnds for compatibility (create QualifiedName from identifier)
			qn := &ast.QualifiedName{
				Parts: []ast.NameSegment{
					{Text: payloadUsage.Ident.Name, Span: payloadUsage.Ident.NameSpan},
				},
			}
			qn.NodeSpan = payloadUsage.Ident.NameSpan
			fe.Payload = qn
		} else if p.at(lexer.LBracket) {
			// `of [1] Publish` — the multiplicity may precede the typing
			// (SysML.xtext:1306).
			fe.PayloadMultiplicity = p.parseMultiplicity()
			fe.Payload = p.parseRelationshipTarget()
		} else {
			// Simple reference
			fe.Payload = p.parseRelationshipTarget() // Allow feature chains, not just qualified names
			// `of Publish[1]` — an OwnedFeatureTyping may be followed by an
			// OwnedMultiplicity (SysML.xtext:1305).
			if p.at(lexer.LBracket) {
				fe.PayloadMultiplicity = p.parseMultiplicity()
			}
		}
	}

	switch {
	case p.acceptKeyword("from"):
		if fe == nil {
			fe = &ast.FlowEnds{}
		}
		fe.From = p.parseRelationshipTarget() // Allow feature chains
		p.parseFlowTo(fe)
	case !hasOf && p.atName():
		// Shorthand `x to y`.
		fe = &ast.FlowEnds{}
		fe.From = p.parseRelationshipTarget() // Allow feature chains
		p.parseFlowTo(fe)
	}
	if fe != nil {
		fe.NodeSpan = p.spanFrom(start)
		u.FlowEnds = fe
	}
}

// parseFlowTo consumes the `to <end>` tail of a flow, recording a diagnostic if
// `to` is absent.
func (p *Parser) parseFlowTo(fe *ast.FlowEnds) {
	if p.acceptKeyword("to") {
		fe.To = p.parseRelationshipTarget() // Allow feature chains
		return
	}
	p.error(p.peek().Span, "expected 'to' between flow ends")
}

// atTransitionEnds reports whether the `transition` at the cursor states its
// ends — `first <source>` or `<source> to <target>` — as opposed to declaring a
// transition feature without them (`transition t : Signalling;`). The
// transition's own name, when it has one, stands between the keyword and the
// ends.
func (p *Parser) atTransitionEnds() bool {
	i := 1
	// Only the `first` spelling can carry a name of the transition's own; in the
	// `to` spelling the first name is the source itself.
	if (p.peekN(i).Kind == lexer.Identifier || p.peekN(i).Kind == lexer.UnrestrictedName) &&
		p.peekIsKeyword(i+1, "first") {
		i++
	}
	if p.peekIsKeyword(i, "first") {
		return true
	}
	// `<source> to <target>`, where the source may be a qualified name and, as a
	// state name, may be spelled with a keyword.
	for {
		switch p.peekN(i).Kind {
		case lexer.Identifier, lexer.UnrestrictedName, lexer.Keyword:
		default:
			return false
		}
		if p.peekN(i+1).Kind == lexer.ColonColon {
			i += 2
			continue
		}
		return p.peekIsKeyword(i+1, "to")
	}
}

// atRelationshipKeyword checks if current token is a relationship keyword (redefines, subsets, etc.).
func (p *Parser) atRelationshipKeyword() bool {
	if t := p.peek(); t.Kind == lexer.Keyword {
		if _, ok := relationshipKeywords[t.KeywordID]; ok {
			return true
		}
		// Special multi-word keywords
		if t.KeywordID == "defined" || t.KeywordID == "inverse" || t.KeywordID == "featured" {
			return true
		}
		// `typed` states a relationship only in `typed by`; elsewhere it is a name.
		if t.KeywordID == "typed" {
			n := p.peekN(1)
			return n.Kind == lexer.Keyword && n.KeywordID == "by"
		}
	}
	return false
}

// relationshipClauseKind consumes the operator/keyword that begins a
// relationship clause and returns its kind. Reports ok=false (consuming
// nothing) when the current token does not begin a relationship clause.
func (p *Parser) relationshipClauseKind(isUsage bool) (ast.RelationshipKind, bool) {
	if t := p.peek(); t.Kind == lexer.Keyword {
		if k, ok := relationshipKeywords[t.KeywordID]; ok {
			p.advance()
			// 'disjoint' requires 'from' keyword after it
			if k == ast.RelDisjoint {
				p.expect2Keyword("from")
			}
			return k, true
		}
		// `typed by` is the long spelling of ':' (KerML.xtext TypedBy:600).
		if t.KeywordID == "defined" ||
			(t.KeywordID == "typed" && p.peekN(1).KeywordID == "by") {
			p.advance()
			p.expect2Keyword("by")
			return ast.RelTyping, true
		}
		if t.KeywordID == "inverse" {
			p.advance()
			p.expect2Keyword("of")
			return ast.RelInverseOf, true
		}
		// `featured by T` states the types a feature is featured by, and is
		// reached from FeatureDeclaration alone (KerML.xtext:569, 659).
		if t.KeywordID == "featured" {
			p.advance()
			p.expect2Keyword("by")
			return ast.RelFeaturedBy, true
		}
	}
	switch p.peek().Kind {
	case lexer.Colon:
		p.advance()
		return ast.RelTyping, true
	case lexer.ColonGt:
		p.advance()
		if isUsage {
			return ast.RelSubsets, true
		}
		return ast.RelSpecializes, true
	case lexer.ColonGtGt:
		p.advance()
		return ast.RelRedefines, true
	case lexer.ColonColonGt:
		p.advance()
		return ast.RelReferences, true
	case lexer.EqGt:
		p.advance()
		return ast.RelCrosses, true
	}
	return 0, false
}

// parseMultiplicity parses `[ lower ( .. upper )? ]` when a `[` is present.
func (p *Parser) parseMultiplicity() *ast.Multiplicity {
	if p.peek().Kind != lexer.LBracket {
		return nil
	}
	start := p.peek().Span.Offset
	p.advance() // '['
	m := &ast.Multiplicity{}
	m.Lower = p.parseMultiplicityBound()
	if p.accept2(lexer.DotDot) {
		m.IsRange = true
		m.Upper = p.parseMultiplicityBound()
	}
	p.expect(lexer.RBracket, "expected ']' to close multiplicity")
	m.NodeSpan = p.spanFrom(start)
	return m
}

// parseMultiplicityBound parses a single bound: `*` (infinity) or an expression
// above range precedence, so the multiplicity's own `..` is not read as a range.
func (p *Parser) parseMultiplicityBound() ast.Node {
	if p.peek().Kind == lexer.Star {
		star := p.peek()
		p.advance()
		inf := &ast.LiteralInfinity{}
		inf.NodeSpan = star.Span
		return inf
	}
	// MultiplicityExpressionMember (KerML.xtext) admits a literal or a feature
	// reference, so a prefix operator (`-1`) is a syntax error, not a negative bound.
	if p.atPrefixOperator() {
		op := p.peek()
		p.error(op.Span, fmt.Sprintf("a multiplicity bound cannot start with '%s': "+
			"a bound is a literal or a feature name (KerML.xtext MultiplicityExpressionMember)",
			p.src.Text(op.Span)))
	}
	return p.parseBinary(precAdditive)
}

// atMetadataIdentification reports whether an identification is declared before
// the metadata typing, which the typing keyword (`:`, `defined by`, `typed by`)
// marks: `@ m : Meta;` names the usage where `@ Meta;` only types it.
func (p *Parser) atMetadataIdentification() bool {
	if p.at(lexer.Lt) {
		return true
	}
	off := 0
	switch p.peek().Kind {
	case lexer.Identifier, lexer.UnrestrictedName, lexer.Keyword:
		off = 1
	}
	switch t := p.peekN(off); t.Kind {
	case lexer.Colon:
		return true
	case lexer.Keyword:
		if t.KeywordID != "defined" && t.KeywordID != "typed" {
			return false
		}
		n := p.peekN(off + 1)
		return n.Kind == lexer.Keyword && n.KeywordID == "by"
	}
	return false
}

// parseMetadataUsage parses one `@` metadata usage (SysML v2 MetadataUsage):
// `@Type;`, or `@Type { prop = value; }` binding its features. Each is a member
// of its own rather than a prefix of the declaration after it.
func (p *Parser) parseMetadataUsage(start int) *ast.PrefixMetadata {
	p.advance() // '@'

	// A declared identification precedes the typing, and may be absent from it:
	// `@ m : Meta;`, `@ <sn> : Meta;`, `@ : Meta;`.
	var ident ast.Identification
	if p.atMetadataIdentification() {
		ident = p.parseIdentification()
		if !p.accept2(lexer.Colon) {
			p.advance() // 'defined' | 'typed'
			p.advance() // 'by'
		}
	}

	metaType := p.parseQualifiedName()
	if metaType == nil {
		p.error(p.peek().Span, "expected metadata type after '@'")
		return nil
	}
	pm := &ast.PrefixMetadata{Ident: ident, Type: metaType}

	// `about` names the elements the usage annotates (SysML.xtext:145-147).
	if p.acceptKeyword("about") {
		pm.About = p.parseQualifiedNameList()
		if len(pm.About) == 0 {
			p.error(p.peek().Span, "expected an annotated element after 'about'")
		}
	}

	if p.at(lexer.LBrace) {
		p.advance() // '{'
		var body []ast.Node
		leave := p.pushBodyContext(bodyOther)
		for !p.at(lexer.RBrace) && !p.atEOF() {
			if m := p.parseBodyMember(); m != nil {
				body = append(body, m)
				continue
			}
			expr := p.ParseExpression()
			p.accept2(lexer.Semicolon)
			body = append(body, expr)
		}
		leave()
		p.expect(lexer.RBrace, "expected '}' after metadata body")
		pm.Body = body
		pm.HasBody = true
	} else {
		// A usage is a member of its own, so it ends here rather than annotating
		// whatever follows it — `#Type` is the prefix spelling.
		p.expectSemicolon("a metadata usage")
	}

	pm.NodeSpan = p.spanFrom(start)
	return pm
}
