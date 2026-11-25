package libs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func sampleParseRecord() *ParseRecord {
	return &ParseRecord{
		Name: "a.sysml", Kind: source.KindSysML, Language: "sysml",
		AST: []byte{1, 2, 3}, RootID: 7,
		ParseDiags:    []parser.Diagnostic{{Span: source.Span{Offset: 2, Len: 3}, Message: "bad", Code: "syntax"}},
		ParseWarnings: []parser.Diagnostic{{Span: source.Span{Offset: 5, Len: 1}, Message: "warn", Code: "nonstandard-notation"}},
		Warnings:      []string{"converted"},
	}
}

// A parse key is a function of the document's name, language and bytes and of
// this build alone: changing any field, or asking in another format, misses.
func TestParseKeyIsContentAddressed(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	content := []byte("package A {}")
	key := c.ParseKey("a.sysml", "sysml", source.KindSysML, content)
	if again := c.ParseKey("a.sysml", "sysml", source.KindSysML, content); again != key {
		t.Fatalf("same inputs gave %q and %q", key, again)
	}
	for label, other := range map[string]string{
		"content":  c.ParseKey("a.sysml", "sysml", source.KindSysML, []byte("package A { }")),
		"name":     c.ParseKey("b.sysml", "sysml", source.KindSysML, content),
		"language": c.ParseKey("a.sysml", "kerml", source.KindSysML, content),
		"kind":     c.ParseKey("a.sysml", "sysml", source.KindKerML, content),
	} {
		if other == key {
			t.Errorf("%s produced the same key", label)
		}
	}
	// Delimiters keep a pair of short fields from spelling a pair of long ones.
	if c.ParseKey("a\x00b", "c", source.KindSysML, []byte("d")) == c.ParseKey("a", "b\x00c", source.KindSysML, []byte("d")) {
		t.Error("name and language were not length-delimited")
	}
	suffix := parseRecordSuffix
	if !strings.HasSuffix(key, suffix) {
		t.Fatalf("key %q does not end in the parse format %q", key, suffix)
	}
	newer := strings.TrimSuffix(key, suffix) + "-p" + strconv.Itoa(parseFormatVersion+1)
	if err := c.StoreParse(newer, sampleParseRecord()); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.LoadParse(key); ok {
		t.Error("a record of a newer parse format produced a hit")
	}
}

// A second Cache over the same directory — the stand-in for another process —
// reads back every field of the parse, not just the tree bytes.
func TestParseRecordSurvivesAnotherCache(t *testing.T) {
	dir := t.TempDir()
	writer := &Cache{dir: dir}
	key := writer.ParseKey("a.sysml", "sysml", source.KindSysML, []byte("package A {}"))
	want := sampleParseRecord()
	if err := writer.StoreParse(key, want); err != nil {
		t.Fatalf("StoreParse: %v", err)
	}
	got, ok := (&Cache{dir: dir}).LoadParse(key)
	if !ok {
		t.Fatal("the second cache missed the parse record")
	}
	if got.Name != want.Name || got.Kind != want.Kind || got.Language != want.Language ||
		got.RootID != want.RootID || string(got.AST) != string(want.AST) ||
		len(got.ParseDiags) != 1 || got.ParseDiags[0].Message != "bad" ||
		len(got.ParseWarnings) != 1 || got.ParseWarnings[0].Message != "warn" ||
		len(got.Warnings) != 1 || got.Warnings[0] != "converted" {
		t.Fatalf("the record came back changed: %+v", got)
	}
}

// Damage of any kind is a miss, never an error or a partial restore.
func TestParseRecordMissesAreBenign(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{dir: dir}
	key := c.ParseKey("a.sysml", "sysml", source.KindSysML, []byte("package A {}"))
	if _, ok := c.LoadParse(key); ok {
		t.Fatal("an absent record produced a hit")
	}
	if err := os.MkdirAll(c.parseRecordDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.parseRecordDir(), key+".idx"), []byte("not a gob stream"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rec, ok := c.LoadParse(key); ok || rec != nil {
		t.Fatal("an undecodable record produced a hit")
	}
	if _, err := os.Stat(filepath.Join(c.parseRecordDir(), key+".idx")); !os.IsNotExist(err) {
		t.Fatalf("the undecodable record survived: %v", err)
	}
	if err := c.StoreParse(key, sampleParseRecord()); err != nil {
		t.Fatalf("StoreParse after a damaged record: %v", err)
	}
	if _, ok := c.LoadParse(key); !ok {
		t.Fatal("a rewritten record was not found")
	}
}

// A single bit flip that changes a payload is a typed miss: the record is removed
// and a store rewrites it rather than serving corrupt data.
func TestParseRecordBitFlipsAreTypedMisses(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{dir: dir}
	key := c.ParseKey("a.sysml", "sysml", source.KindSysML, []byte("package A {}"))
	rec := sampleParseRecord()
	rec.AST = bytes.Repeat([]byte{0x5a}, 1300)
	if err := c.StoreParse(key, rec); err != nil {
		t.Fatalf("StoreParse: %v", err)
	}
	path := filepath.Join(c.parseRecordDir(), key+".idx")
	pristine, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pristine) < 1024 {
		t.Fatalf("the fixture record has only %d bytes", len(pristine))
	}
	stored := *rec
	stored.Digest = rec.Digest // StoreParse filed the digest with the payload
	served, survive := 0, 0
	for i := range pristine {
		flipped := append([]byte(nil), pristine...)
		flipped[i] ^= 0x01
		if err := os.WriteFile(path, flipped, 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := c.LoadParseChecked(key)
		if err == nil {
			// A gob byte-slice run can leave the decoded payload unchanged;
			// only a changed payload would be a wrong answer.
			if got == nil || !reflect.DeepEqual(*got, stored) {
				served++
				t.Errorf("byte %d: a changed payload was served as %+v", i, got)
			}
			continue
		}
		if !errors.Is(err, ErrDamagedParseRecord) || got != nil {
			served++
			t.Errorf("byte %d: the miss was not typed: %+v, %v", i, got, err)
		}
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			survive++
		}
	}
	if served > 0 || survive > 0 {
		t.Fatalf("%d bit flips served a wrong payload and %d damaged records survived", served, survive)
	}
	if err := c.StoreParse(key, rec); err != nil {
		t.Fatalf("rewriting after damage: %v", err)
	}
	if got, err := c.LoadParseChecked(key); err != nil || got == nil || got.RootID != rec.RootID {
		t.Fatalf("the rewritten record was not restored: %+v, %v", got, err)
	}
}

// A store that died mid-write leaves a temp file in parses/; the parse prune
// removes it, so a crashed writer cannot grow the directory without bound.
func TestPruneParsesRemovesCrashedTempFiles(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	if err := os.MkdirAll(c.parseRecordDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(c.parseRecordDir(), "deadbeef-p1.idx.tmp-123")
	if err := os.WriteFile(stale, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-maxIdleAge - time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	if err := c.StoreParse(c.ParseKey("a.sysml", "sysml", source.KindSysML, []byte("x")), sampleParseRecord()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("the crashed temp file survived the prune: %v", err)
	}
	if _, ok := c.LoadParse(c.ParseKey("a.sysml", "sysml", source.KindSysML, []byte("x"))); !ok {
		t.Fatal("the prune removed the parse record")
	}
}

// Parse and analysis records share a directory without either pruning the
// other: their key suffixes are distinct.
func TestParseAndModelRecordsCoexist(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{dir: dir}
	pkey := c.ParseKey("a.sysml", "sysml", source.KindSysML, []byte("x"))
	mkey := c.ModelKey("digest", "library")
	if pkey == mkey || strings.HasSuffix(pkey, modelRecordSuffix) || strings.HasSuffix(mkey, parseRecordSuffix) {
		t.Fatalf("parse key %q and model key %q share a suffix", pkey, mkey)
	}
	if err := c.StoreParse(pkey, sampleParseRecord()); err != nil {
		t.Fatal(err)
	}
	if err := c.StoreModel(mkey, &ModelRecord{Documents: []ModelDocument{{Name: "a.sysml"}}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.LoadParse(pkey); !ok {
		t.Error("the parse record was lost")
	}
	if _, ok := c.LoadModel(mkey); !ok {
		t.Error("the model record was lost")
	}
}
