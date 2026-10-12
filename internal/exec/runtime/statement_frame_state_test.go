package runtime

import (
	"strings"
	"testing"
)

func TestSnapshotRestoresBodyCellsInUnorderedStrands(t *testing.T) {
	rootValues := map[string]Value{"root": {Kind: ValNull}}
	rootCells := newBodyCells(rootValues, nil)
	rootCell := rootCells.cell("root")
	rootCell.binding = &bodyBinding{frozen: true}
	rootBinding := rootCell.binding

	blockValues := map[string]Value{"block": {Kind: ValNull}}
	blockCells := newBodyCells(blockValues, nil)
	blockCell := blockCells.cell("block")
	blockCell.binding = &bodyBinding{frozen: true}
	blockBinding := blockCell.binding

	engine := &engineFrame{engine: &stmtEngine{env: &stmtEnv{
		locals: rootValues, localCells: rootCells,
	}}}
	block := &blockFrame{locals: blockValues, cells: blockCells}
	run := &bodyRun{
		work: &statementWork{},
		cursor: []bodyFrame{&stmtListFrame{strands: []*stmtStrand{{
			cursor: []bodyFrame{engine, block},
		}}}},
	}

	capture := (&executorCaptures{}).captureBody(run)
	delete(rootValues, "root")
	rootCell.binding.frozen = false
	delete(blockValues, "block")
	blockCell.binding.frozen = false

	capture.restore()

	if value, ok := rootValues["root"]; !ok || value.Kind != ValNull {
		t.Fatalf("restored root local = %v, present %t; want null", value, ok)
	}
	if rootCell.binding == rootBinding || !rootCell.binding.frozen {
		t.Fatalf("restored root binding = %+v, same pointer %t; want a frozen clone",
			rootCell.binding, rootCell.binding == rootBinding)
	}
	if value, ok := blockValues["block"]; !ok || value.Kind != ValNull {
		t.Fatalf("restored block local = %v, present %t; want null", value, ok)
	}
	if blockCell.binding == blockBinding || !blockCell.binding.frozen {
		t.Fatalf("restored block binding = %+v, same pointer %t; want a frozen clone",
			blockCell.binding, blockCell.binding == blockBinding)
	}
}

func TestStatementFrameStateSpellingIncludesBodyBindingState(t *testing.T) {
	locals := map[string]Value{"value": {Kind: ValNull}}
	cells := newBodyCells(locals, nil)
	cell := cells.cell("value")
	cell.binding = &bodyBinding{}
	speller := &stateSpeller{}

	block := (&blockFrame{locals: locals, cells: cells}).spell(speller)
	if !strings.Contains(block, "[tracking]") {
		t.Fatalf("block state = %q, want tracking binding state", block)
	}
	cell.fv.Written = true
	block = (&blockFrame{locals: locals, cells: cells}).spell(speller)
	if !strings.Contains(block, "[written]") {
		t.Fatalf("block state = %q, want written binding state", block)
	}

	engine := (&engineFrame{engine: &stmtEngine{env: &stmtEnv{
		locals: locals, localCells: cells,
	}}}).spell(speller)
	if !strings.Contains(engine, "[written]") {
		t.Fatalf("engine state = %q, want written binding state", engine)
	}
}
