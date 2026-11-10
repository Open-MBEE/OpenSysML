package repl

import "testing"

// Every command of the table says what it does, so a host gating the prompt
// by effect never meets an unclassified command; aliases carry their command's.
func TestEveryMetaCommandHasAnEffect(t *testing.T) {
	byName := map[string]Effect{}
	for _, c := range metaCommandTable {
		if c.effect < Reads || c.effect > Executes {
			t.Errorf("%s has effect %d; classify it as Reads, Shapes, Writes or Executes", c.name, c.effect)
		}
		byName[c.name] = c.effect
	}
	for name, want := range map[string]Effect{
		"%help": Reads, "%list": Reads, "%print": Reads, "%tokens": Reads, "%tool": Reads,
		"%load": Shapes, "%clear": Shapes, "%seed": Shapes, cmdImport: Shapes,
		"%save": Writes, "%publish": Writes,
		"%eval": Executes, "%instantiate": Executes, "%action": Executes, "%solve": Executes, "%advance": Executes,
	} {
		if byName[name] != want {
			t.Errorf("%s has effect %d, want %d", name, byName[name], want)
		}
	}
	if byName["%exit"] != byName["%quit"] {
		t.Errorf("%%exit has effect %d, %%quit %d; an alias carries its command's", byName["%exit"], byName["%quit"])
	}
	if got, ok := CommandEffect("%eval"); !ok || got != Executes {
		t.Errorf("CommandEffect(%%eval) = %d, %v", got, ok)
	}
	if _, ok := CommandEffect("%nothing"); ok {
		t.Error("CommandEffect answers an unknown command")
	}
}
