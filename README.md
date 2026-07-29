# go-code-chunker

[![Go Reference](https://pkg.go.dev/badge/github.com/ieshan/go-code-chunker.svg)](https://pkg.go.dev/github.com/ieshan/go-code-chunker)

Tree-sitter AST-based code chunker for Go. Splits source files into semantically coherent pieces (functions, classes, declarations) suitable for embedding and semantic search, and optionally extracts the graph edges (calls, imports, supertypes, type references) between them.

## Packages

- **chunker/** — The cAST chunking algorithm and the graph edge extractor. Takes a tree-sitter AST and produces semantically coherent chunks with metadata (language, node kind, symbol name, line range). No CGo, no language knowledge.
- **langs/** — Per-language tree-sitter configurations for 15 languages: Go, Python, JavaScript, TypeScript, TSX, Ruby, C, C++, Bash, HTML, CSS, Markdown, JSON, YAML, Vue. This is the only package with CGo (tree-sitter grammars include C sources).

## Requirements

- Go 1.26+
- A C compiler (CGo is required for tree-sitter grammars). Ensure `cc` is on `PATH`, or set `CC`.

## Usage

```go
import (
    "github.com/ieshan/go-code-chunker/chunker"
    "github.com/ieshan/go-code-chunker/langs"
)

ch, err := chunker.NewChunker(langs.AllLanguages(), chunker.DefaultConfig())
chunks, err := ch.ChunkFile("main.go", sourceBytes)
```

Each `chunker.Chunk` contains the source text, byte range, line range, language name, AST node kind, and symbol name.

## Graph edges

`Analyze` returns chunks and the relationships between symbols from the same parse, so building a code graph costs one extra AST walk rather than a second parse:

```go
ch, err := chunker.NewChunker(langs.AllLanguages(), chunker.DefaultConfig())
res, err := ch.Analyze("main.go", sourceBytes)

for _, e := range res.Edges {
    fmt.Printf("%s: %s -> %s\n", e.Kind, e.Source, e.Target)
}
// call: Area -> helper
// call: Area -> Println   (TargetQualifier "fmt")
// import: -> strings      (TargetQualifier "str", the local alias)
```

Four edge kinds are extracted:

| Kind | Meaning |
|---|---|
| `call` | function, method, or command invocation |
| `import` | import, include, or require of another module |
| `inherit` | extends, implements, subclasses, or includes a mixin |
| `reference` | type reference or instantiation (`new Thing()`, a field's type) |

Each edge names the enclosing definition it came from (`Source`, plus `SourceParent` for methods) and the identifier it points at (`Target`, plus `TargetQualifier` for `pkg.Foo` or `obj.method`).

**Targets are deliberately unresolved.** Mapping `helper` to a particular definition needs whole-project knowledge, so resolution belongs to the caller, which can weigh imports, same-file preference, and its own symbol table.

`ChunkFile` is unchanged and skips edge extraction entirely.

### Declaring edge rules

Edge extraction is driven by the declarative `EdgeRules` on each `LanguageConfig`, in the same spirit as `NodeKinds`. Adding or correcting a language means editing node kinds and field names, not writing a traversal:

```go
Edges: chunker.EdgeRules{
    Rules: []chunker.EdgeRule{
        {Kind: "call_expression", Edge: chunker.EdgeCall, TargetField: "function"},
        {Kind: "import_spec", Edge: chunker.EdgeImport},
    },
    Qualified: map[string]chunker.QualifiedFields{
        "selector_expression": {Qualifier: "operand", Name: "field"},
    },
    ImportModuleKinds: []string{"interpreted_string_literal"},
}
```

A zero `EdgeRules` disables extraction, which is what the data and markup languages (JSON, YAML, CSS, Markdown, HTML) declare. `CallNameEdges` handles languages that spell structural relationships as ordinary calls — Ruby's `require` and `include`, Bash's `source`, JavaScript's `require`.

### Coverage notes

- **Go** has no inheritance; struct and interface embedding surfaces as a `reference` edge.
- **C** declares no `NameField`, so its edges carry an empty `Source` (the same limitation that leaves `Chunk.Name` empty for C). Attribute them by line instead.
- **C++** member functions nest their name inside a declarator chain, so edges inside a method are attributed to the enclosing class rather than the method.
- **Vue** collects edges from the injected `<script>` grammar as well as the outer template.
- `reference` edges include primitive type names (Go's `float64`, for instance). Filter by target if your graph does not want them.

## License

MPL-2.0. See [LICENSE](LICENSE).
