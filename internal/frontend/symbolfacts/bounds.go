package symbolfacts

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// BoundText renders a multiplicity bound; an unevaluable one renders empty.
func BoundText(b semantics.Bound) string {
	if !b.Known {
		return ""
	}
	if b.Infinite {
		return "*"
	}
	return strconv.FormatInt(b.Value, 10)
}
