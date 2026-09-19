// Package doccountstest writes a small tree the suite figures can be counted
// from, so the generator's tests and the command's share one fixture.
package doccountstest

import (
	"os"
	"path/filepath"
)

// TB is the part of *testing.T the writers need, so this package links no `testing`.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Suite is what counting the fixture tree yields, spelt as the blocks print it.
type Suite struct {
	ConformanceCases   int
	Robustness         int
	GRPCConformance    int
	GRPCRobustness     int
	GoldenASTs         int
	NegativeTable      int
	TestFunctions      string
	ConformanceSummary string
	TraceSummary       string
}

// Expected is the census of the tree WriteSuiteFixture writes.
var Expected = Suite{
	ConformanceCases:   7,
	Robustness:         7,
	GRPCConformance:    2,
	GRPCRobustness:     3,
	GoldenASTs:         3,
	NegativeTable:      3,
	TestFunctions:      "10",
	ConformanceSummary: "7 conformance cases (all passing: calc×4, and one each of action, send and state)",
	TraceSummary:       "3 golden execution traces under the default schedule (calc×2, action×1), and 1 more `.trace.golden` files",
}

// WriteSuiteFixture writes the fixture tree under root.
func WriteSuiteFixture(t TB, root string) {
	t.Helper()
	conformance := "internal/core/runtime/testdata/conformance/"
	for _, name := range []string{"calc_a", "calc_b", "calc_c", "calc_d", "action_a", "state_a", "send_a"} {
		Write(t, root, conformance+name+".sysml", "package P;\n")
		Write(t, root, conformance+name+".expected.json", "{}\n")
	}
	Write(t, root, conformance+"calc_a.expected.json", `{"outcomes": [{}, {}]}`+"\n")
	Write(t, root, conformance+"calc_a.check.expected.json", "{}\n")
	Write(t, root, conformance+"calc_a.trace.golden", "trace\n")
	Write(t, root, conformance+"calc_b.trace.golden", "trace\n")
	Write(t, root, conformance+"action_a.trace.golden", "trace\n")
	Write(t, root, conformance+"calc_a.declared.trace.golden", "trace\n")
	Write(t, root, conformance+"known_failures.txt", "# none\n")

	parse := "tests/parser/testdata/parse/"
	for _, name := range []string{"a.sysml", "b.sysml", "c.kerml"} {
		Write(t, root, parse+name, "package P;\n")
		Write(t, root, parse+name[:len(name)-len(filepath.Ext(name))]+".golden", "AST\n")
	}
	Write(t, root, parse+"README.md", "not a fixture\n")

	Write(t, root, "tests/parser/negative_test.go", `package parser_test

import "testing"

func TestNegative(t *testing.T) {
	cases := []struct{ name string }{{"a"}, {"b"}, {"c"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { t.Run("nested", func(t *testing.T) {}) })
	}
}

func TestNegativeKerML(t *testing.T) {
	for _, tc := range []string{"a", "b"} {
		t.Run(tc, func(t *testing.T) {})
	}
}

func TestGolden(t *testing.T) {}
`)
	Write(t, root, "internal/core/parser/negative_test.go", `package parser

import "testing"

func TestOtherNegative(t *testing.T) {
	t.Run("only", func(t *testing.T) {})
}
`)
	Write(t, root, "internal/core/runtime/robustness_test.go", `package runtime

import "testing"

func TestRuntimeRobustness(t *testing.T) {
	for _, name := range setup() {
		if name == "" {
			t.Fatal("empty")
		}
	}
	t.Run("a", func(t *testing.T) {})
	t.Run("b", func(t *testing.T) {})
	for _, name := range []string{"c", "d", "e"} {
		t.Run(name, func(t *testing.T) {})
	}
}

func setup() []string { return nil }

func TestExecutionConformance(t *testing.T) {}

func helper(t *testing.T) {}

func Testlower(t *testing.T) {}
`)
	Write(t, root, "internal/core/runtime/robustness_signals_test.go", `package runtime

import "testing"

func TestRuntimeRobustnessSignals(t *testing.T) {
	t.Run("f", func(t *testing.T) {})
	t.Run("g", func(t *testing.T) {})
}
`)
	grpc := "tests/grpc/testdata/conformance/"
	Write(t, root, grpc+"a.expected.json", "{}\n")
	Write(t, root, grpc+"b.expected.json", "{}\n")
	Write(t, root, grpc+"b.check.expected.json", "{}\n")
	Write(t, root, "internal/grpc/robustness_test.go", `package grpc

import "testing"

func TestGRPCRobustness(t *testing.T) {
	t.Run("a", func(t *testing.T) {})
	t.Run("b", func(t *testing.T) {})
}
`)
	Write(t, root, "internal/grpc/robustness_streams_test.go", `package grpc

import "testing"

func TestGRPCRobustnessStreams(t *testing.T) {
	t.Run("c", func(t *testing.T) {})
}
`)
	Write(t, root, "internal/lsp/server_test.go", `package lsp

import "testing"

func TestHover(t *testing.T) {}
`)
	Write(t, root, "internal/skipped/testdata/ignored_test.go", "package ignored\n\nimport \"testing\"\n\nfunc TestIgnored(t *testing.T) {}\n")
	Write(t, root, "client/nested/go.mod", "module example.com/nested\n")
	Write(t, root, "client/nested/nested_test.go", "package nested\n\nimport \"testing\"\n\nfunc TestNested(t *testing.T) {}\n")
}

// Write writes content at path, relative to root, creating the directories.
func Write(t TB, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
