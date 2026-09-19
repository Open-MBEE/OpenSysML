package queryplan

import "testing"

func TestCompileCoverageAndSetOperations(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Unsatisfied :> Query {
	in root : Element;
	WhereRelated(
		source = Descendants(source = root, maxDepth = 2),
		relationshipKind = "satisfaction",
		direction = "incoming",
		maxDepth = 1,
		exists = false
	)
}
calc def Defaulted :> Query {
	in root : Element;
	WhereRelated(OwnedElements(source = root), "verification", "incoming", 1)
}
calc def Gaps :> Query {
	in root : Element;
	Union(
		source = Except(source = OwnedElements(source = root), exclude = Defaulted(root = root)),
		other = Unsatisfied(root = root)
	)
}
`)
	tests := []struct {
		name      string
		operation Operation
		arguments []string
	}{
		{"Unsatisfied", OperationWhereRelated, []string{"source", "relationshipKind", "direction", "maxDepth", "exists"}},
		{"Defaulted", OperationWhereRelated, []string{"source", "relationshipKind", "direction", "maxDepth"}},
		{"Gaps", OperationUnion, []string{"source", "other"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program := fixture.compile(t, test.name)
			definitions := program.Definitions()
			expression := definitions[len(definitions)-1].Expression()
			if expression.Operation() != test.operation {
				t.Fatalf("operation = %s, want %s", expression.Operation(), test.operation)
			}
			args := expression.Arguments()
			if len(args) != len(test.arguments) {
				t.Fatalf("arguments = %+v, want %v", args, test.arguments)
			}
			for i, name := range test.arguments {
				if args[i].Name != name {
					t.Fatalf("argument %d = %q, want %q", i, args[i].Name, name)
				}
			}
		})
	}
	gaps := fixture.compile(t, "Gaps")
	definitions := gaps.Definitions()
	nested := definitions[len(definitions)-1].Expression().Arguments()[0].Value.Operation()
	if nested != OperationExcept {
		t.Fatalf("Union source operation = %s, want %s", nested, OperationExcept)
	}
}

func TestCompileValidatesCoverageAndSetOperationArguments(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def MissingKind :> Query {
	in root : Element;
	WhereRelated(source = root, direction = "incoming", maxDepth = 1)
}
calc def MissingExclude :> Query {
	in root : Element;
	Except(source = root)
}
calc def UnknownArgument :> Query {
	in root : Element;
	Union(source = root, other = root, distinct = true)
}
calc def DuplicateArgument :> Query {
	in root : Element;
	Except(source = root, exclude = root, exclude = root)
}
calc def TooFewPositional :> Query {
	in root : Element;
	WhereRelated(root, "satisfaction", "incoming")
}
calc def ExistsAsString :> Query {
	in root : Element;
	WhereRelated(source = root, relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, exists = "no")
}
calc def DepthAsBoolean :> Query {
	in root : Element;
	WhereRelated(root, "satisfaction", "incoming", true)
}
calc def ExcludeAsString :> Query {
	in root : Element;
	Except(source = root, exclude = "root")
}
`)
	kinds := []struct {
		name string
		kind ErrorKind
	}{
		{"MissingKind", ErrorMissingArgument},
		{"MissingExclude", ErrorMissingArgument},
		{"UnknownArgument", ErrorUnknownArgument},
		{"DuplicateArgument", ErrorDuplicateArgument},
		{"TooFewPositional", ErrorArgumentCount},
	}
	for _, test := range kinds {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planning := planningError(t, err, test.kind)
			if planning.Origin.Doc != "queries.sysml" {
				t.Fatalf("error origin = %+v", planning.Origin)
			}
		})
	}
	types := []struct {
		name      string
		parameter string
		expected  string
		actual    string
		argument  string
	}{
		{"ExistsAsString", "exists", "ScalarValues::Boolean", "ScalarValues::String", `"no"`},
		{"DepthAsBoolean", "maxDepth", "ScalarValues::Integer", "ScalarValues::Boolean", "true"},
		{"ExcludeAsString", "exclude", "KerML::Root::Element", "ScalarValues::String", `"root"`},
	}
	for _, test := range types {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planning := planningError(t, err, ErrorArgumentType)
			if planning.Parameter != test.parameter || planning.Expected != test.expected || planning.Actual != test.actual {
				t.Fatalf("argument type error = %+v", planning)
			}
			span := planning.Origin.Span
			if got := fixture.content[span.Offset:span.End()]; got != test.argument {
				t.Fatalf("argument origin = %q, want %q", got, test.argument)
			}
		})
	}
}
