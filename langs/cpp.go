package langs

import (
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"

	"github.com/ieshan/go-code-chunker/chunker"
)

var cppLanguage = sync.OnceValue(func() chunker.LanguageConfig {
	return chunker.LanguageConfig{
		Name:       "cpp",
		Extensions: []string{".cpp", ".cc", ".cxx", ".hpp"},
		Grammar:    sitter.NewLanguage(cpp.Language()),
		// function_definition appears in both TopLevel and Nested intentionally:
		// it represents both free functions (top-level boundary) and class member
		// functions (nested construct). The chunker resolves the distinction via
		// AST parent-chain walk at atom collection time.
		NodeKinds: chunker.NodeKindSet{
			TopLevel:  []string{"function_definition", "class_specifier", "struct_specifier", "declaration"},
			Nested:    []string{"function_definition"},
			NameField: "name",
		},
		Edges: chunker.EdgeRules{
			Rules: []chunker.EdgeRule{
				{Kind: "call_expression", Edge: chunker.EdgeCall, TargetField: "function"},
				{Kind: "preproc_include", Edge: chunker.EdgeImport},
				{Kind: "base_class_clause", Edge: chunker.EdgeInherit, Descend: true},
				{Kind: "type_identifier", Edge: chunker.EdgeReference},
			},
			Qualified: map[string]chunker.QualifiedFields{
				"field_expression":     {Qualifier: "argument", Name: "field"},
				"qualified_identifier": {Qualifier: "scope", Name: "name"},
			},
			NameKinds:         []string{"type_identifier", "qualified_identifier"},
			ImportModuleKinds: []string{"system_lib_string", "string_content"},
		},
	}
})

// CppLanguage returns the LanguageConfig for C++ source files (.cpp, .cc, .cxx, .hpp).
func CppLanguage() chunker.LanguageConfig {
	return cppLanguage()
}
