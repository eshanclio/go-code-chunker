package chunker

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// edgeSets holds pre-computed O(1) lookup maps for a language's edge rules.
type edgeSets struct {
	rules         map[string][]EdgeRule // node kind -> rules firing on it
	nameKinds     map[string]struct{}
	moduleKinds   map[string]struct{}
	aliasKinds    map[string]struct{}
	callNameEdges map[string]EdgeKind
	qualified     map[string]QualifiedFields
	enabled       bool
}

// buildEdgeSets pre-computes lookup maps from a language's declarative EdgeRules.
func buildEdgeSets(r EdgeRules) edgeSets {
	es := edgeSets{enabled: r.enabled()}
	if !es.enabled {
		return es
	}
	es.rules = make(map[string][]EdgeRule, len(r.Rules))
	for _, rule := range r.Rules {
		es.rules[rule.Kind] = append(es.rules[rule.Kind], rule)
	}
	es.nameKinds = toSet(r.NameKinds)
	es.moduleKinds = toSet(r.ImportModuleKinds)
	es.aliasKinds = toSet(r.ImportAliasKinds)
	es.callNameEdges = r.CallNameEdges
	es.qualified = r.Qualified
	return es
}

func toSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	s := make(map[string]struct{}, len(items))
	for _, i := range items {
		s[i] = struct{}{}
	}
	return s
}

// collectEdges walks the AST emitting one Edge per matching rule. Traversal
// continues through matched nodes so that nested calls are not lost.
func (c *Chunker) collectEdges(node *sitter.Node, src []byte, langCfg *LanguageConfig, filePath string, edges *[]Edge) {
	if node == nil {
		return
	}
	es := c.edgeMap[langCfg.Name]
	if !es.enabled {
		return
	}

	for _, rule := range es.rules[node.Kind()] {
		c.applyEdgeRule(node, src, langCfg, filePath, rule, es, edges)
	}

	for i := range node.NamedChildCount() {
		c.collectEdges(node.NamedChild(i), src, langCfg, filePath, edges)
	}
}

// applyEdgeRule emits the edges produced by a single rule match.
func (c *Chunker) applyEdgeRule(node *sitter.Node, src []byte, langCfg *LanguageConfig, filePath string, rule EdgeRule, es edgeSets, edges *[]Edge) {
	if rule.Edge == EdgeImport {
		if e, ok := c.buildImportEdge(node, src, langCfg, filePath, es); ok {
			*edges = append(*edges, e)
		}
		return
	}

	if rule.Descend {
		// Descend from the named field when the rule scopes one (e.g. Python's
		// class_definition/superclasses, whose argument_list kind is otherwise
		// indistinguishable from a call's arguments), else from the node itself.
		root := node
		if rule.TargetField != "" {
			if root = node.ChildByFieldName(rule.TargetField); root == nil {
				return
			}
		}
		var names []*sitter.Node
		collectNamedKinds(root, es.nameKinds, &names)
		for _, n := range names {
			c.appendEdge(node, n, src, langCfg, filePath, rule.Edge, "", n.Utf8Text(src), es, edges)
		}
		return
	}

	target := targetNode(node, rule.TargetField)
	if target == nil {
		return
	}

	qualifier, name := c.decomposeTarget(target, src, es)
	if rule.QualifierField != "" {
		if q := node.ChildByFieldName(rule.QualifierField); q != nil {
			qualifier = strings.TrimSpace(q.Utf8Text(src))
		}
	}
	if name == "" {
		return
	}

	// Languages that spell structural relationships as ordinary calls
	// (Ruby's require and include, Bash's source) reclassify by callee name.
	if rule.Edge == EdgeCall && qualifier == "" {
		if override, ok := es.callNameEdges[name]; ok {
			c.applyCallNameOverride(node, target, src, langCfg, filePath, override, es, edges)
			return
		}
	}

	c.appendEdge(node, target, src, langCfg, filePath, rule.Edge, qualifier, name, es, edges)
}

// applyCallNameOverride emits the edge that a reclassified call produces:
// an import edge from the call's module argument, or one edge per named
// argument for any other kind (e.g. Ruby's `include SomeModule`).
func (c *Chunker) applyCallNameOverride(node, callee *sitter.Node, src []byte, langCfg *LanguageConfig, filePath string, kind EdgeKind, es edgeSets, edges *[]Edge) {
	if kind == EdgeImport {
		// Exclude the callee itself: in `source ./lib.sh` the callee and the
		// module are both bare words, and the callee comes first.
		if e, ok := c.buildImportEdgeExcluding(node, src, langCfg, filePath, es, callee); ok {
			*edges = append(*edges, e)
		}
		return
	}
	args := node.ChildByFieldName("arguments")
	if args == nil {
		return
	}
	var names []*sitter.Node
	collectNamedKinds(args, es.nameKinds, &names)
	for _, n := range names {
		c.appendEdge(node, n, src, langCfg, filePath, kind, "", n.Utf8Text(src), es, edges)
	}
}

// buildImportEdge resolves an import node to its module path and local alias.
func (c *Chunker) buildImportEdge(node *sitter.Node, src []byte, langCfg *LanguageConfig, filePath string, es edgeSets) (Edge, bool) {
	return c.buildImportEdgeExcluding(node, src, langCfg, filePath, es, nil)
}

// buildImportEdgeExcluding is buildImportEdge with a subtree skipped during the
// module search, used when the module and the callee share a node kind.
func (c *Chunker) buildImportEdgeExcluding(node *sitter.Node, src []byte, langCfg *LanguageConfig, filePath string, es edgeSets, exclude *sitter.Node) (Edge, bool) {
	moduleNode := firstDescendantOfKinds(node, es.moduleKinds, exclude)
	if moduleNode == nil {
		return Edge{}, false
	}
	module := cleanModulePath(moduleNode.Utf8Text(src))
	if module == "" {
		return Edge{}, false
	}

	// The alias sits beside the module path, which for aliased forms means a
	// wrapper node (Go's import_spec, Python's aliased_import) rather than the
	// import statement itself.
	alias := ""
	if es.aliasKinds != nil {
		if parent := moduleNode.Parent(); parent != nil {
			for i := range parent.NamedChildCount() {
				child := parent.NamedChild(i)
				if _, ok := es.aliasKinds[child.Kind()]; ok && child.Id() != moduleNode.Id() {
					alias = strings.TrimSpace(child.Utf8Text(src))
					break
				}
			}
		}
	}

	srcName, srcParent, srcLine := c.enclosingDefinition(node, src, langCfg)
	return Edge{
		Kind:            EdgeImport,
		FilePath:        filePath,
		Language:        langCfg.Name,
		Source:          srcName,
		SourceParent:    srcParent,
		SourceLine:      srcLine,
		Target:          module,
		TargetQualifier: alias,
		Line:            int(node.StartPosition().Row) + 1,
	}, true
}

// appendEdge attributes an edge to its enclosing definition and records it,
// skipping self-references which carry no information.
func (c *Chunker) appendEdge(owner, target *sitter.Node, src []byte, langCfg *LanguageConfig, filePath string, kind EdgeKind, qualifier, name string, es edgeSets, edges *[]Edge) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	srcName, srcParent, srcLine := c.enclosingDefinition(owner, src, langCfg)
	// A definition referring to itself (e.g. a Go type_spec's own
	// type_identifier) is noise, not a relationship.
	if kind == EdgeReference && qualifier == "" && name == srcName {
		return
	}
	*edges = append(*edges, Edge{
		Kind:            kind,
		FilePath:        filePath,
		Language:        langCfg.Name,
		Source:          srcName,
		SourceParent:    srcParent,
		SourceLine:      srcLine,
		Target:          name,
		TargetQualifier: qualifier,
		Line:            int(target.StartPosition().Row) + 1,
	})
}

// decomposeTarget resolves a target expression to (qualifier, name), unwrapping
// qualified-name nodes such as Go's selector_expression. Nodes of any other
// kind contribute their own text as the name.
func (c *Chunker) decomposeTarget(node *sitter.Node, src []byte, es edgeSets) (qualifier, name string) {
	if fields, ok := es.qualified[node.Kind()]; ok {
		nameNode := node.ChildByFieldName(fields.Name)
		qualNode := node.ChildByFieldName(fields.Qualifier)
		if nameNode != nil {
			name = strings.TrimSpace(nameNode.Utf8Text(src))
		}
		if qualNode != nil {
			qualifier = strings.TrimSpace(qualNode.Utf8Text(src))
		}
		return qualifier, name
	}
	return "", strings.TrimSpace(node.Utf8Text(src))
}

// enclosingDefinition walks up from node to the innermost definition that
// contains it, returning that definition's name, its own parent symbol, and its
// start line. All three are zero-valued for file-level nodes such as imports.
//
// The walk includes node itself, because rules may be anchored on a definition
// node (Python's class_definition/superclasses) rather than inside its body.
func (c *Chunker) enclosingDefinition(node *sitter.Node, src []byte, langCfg *LanguageConfig) (name, parent string, line int) {
	sets := c.kindMap[langCfg.Name]
	for p := node; p != nil; p = p.Parent() {
		kind := p.Kind()
		_, inTopLevel := sets.topLevel[kind]
		_, inNested := sets.nested[kind]
		if !inTopLevel && !inNested {
			continue
		}
		if langCfg.NodeKinds.NameField == "" {
			continue
		}
		nameNode := p.ChildByFieldName(langCfg.NodeKinds.NameField)
		if nameNode == nil {
			// Wrapper kinds without their own name field (e.g. Go's
			// type_declaration) delegate to the inner spec node; keep walking.
			continue
		}
		defName := strings.TrimSpace(nameNode.Utf8Text(src))
		if defName == "" {
			continue
		}
		defParent := ""
		if inNested {
			defParent = c.extractParentName(p, src, langCfg)
		}
		return defName, defParent, int(p.StartPosition().Row) + 1
	}
	return "", "", 0
}

// targetNode returns the child named by field, or the node itself when field is
// empty (the shape used by leaf rules such as type_identifier).
func targetNode(node *sitter.Node, field string) *sitter.Node {
	if field != "" {
		return node.ChildByFieldName(field)
	}
	return node
}

// collectNamedKinds gathers descendants whose kind is in kinds, without
// recursing into a match.
func collectNamedKinds(node *sitter.Node, kinds map[string]struct{}, out *[]*sitter.Node) {
	for i := range node.NamedChildCount() {
		child := node.NamedChild(i)
		if _, ok := kinds[child.Kind()]; ok {
			*out = append(*out, child)
			continue
		}
		collectNamedKinds(child, kinds, out)
	}
}

// firstDescendantOfKinds returns the first descendant (depth-first, including
// node itself) whose kind is in kinds, skipping the exclude subtree, or nil.
func firstDescendantOfKinds(node *sitter.Node, kinds map[string]struct{}, exclude *sitter.Node) *sitter.Node {
	if kinds == nil || node == nil {
		return nil
	}
	if exclude != nil && node.Id() == exclude.Id() {
		return nil
	}
	if _, ok := kinds[node.Kind()]; ok {
		return node
	}
	for i := range node.NamedChildCount() {
		if found := firstDescendantOfKinds(node.NamedChild(i), kinds, exclude); found != nil {
			return found
		}
	}
	return nil
}

// edgeKey identifies an edge for duplicate detection.
type edgeKey struct {
	kind      EdgeKind
	source    string
	target    string
	qualifier string
	line      int
}

// dedupeEdges drops exact duplicates and demotes redundant reference edges.
//
// A supertype name occupies a type position, so grammars with distinct type
// nodes (TypeScript's `implements Iface`, C++'s base_class_clause) yield both an
// inherit and a reference edge for it. The inherit edge is strictly more
// informative, so the reference edge is dropped.
func dedupeEdges(edges []Edge) []Edge {
	if len(edges) < 2 {
		return edges
	}
	seen := make(map[edgeKey]struct{}, len(edges))
	inherited := make(map[[2]string]struct{})
	for _, e := range edges {
		if e.Kind == EdgeInherit {
			inherited[[2]string{e.Source, e.Target}] = struct{}{}
		}
	}

	out := edges[:0]
	for _, e := range edges {
		if e.Kind == EdgeReference {
			if _, ok := inherited[[2]string{e.Source, e.Target}]; ok {
				continue
			}
		}
		k := edgeKey{e.Kind, e.Source, e.Target, e.TargetQualifier, e.Line}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, e)
	}
	return out
}

// cleanModulePath strips the quoting that grammars keep around module paths:
// "fmt" -> fmt, <stdio.h> -> stdio.h, 'json' -> json.
func cleanModulePath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	return strings.TrimSpace(s)
}
