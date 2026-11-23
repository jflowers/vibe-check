package ripwireadapter

import (
	"reflect"
	"testing"
)

// Relative-import resolution tests — openspec/changes/relative-import-edges
// tasks 1.2 and 2.3. They pin how a Python relative target or member base
// (leading dot, recorded verbatim by ripwire's --deps) resolves against the
// importing file's package: the anchor climb, the no-ancestor-guess rule,
// climb-above-root as unresolved, and never external.

func TestRelativeBaseDir_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceDir  string
		base       string
		wantAnchor string
		wantDir    string
		wantOK     bool
	}{
		{name: "single dot is the package", sourceDir: "pk/b", base: ".", wantAnchor: "pk/b", wantDir: "pk/b", wantOK: true},
		{name: "two dots climb one", sourceDir: "pk/b", base: "..", wantAnchor: "pk", wantDir: "pk", wantOK: true},
		{name: "three dots reach the crawl root", sourceDir: "pk/b", base: "...", wantAnchor: ".", wantDir: ".", wantOK: true},
		{name: "climbing above the crawl root", sourceDir: "pk/b", base: "....", wantOK: false},
		{name: "dot with remainder", sourceDir: "pk/b", base: ".api", wantAnchor: "pk/b", wantDir: "pk/b/api", wantOK: true},
		{name: "dots with dotted remainder", sourceDir: "pk/a", base: "..b.api", wantAnchor: "pk", wantDir: "pk/b/api", wantOK: true},
		{name: "root-level file single dot", sourceDir: ".", base: ".", wantAnchor: ".", wantDir: ".", wantOK: true},
		{name: "root-level file with remainder", sourceDir: ".", base: ".pk", wantAnchor: ".", wantDir: "pk", wantOK: true},
		{name: "root-level file climbing", sourceDir: ".", base: "..", wantOK: false},
		{name: "absolute base rejected", sourceDir: "pk", base: "pk.b", wantOK: false},
		{name: "empty component rejected", sourceDir: "pk", base: ".a..b", wantOK: false},
		{name: "trailing dot rejected", sourceDir: "pk", base: "..a.", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			anchor, dir, ok := relativeBaseDir(tt.sourceDir, tt.base)
			if ok != tt.wantOK {
				t.Fatalf("relativeBaseDir(%q, %q) ok: got %v, want %v", tt.sourceDir, tt.base, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if anchor != tt.wantAnchor || dir != tt.wantDir {
				t.Errorf("relativeBaseDir(%q, %q): got (%q, %q), want (%q, %q)",
					tt.sourceDir, tt.base, anchor, dir, tt.wantAnchor, tt.wantDir)
			}
		})
	}
}

// relativeArch is the module universe shared by the relative-resolution
// tests: a package root, two subpackages, and a nested one.
func relativeArch() archTable {
	return syntheticArch(
		archModuleRow{Path: "pk"},
		archModuleRow{Path: "pk/a"},
		archModuleRow{Path: "pk/a/deep"},
		archModuleRow{Path: "pk/b"},
	)
}

func TestResolveTarget_RelativeTable(t *testing.T) {
	t.Parallel()

	table := relativeArch()
	moduleByDir := map[string]*mappedModule{}
	for _, row := range table.Rows {
		identity, err := moduleIdentity(languagePython, row.Path)
		if err != nil {
			t.Fatalf("moduleIdentity: %v", err)
		}
		moduleByDir[row.Path] = &mappedModule{Identity: identity}
	}

	tests := []struct {
		name      string
		sourceDir string
		target    string
		want      targetResolution
	}{
		{name: "relative base into a sibling module file", sourceDir: "pk/a", target: "..b.api", want: targetResolution{identity: "pk.b"}},
		{name: "bare double dot is the parent package", sourceDir: "pk/b", target: "..", want: targetResolution{identity: "pk"}},
		{name: "single dot is the own package", sourceDir: "pk/a", target: ".", want: targetResolution{identity: "pk.a"}},
		{name: "own-package module file", sourceDir: "pk/b", target: ".api", want: targetResolution{identity: "pk.b"}},
		{name: "nested package climbs two", sourceDir: "pk/a/deep", target: "...", want: targetResolution{identity: "pk"}},
		{name: "nested package sibling file", sourceDir: "pk/a/deep", target: "..inner", want: targetResolution{identity: "pk.a"}},
		{name: "subpackage below the anchor", sourceDir: "pk", target: ".a.deep.x", want: targetResolution{identity: "pk.a.deep"}},
		{name: "climbing above the crawl root", sourceDir: "pk/a", target: "....", want: targetResolution{unresolved: true}},
		{name: "malformed relative target", sourceDir: "pk/a", target: ".x..y", want: targetResolution{unresolved: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, nameResolved, err := resolveTarget(languagePython, tt.sourceDir, tt.target, moduleByDir)
			if err != nil {
				t.Fatalf("resolveTarget: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveTarget(%q, %q): got %+v, want %+v", tt.sourceDir, tt.target, got, tt.want)
			}
			if nameResolved {
				t.Errorf("resolveTarget(%q, %q): relative resolution reported name-resolved", tt.sourceDir, tt.target)
			}
		})
	}
}

func TestResolveTarget_RelativeNoAncestorGuess(t *testing.T) {
	t.Parallel()

	// The anchor "ns" (a namespace-package directory with no --arch row)
	// and the candidate "ns/x" are not modules, but the crawl root "." is.
	// The statement names nothing at or above "." beyond the anchor, so no
	// edge to "." is guessed.
	moduleByDir := map[string]*mappedModule{".": {Identity: "."}, "ns/inner": {Identity: "ns/inner"}}
	got, _, err := resolveTarget(languagePython, "ns/inner", "..x", moduleByDir)
	if err != nil {
		t.Fatalf("resolveTarget: %v", err)
	}
	if want := (targetResolution{unresolved: true}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestMapToModules_RelativeTargets(t *testing.T) {
	t.Parallel()

	deps := depsGraph{Rows: []depsFile{
		{Path: "pk/a/__init__.py", IncTargets: []string{"..b.api", "."}},
		{Path: "pk/b/__init__.py", IncTargets: []string{"..", ".api"}},
	}}
	mapping, err := mapToModules(languagePython, relativeArch(), deps, nil)
	if err != nil {
		t.Fatalf("mapToModules: %v", err)
	}
	// "." and ".api" resolve to the importing module itself: a self-edge,
	// the same as an absolute same-package import (design Non-Goals).
	wantEdges := []moduleEdge{
		{From: "pk.a", To: "pk.a"},
		{From: "pk.a", To: "pk.b"},
		{From: "pk.b", To: "pk"},
		{From: "pk.b", To: "pk.b"},
	}
	if !reflect.DeepEqual(mapping.Edges, wantEdges) {
		t.Errorf("edges: got %v, want %v", mapping.Edges, wantEdges)
	}
	for _, identity := range []string{"pk", "pk.a", "pk.a.deep", "pk.b"} {
		if got := moduleByIdentity(t, mapping, identity).ExternalOut; got != 0 {
			t.Errorf("%s external out: got %d, want 0 (relative targets are never external)", identity, got)
		}
	}
	// Only relative targets: nothing was name-resolved, nothing dropped.
	assertHasCodes(t, mapping.Warnings,
		[]string{"directory-as-module"},
		[]string{warningNameResolvedEdges, warningAmbiguityRate})
}

func TestMapToModules_RelativeClimbAboveRoot(t *testing.T) {
	t.Parallel()

	deps := depsGraph{Rows: []depsFile{{Path: "pk/a/deep/__init__.py", IncTargets: []string{"...."}}}}
	mapping, err := mapToModules(languagePython, relativeArch(), deps, nil)
	if err != nil {
		t.Fatalf("mapToModules: %v", err)
	}
	if len(mapping.Edges) != 0 {
		t.Errorf("edges: got %v, want none", mapping.Edges)
	}
	if got := moduleByIdentity(t, mapping, "pk.a.deep").ExternalOut; got != 0 {
		t.Errorf("external out: got %d, want 0", got)
	}
	assertHasCodes(t, mapping.Warnings, []string{warningAmbiguityRate}, nil)
}

func TestMapToModules_RelativeMemberEdge(t *testing.T) {
	t.Parallel()

	// `from .. import a` in pk/b: base edge pk.b → pk, member edge
	// pk.b → pk.a. `from ... import b` in pk/a/deep: base → pk, member →
	// pk.b. `from . import missing` names no module: base self-edge only.
	deps := depsGraph{Rows: []depsFile{
		{Path: "pk/b/__init__.py", IncTargets: []string{".."}},
		{Path: "pk/a/deep/__init__.py", IncTargets: []string{"...", "."}},
	}}
	members := map[string][]memberImport{
		"pk/b/__init__.py":      {{Base: "..", Name: "a"}},
		"pk/a/deep/__init__.py": {{Base: ".", Name: "missing"}, {Base: "...", Name: "b"}},
	}
	mapping, err := mapToModules(languagePython, relativeArch(), deps, members)
	if err != nil {
		t.Fatalf("mapToModules: %v", err)
	}
	wantEdges := []moduleEdge{
		{From: "pk.a.deep", To: "pk"},
		{From: "pk.a.deep", To: "pk.a.deep"},
		{From: "pk.a.deep", To: "pk.b"},
		{From: "pk.b", To: "pk"},
		{From: "pk.b", To: "pk.a"},
	}
	if !reflect.DeepEqual(mapping.Edges, wantEdges) {
		t.Errorf("edges: got %v, want %v", mapping.Edges, wantEdges)
	}
}

func TestMapToModules_RelativeMemberUnanchored(t *testing.T) {
	t.Parallel()

	// The scan found `from .. import a`, but ripwire recorded no ".."
	// include for the file: no member edge (the anchor is verbatim, so
	// the absolute spelling "pk" does not anchor a relative member).
	deps := depsGraph{Rows: []depsFile{{Path: "pk/b/__init__.py", IncTargets: []string{"pk"}}}}
	members := map[string][]memberImport{"pk/b/__init__.py": {{Base: "..", Name: "a"}}}
	mapping, err := mapToModules(languagePython, relativeArch(), deps, members)
	if err != nil {
		t.Fatalf("mapToModules: %v", err)
	}
	wantEdges := []moduleEdge{{From: "pk.b", To: "pk"}}
	if !reflect.DeepEqual(mapping.Edges, wantEdges) {
		t.Errorf("edges: got %v, want %v", mapping.Edges, wantEdges)
	}
}

func TestMapToModules_RelativeMemberClimb(t *testing.T) {
	t.Parallel()

	// A relative member base that climbs above the crawl root adds no
	// member edge, even when anchored and even when a module named like
	// the member exists at the root.
	table := syntheticArch(archModuleRow{Path: "pk"}, archModuleRow{Path: "a"})
	deps := depsGraph{Rows: []depsFile{{Path: "pk/mod.py", IncTargets: []string{"..."}}}}
	members := map[string][]memberImport{"pk/mod.py": {{Base: "...", Name: "a"}}}
	mapping, err := mapToModules(languagePython, table, deps, members)
	if err != nil {
		t.Fatalf("mapToModules: %v", err)
	}
	if len(mapping.Edges) != 0 {
		t.Errorf("edges: got %v, want none", mapping.Edges)
	}
	assertHasCodes(t, mapping.Warnings, []string{warningAmbiguityRate}, nil)
}
