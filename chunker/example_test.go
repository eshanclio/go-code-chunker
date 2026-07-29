package chunker_test

import (
	"fmt"

	"github.com/ieshan/go-code-chunker/chunker"
	"github.com/ieshan/go-code-chunker/langs"
)

func ExampleNewChunker() {
	c, err := chunker.NewChunker(langs.AllLanguages(), chunker.DefaultConfig())
	if err != nil {
		panic(err)
	}
	_ = c
	fmt.Println("chunker ready")
	// Output: chunker ready
}

func ExampleChunker_ChunkFile() {
	c, err := chunker.NewChunker(
		[]chunker.LanguageConfig{langs.GoLanguage()},
		chunker.DefaultConfig(),
	)
	if err != nil {
		panic(err)
	}

	src := `package main

func Hello() string {
	return "hello"
}
`
	chunks, err := c.ChunkFile("main.go", []byte(src))
	if err != nil {
		panic(err)
	}
	for _, ch := range chunks {
		fmt.Printf("kind=%s name=%s\n", ch.NodeKind, ch.Name)
	}
	// Output: kind=function_declaration name=Hello
}

func ExampleChunker_Analyze() {
	c, err := chunker.NewChunker(
		[]chunker.LanguageConfig{langs.GoLanguage()},
		chunker.DefaultConfig(),
	)
	if err != nil {
		panic(err)
	}

	src := `package main

import "fmt"

func Greet() {
	fmt.Println(name())
}

func name() string { return "world" }
`
	res, err := c.Analyze("main.go", []byte(src))
	if err != nil {
		panic(err)
	}
	for _, e := range res.Edges {
		if e.Kind == chunker.EdgeReference {
			continue // omit type references to keep the output short
		}
		target := e.Target
		if e.TargetQualifier != "" {
			target = e.TargetQualifier + "." + e.Target
		}
		fmt.Printf("%s: %q -> %s\n", e.Kind, e.Source, target)
	}
	// Output:
	// import: "" -> fmt
	// call: "Greet" -> fmt.Println
	// call: "Greet" -> name
}
