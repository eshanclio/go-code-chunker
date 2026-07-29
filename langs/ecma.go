package langs

import "github.com/ieshan/go-code-chunker/chunker"

// ecmaEdgeRules returns the edge rules shared by the ECMAScript-family
// grammars. JavaScript, TypeScript, and TSX describe calls, imports, and class
// heritage identically; only TypeScript and TSX add type syntax, which typed
// selects.
func ecmaEdgeRules(typed bool) chunker.EdgeRules {
	rules := []chunker.EdgeRule{
		{Kind: "call_expression", Edge: chunker.EdgeCall, TargetField: "function"},
		{Kind: "new_expression", Edge: chunker.EdgeReference, TargetField: "constructor"},
		{Kind: "import_statement", Edge: chunker.EdgeImport},
		// extends and implements both live under class_heritage.
		{Kind: "class_heritage", Edge: chunker.EdgeInherit, Descend: true},
	}
	nameKinds := []string{"identifier"}

	if typed {
		rules = append(rules,
			chunker.EdgeRule{Kind: "extends_type_clause", Edge: chunker.EdgeInherit, Descend: true},
			chunker.EdgeRule{Kind: "type_identifier", Edge: chunker.EdgeReference},
		)
		nameKinds = append(nameKinds, "type_identifier")
	}

	return chunker.EdgeRules{
		Rules: rules,
		Qualified: map[string]chunker.QualifiedFields{
			"member_expression": {Qualifier: "object", Name: "property"},
		},
		NameKinds: nameKinds,
		// string_fragment is the unquoted body of an import's module string;
		// it also carries the argument of a require() call.
		ImportModuleKinds: []string{"string_fragment"},
		CallNameEdges:     map[string]chunker.EdgeKind{"require": chunker.EdgeImport},
	}
}
