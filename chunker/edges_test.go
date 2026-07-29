package chunker_test

import (
	"os"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"

	"github.com/ieshan/go-code-chunker/chunker"
	"github.com/ieshan/go-code-chunker/langs"
)

// goConfig mirrors langs.GoLanguage() so the chunker package can test edge
// extraction without importing langs (which would create an import cycle).
func goConfig() chunker.LanguageConfig {
	return chunker.LanguageConfig{
		Name:       "go",
		Extensions: []string{".go"},
		Grammar:    sitter.NewLanguage(golang.Language()),
		NodeKinds: chunker.NodeKindSet{
			TopLevel:  []string{"function_declaration", "method_declaration", "type_declaration", "type_spec"},
			Nested:    []string{"field_declaration"},
			NameField: "name",
		},
		Edges: chunker.EdgeRules{
			Rules: []chunker.EdgeRule{
				{Kind: "call_expression", Edge: chunker.EdgeCall, TargetField: "function"},
				{Kind: "import_spec", Edge: chunker.EdgeImport},
				{Kind: "type_identifier", Edge: chunker.EdgeReference},
			},
			Qualified: map[string]chunker.QualifiedFields{
				"selector_expression": {Qualifier: "operand", Name: "field"},
			},
			ImportModuleKinds: []string{"interpreted_string_literal"},
			ImportAliasKinds:  []string{"package_identifier"},
		},
	}
}

// noEdgeConfig is goConfig with edge extraction switched off.
func noEdgeConfig() chunker.LanguageConfig {
	cfg := goConfig()
	cfg.Edges = chunker.EdgeRules{}
	return cfg
}

func analyze(t *testing.T, cfg chunker.LanguageConfig, path, src string) chunker.Analysis {
	t.Helper()
	c, err := chunker.NewChunker([]chunker.LanguageConfig{cfg}, chunker.DefaultConfig())
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}
	res, err := c.Analyze(path, []byte(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return res
}

// findEdge returns the first edge matching kind, source, and target.
func findEdge(edges []chunker.Edge, kind chunker.EdgeKind, source, target string) (chunker.Edge, bool) {
	for _, e := range edges {
		if e.Kind == kind && e.Source == source && e.Target == target {
			return e, true
		}
	}
	return chunker.Edge{}, false
}

func countKind(edges []chunker.Edge, kind chunker.EdgeKind) int {
	n := 0
	for _, e := range edges {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

const edgeSrc = `package main

import (
	"fmt"
	str "strings"
)

type Circle struct {
	r float64
}

func (c Circle) Area() float64 {
	v := helper(c.r)
	fmt.Println(v)
	str.ToUpper("a")
	return v
}

func helper(f float64) float64 { return f * 2 }
`

func TestAnalyze_CallEdgeAttribution(t *testing.T) {
	res := analyze(t, goConfig(), "a.go", edgeSrc)

	e, ok := findEdge(res.Edges, chunker.EdgeCall, "Area", "helper")
	if !ok {
		t.Fatalf("expected call edge Area -> helper, got %+v", res.Edges)
	}
	if e.TargetQualifier != "" {
		t.Errorf("unqualified call should have empty qualifier, got %q", e.TargetQualifier)
	}
	if e.Line != 13 {
		t.Errorf("expected reference on line 13, got %d", e.Line)
	}
	if e.SourceLine != 12 {
		t.Errorf("expected enclosing definition to start on line 12, got %d", e.SourceLine)
	}
	if e.Language != "go" {
		t.Errorf("expected language go, got %q", e.Language)
	}
	if e.FilePath != "a.go" {
		t.Errorf("expected file path a.go, got %q", e.FilePath)
	}
}

func TestAnalyze_QualifiedCallSplitsReceiver(t *testing.T) {
	res := analyze(t, goConfig(), "a.go", edgeSrc)

	for _, tc := range []struct{ qualifier, target string }{
		{"fmt", "Println"},
		{"str", "ToUpper"},
	} {
		e, ok := findEdge(res.Edges, chunker.EdgeCall, "Area", tc.target)
		if !ok {
			t.Errorf("expected call edge Area -> %s", tc.target)
			continue
		}
		if e.TargetQualifier != tc.qualifier {
			t.Errorf("%s: expected qualifier %q, got %q", tc.target, tc.qualifier, e.TargetQualifier)
		}
	}
}

func TestAnalyze_ImportEdges(t *testing.T) {
	res := analyze(t, goConfig(), "a.go", edgeSrc)

	plain, ok := findEdge(res.Edges, chunker.EdgeImport, "", "fmt")
	if !ok {
		t.Fatalf("expected import edge for fmt, got %+v", res.Edges)
	}
	if plain.TargetQualifier != "" {
		t.Errorf("unaliased import should have empty qualifier, got %q", plain.TargetQualifier)
	}

	aliased, ok := findEdge(res.Edges, chunker.EdgeImport, "", "strings")
	if !ok {
		t.Fatal("expected import edge for strings")
	}
	if aliased.TargetQualifier != "str" {
		t.Errorf("expected alias str, got %q", aliased.TargetQualifier)
	}
	// Imports sit at file level and have no enclosing definition.
	if aliased.Source != "" || aliased.SourceLine != 0 {
		t.Errorf("expected file-level import, got source %q line %d", aliased.Source, aliased.SourceLine)
	}
}

func TestAnalyze_ReferenceEdgeSkipsSelfName(t *testing.T) {
	res := analyze(t, goConfig(), "a.go", edgeSrc)

	// type Circle's own type_identifier must not become a self-reference.
	if e, ok := findEdge(res.Edges, chunker.EdgeReference, "Circle", "Circle"); ok {
		t.Errorf("expected no self-reference edge, got %+v", e)
	}
	// A field's type is a genuine reference.
	if _, ok := findEdge(res.Edges, chunker.EdgeReference, "r", "float64"); !ok {
		t.Errorf("expected reference edge r -> float64, got %+v", res.Edges)
	}
}

func TestAnalyze_NestedSourceCarriesParent(t *testing.T) {
	res := analyze(t, goConfig(), "a.go", edgeSrc)

	e, ok := findEdge(res.Edges, chunker.EdgeReference, "r", "float64")
	if !ok {
		t.Fatal("expected reference edge from field r")
	}
	if e.SourceParent != "Circle" {
		t.Errorf("expected SourceParent Circle, got %q", e.SourceParent)
	}
}

func TestAnalyze_NoEdgeRulesYieldsNoEdges(t *testing.T) {
	res := analyze(t, noEdgeConfig(), "a.go", edgeSrc)

	if len(res.Edges) != 0 {
		t.Errorf("expected no edges when EdgeRules is zero, got %d", len(res.Edges))
	}
	if len(res.Chunks) == 0 {
		t.Error("expected chunks even when edge extraction is disabled")
	}
}

func TestAnalyze_ChunksMatchChunkFile(t *testing.T) {
	c, err := chunker.NewChunker([]chunker.LanguageConfig{goConfig()}, chunker.DefaultConfig())
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}

	chunks, err := c.ChunkFile("a.go", []byte(edgeSrc))
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	res, err := c.Analyze("a.go", []byte(edgeSrc))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(chunks) != len(res.Chunks) {
		t.Fatalf("chunk count differs: ChunkFile %d, Analyze %d", len(chunks), len(res.Chunks))
	}
	for i := range chunks {
		if chunks[i] != res.Chunks[i] {
			t.Errorf("chunk %d differs:\n ChunkFile %+v\n Analyze   %+v", i, chunks[i], res.Chunks[i])
		}
	}
}

func TestAnalyze_EmptySource(t *testing.T) {
	res := analyze(t, goConfig(), "a.go", "")

	if len(res.Chunks) != 0 || len(res.Edges) != 0 {
		t.Errorf("expected zero Analysis for empty source, got %+v", res)
	}
}

func TestAnalyze_UnsupportedLanguage(t *testing.T) {
	c, err := chunker.NewChunker([]chunker.LanguageConfig{goConfig()}, chunker.DefaultConfig())
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}
	if _, err := c.Analyze("a.zig", []byte("const x = 1;")); err != chunker.ErrUnsupportedLanguage {
		t.Errorf("expected ErrUnsupportedLanguage, got %v", err)
	}
}

func TestAnalyze_DeduplicatesRepeatedReferences(t *testing.T) {
	// The same call on the same line must be recorded once.
	src := `package main

func a() { b(); b() }

func b() {}
`
	res := analyze(t, goConfig(), "a.go", src)

	if got := countKind(res.Edges, chunker.EdgeCall); got != 1 {
		t.Errorf("expected 1 deduplicated call edge, got %d: %+v", got, res.Edges)
	}
}

func FuzzAnalyze(f *testing.F) {
	// Seed corpus from fixtures across the languages that declare edge rules.
	for _, seed := range []struct{ path, ext string }{
		{"../langs/testdata/go/fixture.go", "fixture.go"},
		{"../langs/testdata/python/fixture.py", "fixture.py"},
		{"../langs/testdata/javascript/fixture.js", "fixture.js"},
		{"../langs/testdata/typescript/fixture.ts", "fixture.ts"},
		{"../langs/testdata/tsx/fixture.tsx", "fixture.tsx"},
		{"../langs/testdata/ruby/fixture.rb", "fixture.rb"},
		{"../langs/testdata/c/fixture.c", "fixture.c"},
		{"../langs/testdata/cpp/fixture.cpp", "fixture.cpp"},
		{"../langs/testdata/bash/fixture.sh", "fixture.sh"},
		{"../langs/testdata/vue/fixture.vue", "fixture.vue"},
	} {
		if data, err := os.ReadFile(seed.path); err == nil {
			f.Add(seed.ext, string(data))
		}
	}
	f.Add("main.go", "package main\nfunc f() { g() }")
	f.Add("main.go", "import (")
	f.Add("main.go", "")
	f.Add("unknown.xyz", "anything")

	c, err := chunker.NewChunker(langs.AllLanguages(), chunker.DefaultConfig())
	if err != nil {
		f.Fatalf("NewChunker: %v", err)
	}

	f.Fuzz(func(t *testing.T, filePath string, src string) {
		res, err := c.Analyze(filePath, []byte(src))
		if err != nil {
			return
		}
		for _, e := range res.Edges {
			if e.Kind == "" {
				t.Error("edge has empty Kind")
			}
			if e.Target == "" {
				t.Errorf("%s edge has empty Target", e.Kind)
			}
			if e.Line < 1 {
				t.Errorf("%s edge Line must be >= 1, got %d", e.Kind, e.Line)
			}
			if e.SourceLine < 0 {
				t.Errorf("%s edge SourceLine must be >= 0, got %d", e.Kind, e.SourceLine)
			}
			if e.Source == "" && e.SourceLine != 0 {
				t.Errorf("edge without a source must have SourceLine 0, got %d", e.SourceLine)
			}
		}
	})
}

func TestAnalyze_EdgesSurviveTreeClose(t *testing.T) {
	// Edge fields are resolved to strings during the walk, so they must remain
	// readable after Analyze returns and the parse tree is freed.
	res := analyze(t, goConfig(), "a.go", edgeSrc)
	if len(res.Edges) == 0 {
		t.Fatal("expected edges")
	}
	for i, e := range res.Edges {
		if e.Target == "" {
			t.Errorf("edge %d has empty target after tree close: %+v", i, e)
		}
	}
}
