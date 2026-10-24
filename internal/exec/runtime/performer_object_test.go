package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// performerObject instantiates the declaration a performer names and, for a
// path from it ("test::scenario.tug.sup"), walks to the nested object it reaches.
func performerObject(t *testing.T, ctx *Context, idx *symbols.Index, object string) *Instance {
	t.Helper()
	declaration, path, _ := strings.Cut(object, ".")
	self, err := ctx.Instantiate(oneSymbol(t, idx, declaration))
	if err != nil {
		t.Fatalf("instantiate %s: %v", declaration, err)
	}
	if path == "" {
		return self
	}
	nested, ok := ctx.Instance(objectAtPath(t, ctx, self, path))
	if !ok {
		t.Fatalf("%s: no object is held at %s", object, path)
	}
	return nested
}
