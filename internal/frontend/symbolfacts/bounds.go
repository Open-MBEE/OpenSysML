package symbolfacts

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// BoundText formats a semantic bound as a decimal value, *, or an empty string.
func BoundText(b semantics.Bound) string {
	if !b.Known {
		return ""
	}
	if b.Infinite {
		return "*"
	}
	return strconv.FormatInt(b.Value, 10)
}
