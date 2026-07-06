# go-code-chunker

[![Go Reference](https://pkg.go.dev/badge/github.com/ieshan/go-code-chunker.svg)](https://pkg.go.dev/github.com/ieshan/go-code-chunker)

Tree-sitter AST-based code chunker for Go. Splits source files into semantically coherent pieces (functions, classes, declarations) suitable for embedding and semantic search.

## Packages

- **chunker/** — The cAST chunking algorithm. Takes a tree-sitter AST and produces semantically coherent chunks with metadata (language, node kind, symbol name, line range). No CGo, no language knowledge.
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

ch := chunker.NewChunker(langs.AllLanguages())
chunks, err := ch.ChunkFile("main.go", sourceBytes)
```

Each `chunker.Chunk` contains the source text, byte range, line range, language name, AST node kind, and symbol name.

## License

MPL-2.0. See [LICENSE](LICENSE).
