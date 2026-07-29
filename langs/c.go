package langs

import (
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	c "github.com/tree-sitter/tree-sitter-c/bindings/go"

	"github.com/ieshan/go-code-chunker/chunker"
)

var cLanguage = sync.OnceValue(func() chunker.LanguageConfig {
	return chunker.LanguageConfig{
		Name:       "c",
		Extensions: []string{".c", ".h"},
		Grammar:    sitter.NewLanguage(c.Language()),
		NodeKinds: chunker.NodeKindSet{
			TopLevel:  []string{"function_definition", "declaration"},
			Nested:    []string{},
			NameField: "",
		},
		Edges: chunker.EdgeRules{
			Rules: []chunker.EdgeRule{
				{Kind: "call_expression", Edge: chunker.EdgeCall, TargetField: "function"},
				{Kind: "preproc_include", Edge: chunker.EdgeImport},
				{Kind: "type_identifier", Edge: chunker.EdgeReference},
			},
			// Both <stdio.h> and "local.h" forms; cleanModulePath strips the
			// surrounding brackets and quotes.
			ImportModuleKinds: []string{"system_lib_string", "string_content"},
		},
	}
})

// CLanguage returns the LanguageConfig for C source files (.c, .h).
// Symbol names are not extracted (NameField is empty) because C function names
// are nested inside declarator chains rather than at a direct "name" field.
func CLanguage() chunker.LanguageConfig {
	return cLanguage()
}
