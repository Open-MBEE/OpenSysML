package repl

import (
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// modesFile declares an enumeration, a package-level sequence of its literals and
// calcs that compare a parameter against a literal written in the calc's own file.
const modesFile = `package Modes {
    private import ScalarValues::*;
    private import SequenceFunctions::includes;
    private import ControlFunctions::collect;
    private import RealFunctions::sum;
    enum def Maturity { heritage; modified; newDesign; }
    attribute flown : Maturity[*] = (Maturity::heritage, Maturity::modified);
    calc def IsModified { in m : Maturity; return : Boolean = m == Maturity::modified; }
    calc def HasModified { in ms : Maturity[*]; return : Boolean = includes(ms, Maturity::modified); }
    part def Unit {
        attribute maturities : Maturity[*] default = flown;
        attribute cost : Real;
    }
    calc def CostIfModified {
        in u : Unit;
        return : Real = if includes(u.maturities, Maturity::modified) ? u.cost else 0.0;
    }
    calc def UnitsCostIfModified {
        in units : Unit[*];
        return : Real = sum(units->collect { in u : Unit; CostIfModified(u) });
    }
}
`

// fleetFile reads the literals of modesFile from another file: through a
// redefinition, through the package-level default, and through the calcs.
const fleetFile = `package Fleet {
    private import ScalarValues::*;
    private import Modes::*;
    part def Pump :> Unit {
        attribute :>> maturities = (Maturity::modified);
        attribute :>> cost = 10.0;
    }
    part def Heater :> Unit {
        attribute :>> cost = 260.0;
    }
    part pumps : Pump[2];
    part heaters : Heater[1];
    attribute pumpCost : Real = UnitsCostIfModified(pumps);
    attribute heaterCost : Real = UnitsCostIfModified(heaters);
    attribute fleetCost : Real = pumpCost + heaterCost;
}
`

const otherFile = "package Other { part def Box; }\n"

// loadFiles loads the files in the order given, each written under one directory.
func loadFiles(t *testing.T, files ...string) *Session {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, len(files))
	for i, content := range files {
		paths[i] = writeFile(t, filepath.Join(dir, string(rune('a'+i))+".sysml"), content)
	}
	s := NewSession()
	if _, err := s.LoadPaths(paths); err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	return s
}

// A qualified enumeration literal written as a %calc argument is the literal the
// calc's body names, whichever file was loaded last: the prompt resolves the
// argument from the last file's scope, through the index, and the index holds the
// defining file's own tree rather than a second one built from its AST.
func TestCalcEnumArgumentIsTheLiteralOfAnEarlierFile(t *testing.T) {
	for name, files := range map[string][]string{
		"defining file alone": {modesFile},
		"defining file last":  {otherFile, modesFile},
		"defining file first": {modesFile, otherFile},
		"defining file amid":  {otherFile, modesFile, fleetFile, otherFile},
	} {
		t.Run(name, func(t *testing.T) {
			s := loadFiles(t, files...)
			wants(t, run(t, s, "%calc Modes::IsModified Modes::Maturity::modified"), "= [true]")
			wants(t, run(t, s, "%calc Modes::IsModified Modes::Maturity::heritage"), "= [false]")
			wants(t, run(t, s, "%calc Modes::HasModified((Modes::Maturity::heritage, Modes::Maturity::modified))"), "= [true]")
			wants(t, run(t, s, "%calc Modes::HasModified((Modes::Maturity::heritage, Modes::Maturity::newDesign))"), "= [false]")
		})
	}
}

// A literal reached through a default declared in another file, through a
// redefinition and through a calc argument is one value under `includes`, so a
// rollup over the units of each file, and the sum of two such rollups, is complete.
func TestEnumLiteralsReadAcrossFilesAreOneValue(t *testing.T) {
	s := loadFiles(t, modesFile, fleetFile, otherFile)
	wants(t, run(t, s, "%eval Fleet::pumpCost"), "= 20.0")
	wants(t, run(t, s, "%eval Fleet::heaterCost"), "= 260.0")
	wants(t, run(t, s, "%eval Fleet::fleetCost"), "= 280.0")
	wants(t, run(t, s, "%calc Modes::UnitsCostIfModified Fleet::heaters"), "= [260.0]")
	wants(t, run(t, s, "%calc Modes::HasModified Fleet::heaters.maturities"), "= [true]")
}

// The index declares a loaded file's names through the file's own scope tree, so
// the symbol the index answers and the one the document's tree holds are the same.
func TestSessionIndexHoldsEachDocumentsOwnTree(t *testing.T) {
	s := loadFiles(t, modesFile, otherFile)
	idx := s.symbolIndex()
	if idx == nil {
		t.Fatal("no index")
	}
	indexed := idx.Declaring("Modes::Maturity::modified")
	if indexed == nil {
		t.Fatal("index does not declare Modes::Maturity::modified")
	}
	var declared *symbols.Symbol
	for _, doc := range s.sessionDocs() {
		pkg, ok := doc.Scope.LookupLocal("Modes")
		if !ok {
			continue
		}
		enum, ok := doc.Scope.ChildFor(pkg.Decl).LookupLocal("Maturity")
		if !ok {
			t.Fatal("Modes declares no Maturity")
		}
		declared, _ = doc.Scope.ChildFor(pkg.Decl).ChildFor(enum.Decl).LookupLocal("modified")
	}
	if declared == nil {
		t.Fatal("no loaded document's tree declares Modes::Maturity::modified")
	}
	if indexed != declared {
		t.Fatalf("the index holds a symbol of its own for Modes::Maturity::modified (%p) rather than the document's (%p)", indexed, declared)
	}
}
