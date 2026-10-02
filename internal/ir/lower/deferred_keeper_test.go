package lower

import "testing"

// The accept a DeferredKeeper annotation marks lowers as the keeper of the
// signal it accepts, however the annotation is spelled; one without the
// annotation, or with one of a definition that merely shares its name, does not.
func TestAccept_KeeperIsReadByResolvedAnnotation(t *testing.T) {
	graph, err := weightedActionGraph(t, `
		import MigrationMetadata::DeferredKeeper;
		metadata def Keeper :> DeferredKeeper;
		item def Ping;
		#MigrationMetadata::DeferredKeeper action receive accept kept : Ping;
		#DeferredKeeper action imported accept kept : Ping;
		action spelled accept kept : Ping { @DeferredKeeper; }
		action plain accept kept : Ping;
		#Keeper action narrowed accept kept : Ping;
	`)
	if err != nil {
		t.Fatalf("ToActionGraphWith: %v", err)
	}
	for name, want := range map[string]bool{"receive": true, "imported": true, "spelled": true, "plain": false, "narrowed": false} {
		accept, ok := graph.Accepts[nodeNamed(t, graph, name)]
		if !ok {
			t.Fatalf("%s lowered no accept", name)
		}
		if accept.Keeper != want {
			t.Errorf("%s: Keeper = %v, want %v", name, accept.Keeper, want)
		}
	}
}

// A definition spelled like the library's, declared by the model itself, marks
// no keeper: detection is by the resolved definition, not the written name.
func TestAccept_KeeperIgnoresAHomonymousDefinition(t *testing.T) {
	graph, err := weightedActionGraph(t, `
		metadata def DeferredKeeper;
		item def Ping;
		#DeferredKeeper action receive accept kept : Ping;
	`)
	if err != nil {
		t.Fatalf("ToActionGraphWith: %v", err)
	}
	if accept := graph.Accepts[nodeNamed(t, graph, "receive")]; accept.Keeper {
		t.Errorf("an accept marked by the model's own DeferredKeeper lowered as a keeper")
	}
}
