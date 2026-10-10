package migrate_test

import (
	"strings"
	"testing"
)

// A generic table over Classifier, Type, Namespace or PackageableElement lists
// the actors and use cases of its scope, which migrate to part and use case
// usages, by name — never the use-case-typed or actor-typed properties and
// include usages that share their v2 type. Actor and UseCase tables list only
// those usages.
func TestClassifierTablesListActorAndUseCaseUsages(t *testing.T) {
	s := session(t, migrateFixtureFile(t, "classifier_usages"))

	for _, name := range []string{"Classifiers", "Types", "Namespaces", "Packageable Elements"} {
		got := rows(t, s, "Tables::'"+name+" Rows'")
		wantInOrder(t, name+" rows", got,
			"returned 4 rows",
			"Operations::Console\n", "Operations::Focus\n", "Operations::Operator\n", "Operations::Scan\n")
		for _, feature := range []string{"Console::current", "Console::user", "Scan::"} {
			if strings.Contains(got, feature) {
				t.Errorf("%s lists the owned feature %s:\n%s", name, feature, got)
			}
		}
	}

	actors := rows(t, s, "Tables::'Actors Rows'")
	wantInOrder(t, "Actors rows", actors, "returned 1 row", "Operations::Operator\n")

	cases := rows(t, s, "Tables::'Use Cases Rows'")
	wantInOrder(t, "Use Cases rows", cases, "returned 2 rows", "Operations::Focus\n", "Operations::Scan\n")
}

// Actor and UseCase tables over a model with neither list no rows, rather than
// naming nothing.
func TestEmptyActorAndUseCaseTablesExecute(t *testing.T) {
	s := session(t, migrateFixtureFile(t, "empty_classifier_tables"))
	for _, name := range []string{"Actors", "Use Cases"} {
		got := rows(t, s, "Tables::'"+name+" Rows'")
		wantInOrder(t, name+" rows", got, "returned 0 rows")
	}
}
