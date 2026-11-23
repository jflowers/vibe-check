package ripwireadapter

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
			name:   "relative bases ignored",
			source: "from . import a\nfrom .. import b\nfrom ..pkg import c\nfrom .x import d\n",
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

	got, _, err := pythonMemberImportScan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := map[string][]memberImport{"pkg/a.py": {{Base: "pkg", Name: "sub"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPythonMemberImportScan_UnreadableFile(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}

	root := t.TempDir()
	writeFile(t, root, "pkg/ok.py", "from pkg import sub\n")
	locked := filepath.Join(root, "pkg", "locked.py")
	writeFile(t, root, "pkg/locked.py", "from pkg import hidden\n")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	got, warnings, err := pythonMemberImportScan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if _, ok := got["pkg/locked.py"]; ok {
		t.Errorf("unreadable file contributed member imports: %v", got)
	}
	if len(warnings) != 1 || warnings[0].Code != warningParseError {
		t.Errorf("warnings: got %v, want one %s", warnings, warningParseError)
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
