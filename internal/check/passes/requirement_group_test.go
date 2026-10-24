package passes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// requirementGroup is a package of n bare requirements and one that requires
// them all; verificationObjective is the same n under `objective { verify … }`.
func requirementGroup(n int) string {
	var src strings.Builder
	src.WriteString("package P {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&src, "\trequirement r%d;\n", i)
	}
	src.WriteString("\trequirement grp {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&src, "\t\trequire r%d;\n", i)
	}
	src.WriteString("\t}\n}\n")
	return src.String()
}

func verificationObjective(n int) string {
	var src strings.Builder
	src.WriteString("package P {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&src, "\trequirement r%d;\n", i)
	}
	src.WriteString("\tverification def V {\n\t\tobjective {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&src, "\t\t\tverify r%d;\n", i)
	}
	src.WriteString("\t\t}\n\t}\n}\n")
	return src.String()
}

func analyzeDiags(src string) []diag.Diagnostic {
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	return Analyze("<t>", root, nil, idx)
}

// requirementGroupShapeEnv names the shape a re-run of the test binary validates
// on its own, so a validation that does not finish is killed, not left running.
const requirementGroupShapeEnv = "OPENSYSML_TEST_REQUIREMENT_GROUP_SHAPE"

// Each `require r` and `verify r` member takes its short name from the
// requirement it references, and that reference resolves through the scope
// asking for the siblings' short names: validating a group of n members must
// stay linear in n, not derive each sibling's name once per other sibling.
func TestRequirementGroupValidatesInLinearTime(t *testing.T) {
	const n = 14
	shapes := map[string]string{"require": requirementGroup(n), "verify": verificationObjective(n)}
	if shape := os.Getenv(requirementGroupShapeEnv); shape != "" {
		if diags := analyzeDiags(shapes[shape]); len(diags) != 0 {
			t.Fatalf("expected no diagnostics, got %v", diags)
		}
		return
	}
	for shape := range shapes {
		t.Run(shape, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run", "^"+t.Name()[:strings.Index(t.Name(), "/")]+"$", "-test.count", "1")
			cmd.Env = append(os.Environ(), requirementGroupShapeEnv+"="+shape)
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("validating a group of %d members did not finish in 60s", n)
			}
			if err != nil {
				t.Fatalf("validating a group of %d members: %v\n%s", n, err, out)
			}
		})
	}
}
