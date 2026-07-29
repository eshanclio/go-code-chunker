package chunker

// QualifiedFields decomposes a qualified-name node into its two halves, e.g.
// Go's selector_expression ("fmt.Println" -> operand "fmt", field "Println")
// or Python's attribute ("os.path.join" -> object "os.path", attribute "join").
type QualifiedFields struct {
	Qualifier string // field name holding the receiver/package part
	Name      string // field name holding the selected identifier
}

// EdgeRule maps one AST node kind to the edge it produces.
//
// The node named by TargetField is resolved to a target name: qualified-name
// nodes are decomposed via [EdgeRules.Qualified], and any other node
// contributes its own source text. When Descend is set the rule instead emits
// one edge per matching descendant, which is how supertype lists are handled.
type EdgeRule struct {
	Kind           string   // AST node kind that triggers this rule, e.g. "call_expression"
	Edge           EdgeKind // edge kind to emit
	TargetField    string   // field holding the target expression; empty means the node's first named child
	QualifierField string   // field holding the receiver, for grammars that qualify on the node itself (e.g. Ruby's call)
	Descend        bool     // emit one edge per descendant whose kind is in EdgeRules.NameKinds
}

// EdgeRules declares how to extract graph edges for one language. The zero
// value disables edge extraction, which is the correct behaviour for data and
// markup languages such as JSON, YAML, and CSS.
type EdgeRules struct {
	// Rules lists the node kinds that produce edges.
	Rules []EdgeRule

	// Qualified maps a qualified-name node kind to the fields that split it
	// into qualifier and name.
	Qualified map[string]QualifiedFields

	// NameKinds lists node kinds that count as a name when a rule descends,
	// e.g. "identifier", "type_identifier", "constant".
	NameKinds []string

	// ImportModuleKinds lists node kinds that carry an import's module text.
	// The first matching descendant of an import node wins. Surrounding
	// quotes and angle brackets are stripped from the result.
	ImportModuleKinds []string

	// ImportAliasKinds lists node kinds that carry an import's local alias,
	// searched among the import node's direct named children.
	ImportAliasKinds []string

	// CallNameEdges reclassifies calls to specific well-known names, for
	// languages that spell structural relationships as ordinary function calls:
	// Ruby's require and include, Bash's source, JavaScript's require.
	//
	// A call whose unqualified target matches a key emits the mapped edge kind
	// instead of [EdgeCall]. [EdgeImport] takes its target from the first
	// ImportModuleKinds descendant; any other kind takes one edge per NameKinds
	// descendant.
	CallNameEdges map[string]EdgeKind
}

// enabled reports whether these rules can produce any edge.
func (r EdgeRules) enabled() bool { return len(r.Rules) > 0 }
