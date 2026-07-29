package langs

import (
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"

	"github.com/ieshan/go-code-chunker/chunker"
)

var goLanguage = sync.OnceValue(func() chunker.LanguageConfig {
	return chunker.LanguageConfig{
		Name:       "go",
		Extensions: []string{".go"},
		Grammar:    sitter.NewLanguage(golang.Language()),
		NodeKinds: chunker.NodeKindSet{
			// type_spec is included alongside type_declaration so that when an oversized
			// struct is recursed into, extractParentName lands on type_spec (which has a
			// direct "name" field) rather than type_declaration (which does not).
			TopLevel:  []string{"function_declaration", "method_declaration", "type_declaration", "type_spec", "var_declaration", "const_declaration"},
			Nested:    []string{"field_declaration"},
			NameField: "name",
		},
		Edges: chunker.EdgeRules{
			Rules: []chunker.EdgeRule{
				{Kind: "call_expression", Edge: chunker.EdgeCall, TargetField: "function"},
				{Kind: "import_spec", Edge: chunker.EdgeImport},
				// Go has no inheritance; struct and interface embedding is the
				// equivalent relationship. An embedded field_declaration has a
				// type but no field_identifier name.
				{Kind: "type_identifier", Edge: chunker.EdgeReference},
			},
			Qualified: map[string]chunker.QualifiedFields{
				"selector_expression": {Qualifier: "operand", Name: "field"},
			},
			ImportModuleKinds: []string{"interpreted_string_literal"},
			ImportAliasKinds:  []string{"package_identifier"},
		},
	}
})

// GoLanguage returns the LanguageConfig for Go source files.
func GoLanguage() chunker.LanguageConfig {
	return goLanguage()
}
