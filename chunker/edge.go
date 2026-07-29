package chunker

// EdgeKind classifies a graph edge extracted from source.
type EdgeKind string

const (
	// EdgeCall is a function, method, or command invocation.
	EdgeCall EdgeKind = "call"
	// EdgeImport is an import, include, or require of another module.
	EdgeImport EdgeKind = "import"
	// EdgeInherit is a supertype relationship: extends, implements, embeds,
	// subclasses, or includes a mixin.
	EdgeInherit EdgeKind = "inherit"
	// EdgeReference is a type reference or instantiation, e.g. a type
	// annotation, a struct field type, or `new Thing()`.
	EdgeReference EdgeKind = "reference"
)

// Edge is one relationship extracted from a source file, pointing from the
// enclosing definition to a named target.
//
// The target is deliberately left unresolved: [Target] holds the identifier as
// written and [TargetQualifier] the receiver or package part when present
// (e.g. "fmt" for `fmt.Println`). Mapping a target to a definition requires
// whole-project knowledge, so resolution is the caller's responsibility.
//
// For [EdgeImport] edges the module path is in [Target] and [TargetQualifier]
// holds the local alias when the import declares one.
type Edge struct {
	Kind            EdgeKind // relationship classification
	FilePath        string   // absolute path to the source file
	Language        string   // language name as reported by LanguageConfig.Name
	Source          string   // name of the enclosing definition; empty at file level (e.g. imports)
	SourceParent    string   // containing symbol of the enclosing definition (e.g. class name for a method); empty for top-level
	SourceLine      int      // 1-based start line of the enclosing definition; 0 when there is none
	Target          string   // referenced identifier, or module path for EdgeImport
	TargetQualifier string   // receiver/package part when qualified, or local alias for EdgeImport; empty otherwise
	Line            int      // 1-based line of the reference itself
}

// Analysis is the combined result of chunking and edge extraction for one file.
type Analysis struct {
	Chunks []Chunk
	Edges  []Edge // nil when the language declares no EdgeRules
}
