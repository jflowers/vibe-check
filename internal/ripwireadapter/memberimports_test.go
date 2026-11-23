package ripwireadapter

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zero-dot-force/vibe-check/metrics"
)

func TestScanPythonMemberImports_Forms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []memberImport
	}{
		{
			name:   "single name",
			source: "from pkg import sub\n",
			want:   []memberImport{{Base: "pkg", Name: "sub"}},
		},
		{
			name:   "multiple names sorted",
			source: "from pkg import zeta, alpha\n",
			want:   []memberImport{{Base: "pkg", Name: "alpha"}, {Base: "pkg", Name: "zeta"}},
		},
		{
			name:   "as alias",
			source: "from pkg.inner import sub as s, other\n",
			want:   []memberImport{{Base: "pkg.inner", Name: "other"}, {Base: "pkg.inner", Name: "sub"}},
		},
		{
			name:   "parenthesized multi-line with trailing comma",
			source: "from pkg import (\n    a,\n    b as bee,  # comment\n)\n",
			want:   []memberImport{{Base: "pkg", Name: "a"}, {Base: "pkg", Name: "b"}},
		},
		{
			name:   "backslash continuation",
			source: "from pkg import a, \\\n    b\n",
			want:   []memberImport{{Base: "pkg", Name: "a"}, {Base: "pkg", Name: "b"}},
		},
		{
			name:   "CRLF backslash continuation",
			source: "from pkg import a, \\\r\n    b\r\n",
			want:   []memberImport{{Base: "pkg", Name: "a"}, {Base: "pkg", Name: "b"}},
		},
		{
			name:   "semicolon-joined statements",
			source: "import os; from pkg import a; from other import b\n",
			want:   []memberImport{{Base: "other", Name: "b"}, {Base: "pkg", Name: "a"}},
		},
		{
			name:   "indented conditional import",
			source: "if TYPE_CHECKING:\n    from pkg import sub\n",
			want:   []memberImport{{Base: "pkg", Name: "sub"}},
		},
		{
			name:   "duplicates collapse",
			source: "from pkg import a\nfrom pkg import a as again\n",
			want:   []memberImport{{Base: "pkg", Name: "a"}},
		},
		{
			name:   "star ignored",
			source: "from pkg import *\n",
			want:   nil,
		},
		{
			name:   "relative bases kept verbatim",
			source: "from . import a\nfrom .. import (b, c as cee)\nfrom ..pkg import c\nfrom .sub import d\n",
			want: []memberImport{
				{Base: ".", Name: "a"},
				{Base: "..", Name: "b"},
				{Base: "..", Name: "c"},
				{Base: "..pkg", Name: "c"},
				{Base: ".sub", Name: "d"},
			},
		},
		{
			name:   "malformed relative bases ignored",
			source: "from .a..b import x\nfrom ..x. import y\n",
			want:   nil,
		},
		{
			name:   "relative base without space before import not matched",
			source: "from .import x\n",
			want:   nil,
		},
		{
			name:   "plain import ignored",
			source: "import pkg.sub\nimport pkg.sub as s\n",
			want:   nil,
		},
		{
			name:   "triple-quoted docstring ignored",
			source: "\"\"\"Docs.\n\nfrom pkg import fake\n\"\"\"\nfrom real import x\n",
			want:   []memberImport{{Base: "real", Name: "x"}},
		},
		{
			name:   "single-quoted triple string ignored",
			source: "X = '''\nfrom pkg import fake\n'''\n",
			want:   nil,
		},
		{
			name:   "string literal on its own line ignored",
			source: "s = 'from pkg import fake'\nt = \"from pkg import \\\"fake\\\"\"\n",
			want:   nil,
		},
		{
			name:   "commented-out import ignored",
			source: "# from pkg import fake\n",
			want:   nil,
		},
		{
			name:   "malformed names rejected",
			source: "from pkg import a b, 1bad, ok\n",
			want:   []memberImport{{Base: "pkg", Name: "ok"}},
		},
		{
			name:   "malformed base rejected",
			source: "from pkg. import a\nfrom pkg..x import b\n",
			want:   nil,
		},
		{
			name:   "unterminated string does not swallow later lines",
			source: "s = 'broken\nfrom pkg import sub\n",
			want:   []memberImport{{Base: "pkg", Name: "sub"}},
		},
		{
			name:   "lone CR line endings",
			source: "x = 1\rfrom pkg import a\r# note\rfrom pkg import b\r",
			want:   []memberImport{{Base: "pkg", Name: "a"}, {Base: "pkg", Name: "b"}},
		},
		{
			name:   "escaped quote keeps the string closed",
			source: "s = 'it\\'s'; from pkg import x\n",
			want:   []memberImport{{Base: "pkg", Name: "x"}},
		},
		{
			name:   "empty source",
			source: "",
			want:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scanPythonMemberImports(tc.source)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPythonMemberImportScan_Fixture(t *testing.T) {
	t.Parallel()

	got, warnings, err := pythonMemberImportScan(filepath.Join("testdata", "python", "src"))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings: got %v, want none", warnings)
	}
	want := map[string][]memberImport{
		"fixturepy/cycle_a/__init__.py": {{Base: "fixturepy.cycle_b.api", Name: "helper_b"}},
		"fixturepy/cycle_b/__init__.py": {{Base: "fixturepy.cycle_a.api", Name: "helper_a"}},
		"fixturepy/ring_a/__init__.py":  {{Base: "fixturepy", Name: "ring_b"}},
		"fixturepy/ring_b/__init__.py":  {{Base: "fixturepy.ring_a.api", Name: "marker"}},
		"fixturepy/wide/consumer.py":    {{Base: "fixturepy", Name: "cycle_b"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPythonMemberImportScan_DenylistSkipped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "pkg/a.py", "from pkg import sub\n")
	writeFile(t, root, "pkg/build/b.py", "from pkg import pruned\n")
	writeFile(t, root, "pkg/notes.txt", "from pkg import fake\n")

	got, warnings, err := pythonMemberImportScan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings: got %v, want none", warnings)
	}
	want := map[string][]memberImport{"pkg/a.py": {{Base: "pkg", Name: "sub"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPythonMemberImportScan_UnreadableFile(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)

	root := t.TempDir()
	writeFile(t, root, "pkg/ok.py", "from pkg import sub\n")
	locked := filepath.Join(root, "pkg", "locked.py")
	writeFile(t, root, "pkg/locked.py", "from pkg import hidden\n")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })

	got, warnings, err := pythonMemberImportScan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := map[string][]memberImport{"pkg/ok.py": {{Base: "pkg", Name: "sub"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (the unreadable file contributes nothing)", got, want)
	}
	if len(warnings) != 1 || warnings[0].Code != warningParseError {
		t.Fatalf("warnings: got %v, want one %s", warnings, warningParseError)
	}
	if !strings.Contains(warnings[0].Message, "pkg/locked.py") {
		t.Errorf("warning message: got %q, want it to name pkg/locked.py", warnings[0].Message)
	}
	if status := graphStatusForWarnings(warnings); status != metrics.StatusPartial {
		t.Errorf("status: got %q, want %q", status, metrics.StatusPartial)
	}
}

func TestPythonMemberImportScan_BadRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	file := filepath.Join(root, "file.py")
	writeFile(t, root, "file.py", "")

	if _, _, err := pythonMemberImportScan(file); !errors.Is(err, errNotDirectory) {
		t.Errorf("file root: got %v, want errNotDirectory", err)
	}
	if _, _, err := pythonMemberImportScan(filepath.Join(root, "missing")); err == nil {
		t.Error("missing root: got nil error, want error")
	}
}
