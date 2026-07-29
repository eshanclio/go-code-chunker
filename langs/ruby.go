package langs

import (
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"

	"github.com/ieshan/go-code-chunker/chunker"
)

var rubyLanguage = sync.OnceValue(func() chunker.LanguageConfig {
	return chunker.LanguageConfig{
		Name:       "ruby",
		Extensions: []string{".rb"},
		Grammar:    sitter.NewLanguage(ruby.Language()),
		// method and singleton_method appear in both TopLevel and Nested
		// intentionally: they represent both module-level methods (top-level
		// boundary) and class methods (nested construct). The chunker resolves the
		// distinction via AST parent-chain walk at atom collection time.
		NodeKinds: chunker.NodeKindSet{
			TopLevel:  []string{"class", "module", "method", "singleton_method"},
			Nested:    []string{"method", "singleton_method"},
			NameField: "name",
		},
		Edges: chunker.EdgeRules{
			Rules: []chunker.EdgeRule{
				// Ruby's call covers `helper(1)`, `obj.other`, and `require 'x'`;
				// CallNameEdges separates the structural cases out.
				{Kind: "call", Edge: chunker.EdgeCall, TargetField: "method", QualifierField: "receiver"},
				{Kind: "superclass", Edge: chunker.EdgeInherit, Descend: true},
				{Kind: "constant", Edge: chunker.EdgeReference},
			},
			NameKinds:         []string{"constant", "scope_resolution"},
			ImportModuleKinds: []string{"string_content"},
			CallNameEdges: map[string]chunker.EdgeKind{
				"require":          chunker.EdgeImport,
				"require_relative": chunker.EdgeImport,
				"load":             chunker.EdgeImport,
				"include":          chunker.EdgeInherit,
				"extend":           chunker.EdgeInherit,
				"prepend":          chunker.EdgeInherit,
			},
		},
	}
})

// RubyLanguage returns the LanguageConfig for Ruby source files (.rb).
func RubyLanguage() chunker.LanguageConfig {
	return rubyLanguage()
}
