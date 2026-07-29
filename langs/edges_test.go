package langs_test

import (
	"testing"

	"github.com/ieshan/go-code-chunker/chunker"
	"github.com/ieshan/go-code-chunker/langs"
)

// wantEdge is one expected edge. Qualifier is compared only when non-empty,
// and Source is compared only when the test sets it.
type wantEdge struct {
	kind      chunker.EdgeKind
	source    string
	qualifier string
	target    string
}

func analyzeSrc(t *testing.T, lang chunker.LanguageConfig, path, src string) []chunker.Edge {
	t.Helper()
	c, err := chunker.NewChunker([]chunker.LanguageConfig{lang}, chunker.DefaultConfig())
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}
	res, err := c.Analyze(path, []byte(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return res.Edges
}

func assertEdges(t *testing.T, edges []chunker.Edge, want []wantEdge) {
	t.Helper()
	for _, w := range want {
		found := false
		for _, e := range edges {
			if e.Kind != w.kind || e.Target != w.target {
				continue
			}
			if w.source != "" && e.Source != w.source {
				continue
			}
			if w.qualifier != "" && e.TargetQualifier != w.qualifier {
				continue
			}
			found = true
			break
		}
		if !found {
			t.Errorf("missing edge %s %s -> %s%s\ngot: %s",
				w.kind, w.source, qualPrefix(w.qualifier), w.target, formatEdges(edges))
		}
	}
}

func qualPrefix(q string) string {
	if q == "" {
		return ""
	}
	return q + "."
}

func formatEdges(edges []chunker.Edge) string {
	s := ""
	for _, e := range edges {
		s += "\n  " + string(e.Kind) + " " + e.Source + " -> " + qualPrefix(e.TargetQualifier) + e.Target
	}
	if s == "" {
		return " (none)"
	}
	return s
}

func TestGoEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.GoLanguage(), "a.go", `package main

import (
	"fmt"
	str "strings"
)

type Circle struct {
	Shape
	r float64
}

func (c Circle) Area() float64 {
	v := helper(c.r)
	fmt.Println(v)
	str.ToUpper("a")
	return v
}

func helper(f float64) float64 { return f * 2 }
`)

	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "fmt"},
		{chunker.EdgeImport, "", "str", "strings"},
		{chunker.EdgeCall, "Area", "", "helper"},
		{chunker.EdgeCall, "Area", "fmt", "Println"},
		{chunker.EdgeCall, "Area", "str", "ToUpper"},
		// Go has no inheritance; struct embedding surfaces as a reference.
		{chunker.EdgeReference, "Circle", "", "Shape"},
	})
}

func TestPythonEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.PythonLanguage(), "a.py", `import os
import numpy as np
from typing import List
from .local import thing

class Base:
    pass

class Child(Base, Mixin):
    def method(self):
        helper(1)
        os.path.join("a")

def helper(x):
    return x
`)

	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "os"},
		{chunker.EdgeImport, "", "np", "numpy"},
		{chunker.EdgeImport, "", "", "typing"},
		{chunker.EdgeImport, "", "", ".local"},
		{chunker.EdgeInherit, "Child", "", "Base"},
		{chunker.EdgeInherit, "Child", "", "Mixin"},
		{chunker.EdgeCall, "method", "", "helper"},
		{chunker.EdgeCall, "method", "os.path", "join"},
	})
}

// A call's argument_list shares its node kind with a class's superclass list,
// so call arguments must never be mistaken for supertypes.
func TestPythonEdges_CallArgsAreNotInheritance(t *testing.T) {
	edges := analyzeSrc(t, langs.PythonLanguage(), "a.py", `def run():
    helper(Thing, Other)
`)

	for _, e := range edges {
		if e.Kind == chunker.EdgeInherit {
			t.Errorf("call arguments produced an inherit edge: %+v", e)
		}
	}
}

func TestJavaScriptEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.JavaScriptLanguage(), "a.js", `import { a } from "./mod";
const x = require("pkg");

class Base {}
class Child extends Base {
  m() { helper(1); obj.method(); }
}
function helper(x) { return x; }
`)

	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "./mod"},
		// require() is a call in the grammar but an import in meaning.
		{chunker.EdgeImport, "", "", "pkg"},
		{chunker.EdgeInherit, "Child", "", "Base"},
		{chunker.EdgeCall, "m", "", "helper"},
		{chunker.EdgeCall, "m", "obj", "method"},
	})
}

func TestTypeScriptEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.TypeScriptLanguage(), "a.ts", `import { a } from "./mod";

interface Iface { m(): void }
interface Sub extends Iface {}

class Base {}

class Child extends Base implements Iface {
  m(): void {
    helper(1);
    new Thing();
  }
}

function helper(x: number): Circle { return x; }
`)

	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "./mod"},
		{chunker.EdgeInherit, "Sub", "", "Iface"},
		{chunker.EdgeInherit, "Child", "", "Base"},
		{chunker.EdgeInherit, "Child", "", "Iface"},
		{chunker.EdgeCall, "m", "", "helper"},
		{chunker.EdgeReference, "m", "", "Thing"},
		{chunker.EdgeReference, "helper", "", "Circle"},
	})

	// A supertype also occupies a type position; the inherit edge wins.
	for _, e := range edges {
		if e.Kind == chunker.EdgeReference && e.Source == "Child" && e.Target == "Iface" {
			t.Errorf("supertype should not also yield a reference edge: %+v", e)
		}
	}
}

func TestTSXEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.TSXLanguage(), "a.tsx", `import { useState } from "react";

function App() {
  const [v] = useState(0);
  return <div>{v}</div>;
}
`)

	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "react"},
		{chunker.EdgeCall, "App", "", "useState"},
	})
}

func TestRubyEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.RubyLanguage(), "a.rb", `require 'json'
require_relative 'local'

module M
end

class Base
end

class Child < Base
  include M
  def method
    helper(1)
    obj.other
  end
end

def helper(x)
  x
end
`)

	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "json"},
		{chunker.EdgeImport, "", "", "local"},
		{chunker.EdgeInherit, "Child", "", "Base"},
		// Mixins are spelled as a call but mean inheritance.
		{chunker.EdgeInherit, "Child", "", "M"},
		{chunker.EdgeCall, "method", "", "helper"},
		{chunker.EdgeCall, "method", "obj", "other"},
	})
}

func TestCEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.CLanguage(), "a.c", `#include <stdio.h>
#include "local.h"

int helper(int x) { return x; }

int main(void) {
	int v = helper(2);
	printf("%d", v);
	return 0;
}
`)

	// C declares no NameField, so Source is empty; see CLanguage's doc comment.
	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "stdio.h"},
		{chunker.EdgeImport, "", "", "local.h"},
		{chunker.EdgeCall, "", "", "helper"},
		{chunker.EdgeCall, "", "", "printf"},
	})
}

func TestCppEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.CppLanguage(), "a.cpp", `#include <vector>

class Base {};

class Child : public Base {
public:
	void m() {
		helper(1);
		obj.method();
		ns::func();
	}
};

int helper(int x) { return x; }
`)

	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "vector"},
		{chunker.EdgeInherit, "Child", "", "Base"},
		{chunker.EdgeCall, "", "", "helper"},
		{chunker.EdgeCall, "", "obj", "method"},
		{chunker.EdgeCall, "", "ns", "func"},
	})
}

func TestBashEdges(t *testing.T) {
	edges := analyzeSrc(t, langs.BashLanguage(), "a.sh", `source ./lib.sh
. ./other.sh

helper() {
	echo "hi"
}

helper
main_thing arg
`)

	assertEdges(t, edges, []wantEdge{
		// The sourced path, not the "source" command itself.
		{chunker.EdgeImport, "", "", "./lib.sh"},
		{chunker.EdgeImport, "", "", "./other.sh"},
		{chunker.EdgeCall, "helper", "", "echo"},
		{chunker.EdgeCall, "", "", "main_thing"},
	})

	for _, e := range edges {
		if e.Kind == chunker.EdgeImport && (e.Target == "source" || e.Target == ".") {
			t.Errorf("import target should be the path, not the command: %+v", e)
		}
	}
}

// Vue delegates its <script> block to an injected grammar; edges must be
// collected from the injected parse too.
func TestVueEdges_FromInjectedScript(t *testing.T) {
	edges := analyzeSrc(t, langs.VueLanguage(), "a.vue", `<template>
  <div>{{ msg }}</div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
export default defineComponent({
  methods: {
    go() { helper(1); }
  }
});
</script>
`)

	assertEdges(t, edges, []wantEdge{
		{chunker.EdgeImport, "", "", "vue"},
		{chunker.EdgeCall, "", "", "defineComponent"},
		{chunker.EdgeCall, "go", "", "helper"},
	})
}

// Data and markup languages have no meaningful relationships to extract.
func TestDataLanguages_ProduceNoEdges(t *testing.T) {
	cases := []struct {
		lang chunker.LanguageConfig
		path string
		src  string
	}{
		{langs.JSONLanguage(), "a.json", `{"a": 1, "b": [2, 3]}`},
		{langs.YAMLLanguage(), "a.yaml", "a: 1\nb:\n  - 2\n"},
		{langs.CSSLanguage(), "a.css", ".a { color: red; }"},
		{langs.MarkdownLanguage(), "a.md", "# Title\n\nSome text.\n"},
		{langs.HTMLLanguage(), "a.html", "<div><p>hi</p></div>"},
	}

	for _, tc := range cases {
		t.Run(tc.lang.Name, func(t *testing.T) {
			if edges := analyzeSrc(t, tc.lang, tc.path, tc.src); len(edges) != 0 {
				t.Errorf("expected no edges, got %s", formatEdges(edges))
			}
		})
	}
}

// Every language must at least be analyzable without error.
func TestAllLanguages_AnalyzeIsSafe(t *testing.T) {
	c, err := chunker.NewChunker(langs.AllLanguages(), chunker.DefaultConfig())
	if err != nil {
		t.Fatalf("NewChunker: %v", err)
	}
	for _, lang := range langs.AllLanguages() {
		for _, ext := range lang.Extensions {
			if _, err := c.Analyze("fixture"+ext, []byte("x")); err != nil {
				t.Errorf("%s (%s): %v", lang.Name, ext, err)
			}
		}
	}
}
