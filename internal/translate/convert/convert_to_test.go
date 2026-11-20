package convert_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// pieces records each Write, failing from the write numbered failAt on.
type pieces struct {
	out            bytes.Buffer
	writes, failAt int
}

var errFull = errors.New("destination full")

func (p *pieces) Write(data []byte) (int, error) {
	p.writes++
	if p.failAt > 0 && p.writes >= p.failAt {
		return 0, errFull
	}
	return p.out.Write(data)
}

// ConvertTo writes what ConvertWith returns; one document's API JSON reaches the
// destination an element at a time, and a destination that fails stops it.
func TestConvertToStreamsOneDocument(t *testing.T) {
	text := []byte("package P { part def V; part car : V { attribute mass = 3; } }")
	for _, to := range []convert.Format{convert.FormatAPIJSON, convert.FormatTurtle, convert.FormatSysML} {
		want, err := convert.ConvertWith("p.sysml", text, convert.FormatSysML, to, convert.Options{})
		if err != nil {
			t.Fatal(err)
		}
		var got pieces
		if err := convert.ConvertTo(&got, "p.sysml", text, convert.FormatSysML, to, convert.Options{}); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.out.Bytes(), want) {
			t.Errorf("%s: ConvertTo wrote %d bytes that differ from ConvertWith's %d", to, got.out.Len(), len(want))
		}
		if to != convert.FormatAPIJSON {
			continue
		}
		var elements []json.RawMessage
		if err := json.Unmarshal(want, &elements); err != nil {
			t.Fatal(err)
		}
		if len(elements) < 2 || got.writes != len(elements)+1 {
			t.Errorf("api-json reached the destination in %d writes for %d elements, want one each and the closing", got.writes, len(elements))
		}
	}
	failing := pieces{failAt: 2}
	err := convert.ConvertTo(&failing, "p.sysml", text, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{})
	if !errors.Is(err, errFull) || failing.writes != 2 {
		t.Errorf("a failing destination: err %v after %d writes, want %v at the second", err, failing.writes, errFull)
	}
	if err := convert.ConvertTo(&pieces{}, "p.sysml", []byte("package {"), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{}); err == nil {
		t.Error("a document that does not parse converted")
	}
}
