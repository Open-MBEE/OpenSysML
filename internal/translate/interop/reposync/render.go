package reposync

import (
	"fmt"
	"strings"
)

// Text renders the change set for a dry run: one line per entry, deltas
// indented under updates and conflicts, and a closing summary.
func (cs *ChangeSet) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "sync diff against %s\n", cs.Scope)
	for _, left := range cs.Uncarried {
		fmt.Fprintf(&b, "not carried by the repository, so not compared: %s (%d triple(s))\n", left.Property, left.Triples)
	}
	if len(cs.Changes) == 0 {
		b.WriteString("nothing to change: the model and the repository agree\n")
		return b.String()
	}
	for _, change := range cs.Changes {
		fmt.Fprintf(&b, "%-8s %s", change.Kind, change.ID)
		if change.Metaclass != "" {
			fmt.Fprintf(&b, " %s", change.Metaclass)
		}
		if change.QualifiedName != "" {
			fmt.Fprintf(&b, " %s", change.QualifiedName)
		}
		switch {
		case change.MintedID != "":
			fmt.Fprintf(&b, " (minted id %s)", change.MintedID)
		case change.Kind == KindDelete && change.RequiresConfirmation:
			b.WriteString(" (needs explicit confirmation)")
		case change.Conflict == ConflictMissingID:
			b.WriteString(" (the annotation names an id the repository branch no longer has)")
		case change.Conflict == ConflictRepositoryChanged:
			b.WriteString(" (changed in the repository since the last-seen commit)")
		}
		b.WriteByte('\n')
		for _, delta := range change.Deltas {
			fmt.Fprintf(&b, "         %s: %s -> %s\n",
				delta.Property, renderValues(delta.Repository), renderValues(delta.Local))
		}
	}
	fmt.Fprintf(&b, "%d create(s), %d update(s), %d delete(s), %d conflict(s)\n",
		cs.count(KindCreate), cs.count(KindUpdate), cs.count(KindDelete), cs.Conflicts())
	return b.String()
}

func renderValues(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}
