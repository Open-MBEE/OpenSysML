package lsp

import (
	"fmt"
	"math"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// hoverMemberLimitKey is the setting capping how many members a hover
// summarizes; 0 leaves the summary out.
const hoverMemberLimitKey = "hoverMemberLimit"

// hoverInheritedMembersKey is the setting that adds the members an element
// inherits to the summary, under the type each comes from.
const hoverInheritedMembersKey = "hoverInheritedMembers"

// defaultHoverMemberLimit is how many members a hover summarizes unless told otherwise.
const defaultHoverMemberLimit = 10

// applyHoverSettings sets the member summary's limit and whether it lists inherited
// members to what a payload asks; a setting absent or of another shape is left alone.
func (s *Server) applyHoverSettings(payload any) {
	limit, hasLimit := hoverMemberLimitSetting(payload)
	inherited, hasInherited := false, false
	if value, ok := sysmlSetting(payload, hoverInheritedMembersKey); ok {
		inherited, hasInherited = boolSetting(value)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if hasLimit {
		s.hoverMemberLimit = limit
	}
	if hasInherited {
		s.hoverInheritedMembers = inherited
	}
}

// hoverMemberLimitSetting reads the member limit: a whole number, not negative.
func hoverMemberLimitSetting(payload any) (int, bool) {
	value, ok := sysmlSetting(payload, hoverMemberLimitKey)
	if !ok {
		return 0, false
	}
	number, ok := value.(float64)
	if !ok || number < 0 || number > math.MaxInt32 || number != math.Trunc(number) {
		return 0, false
	}
	return int(number), true
}

// memberSection is one titled run of member lines in a hover; the members an
// element owns go untitled.
type memberSection struct {
	origin string
	lines  []string
}

// memberSummary is the summary of sym's members a hover shows.
type memberSummary struct {
	sections []memberSection
	// omitted counts the members the limit left out.
	omitted int
}

// memberSummaryOf summarizes the members of sym up to the configured limit: those
// it owns, then, when the setting asks, those it inherits grouped by their origin.
func (s *Server) memberSummaryOf(sym *symbols.Symbol) memberSummary {
	s.mu.Lock()
	limit, inherited := s.hoverMemberLimit, s.hoverInheritedMembers
	s.mu.Unlock()
	if limit == 0 {
		return memberSummary{}
	}
	members := s.ws.MemberSummaryOf(sym, inherited)
	var summary memberSummary
	if len(members) > limit {
		summary.omitted = len(members) - limit
		members = members[:limit]
	}
	sectionOf := map[*symbols.Symbol]int{}
	for _, member := range members {
		index, ok := sectionOf[member.Origin]
		if !ok {
			index = len(summary.sections)
			sectionOf[member.Origin] = index
			section := memberSection{}
			if member.Origin != nil {
				section.origin = s.ws.FQNOf(member.Origin)
			}
			summary.sections = append(summary.sections, section)
		}
		summary.sections[index].lines = append(summary.sections[index].lines, memberLine(member))
	}
	return summary
}

// memberLine writes a member as its declaration would open: visibility,
// direction, kind, name, type and multiplicity.
func memberLine(member model.Member) string {
	var b strings.Builder
	switch member.Symbol.Visibility {
	case ast.VisibilityPrivate:
		b.WriteString("private ")
	case ast.VisibilityProtected:
		b.WriteString("protected ")
	}
	if member.Direction != ast.DirNone {
		b.WriteString(member.Direction.String())
		b.WriteByte(' ')
	}
	b.WriteString(member.Symbol.Notation())
	b.WriteByte(' ')
	b.WriteString(source.NameText(member.Name))
	if types := memberTypeText(member); types != "" {
		b.WriteString(" : ")
		b.WriteString(types)
	}
	b.WriteString(member.Multiplicity)
	return b.String()
}

// memberTypeText is the types a member is written with, else those it takes from the features
// it specializes; the enumeration an enumerated value is implicitly typed by goes unsaid.
func memberTypeText(member model.Member) string {
	if written := declaredTypeTexts(member.Symbol); len(written) > 0 {
		return strings.Join(written, ", ")
	}
	owner := member.Symbol.Owner()
	enumerated := owner != nil && owner.Kind == symbols.SymbolEnumerationDef
	var names []string
	for _, typ := range member.Types {
		if !(enumerated && typ == owner) && typ.Named() {
			names = append(names, source.NameText(typ.Name))
		}
	}
	return strings.Join(names, ", ")
}

// render writes the summary as Markdown, or as plain text; "" for no members.
func (m memberSummary) render(markdown bool) string {
	var blocks []string
	for _, section := range m.sections {
		title := "Members:"
		if section.origin != "" {
			title = "Inherited from " + section.origin + ":"
			if markdown {
				title = "Inherited from `" + section.origin + "`:"
			}
		}
		lines := strings.Join(section.lines, "\n")
		if markdown {
			blocks = append(blocks, title, "```sysml\n"+lines+"\n```")
		} else {
			blocks = append(blocks, title+"\n"+lines)
		}
	}
	if m.omitted == 1 {
		blocks = append(blocks, "… 1 more member")
	} else if m.omitted > 1 {
		blocks = append(blocks, fmt.Sprintf("… %d more members", m.omitted))
	}
	return strings.Join(blocks, "\n\n")
}
