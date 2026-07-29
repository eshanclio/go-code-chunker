package langs

import (
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	bash "github.com/tree-sitter/tree-sitter-bash/bindings/go"

	"github.com/ieshan/go-code-chunker/chunker"
)

var bashLanguage = sync.OnceValue(func() chunker.LanguageConfig {
	return chunker.LanguageConfig{
		Name:       "bash",
		Extensions: []string{".sh", ".bash"},
		Grammar:    sitter.NewLanguage(bash.Language()),
		NodeKinds: chunker.NodeKindSet{
			TopLevel:  []string{"function_definition"},
			Nested:    []string{},
			NameField: "name",
		},
		Edges: chunker.EdgeRules{
			Rules: []chunker.EdgeRule{
				// Every command is a potential call to a function defined
				// elsewhere in the script or sourced from another file.
				{Kind: "command", Edge: chunker.EdgeCall, TargetField: "name"},
			},
			ImportModuleKinds: []string{"word"},
			CallNameEdges: map[string]chunker.EdgeKind{
				"source": chunker.EdgeImport,
				".":      chunker.EdgeImport,
			},
		},
	}
})

// BashLanguage returns the LanguageConfig for Bash scripts (.sh, .bash).
func BashLanguage() chunker.LanguageConfig {
	return bashLanguage()
}
