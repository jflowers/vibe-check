package ripwireadapter

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/zero-dot-force/vibe-check/metrics"
)

// memberImport is one imported name of a Python `from Base import Name`
// statement. ripwire's --deps records such a statement as the include
// target Base alone, dropping Name; the member-import light parse
// recovers Name so a wide-form subpackage import (`from pkg import
// subpkg`) can contribute its directory edge to cycle derivation
// (openspec/changes/wide-form-cycle-edges design D2). Relative bases
// (`from .. import sub`) are kept too (openspec/changes/
// relative-import-edges design D3).
type memberImport struct {
	// Base is the module name after `from`, verbatim: an absolute dotted
	// name (pkg.sub) or a relative one (".", "..", ".sub", "..pkg.sub"),
	// spelled exactly as ripwire's --deps records it in <inc t=>.
	Base string
	// Name is one imported name, with any `as` alias removed.
	Name string
}

// pyFromImportRegex matches one logical `from BASE import NAMES`
// statement. BASE is absolute (`pkg.sub`) or relative: one or more dots,
// optionally followed by a dotted name (`.`, `..`, `.sub`, `..pkg.sub`).
var pyFromImportRegex = regexp.MustCompile(`^\s*from\s+(\.+(?:[A-Za-z_][A-Za-z0-9_.]*)?|[A-Za-z_][A-Za-z0-9_.]*)\s+import\s+(.+?)\s*$`)

// pyIdentifierRegex matches a plain ASCII Python identifier.
var pyIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// pythonMemberImportScan walks the crawl root's Python sources and returns,
// per root-relative slash-form file path (the same form as ripwire's
// --deps p= attribute), the sorted, de-duplicated member imports of every
// `from Base import names` statement (absolute or relative base) in the
// file. Files without such statements are absent from the map.
//
// Directories ripwire prunes (the shared denylist) are skipped, matching
// the coupling data's view; the crawl root itself is never skipped.
// Degradation follows the other light parses: an unreadable file or
// directory produces a parse-error warning and contributes no member
// imports; only a missing or non-directory crawl root fails closed.
func pythonMemberImportScan(crawlRoot string) (map[string][]memberImport, []metrics.Warning, error) {
	info, err := os.Stat(crawlRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("python member-import scan: stat %s: %w", crawlRoot, err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("python member-import scan: %s: %w", crawlRoot, errNotDirectory)
	}

	deny := make(map[string]bool, len(crawlDenylistNames))
	for _, name := range crawlDenylistNames {
		deny[name] = true
	}

	var warnings []metrics.Warning
	results := make(map[string][]memberImport)
	err = filepath.WalkDir(crawlRoot, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, unreadablePathWarning(crawlRoot, filePath, walkErr))
			return nil // skip the unreadable subtree
		}
		if d.IsDir() {
			if filePath != crawlRoot && deny[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".py" {
			return nil
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			warnings = append(warnings, unreadablePathWarning(crawlRoot, filePath, err))
			return nil // no member imports for this file
		}
		members := scanPythonMemberImports(string(data))
		if len(members) == 0 {
			return nil
		}
		rel, err := filepath.Rel(crawlRoot, filePath)
		if err != nil {
			return fmt.Errorf("python member-import scan: relative path of %s: %w", filePath, err)
		}
		results[filepath.ToSlash(rel)] = members
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return results, warnings, nil
}

// scanPythonMemberImports extracts the member imports of every
// `from Base import names` statement in Python source text, sorted by
// (Base, Name) and de-duplicated. Base is kept verbatim, relative bases
// included.
//
// The scan is light — a lexical pass, not a parser:
//   - comments are dropped and string literals (single- or triple-quoted,
//     with backslash escapes) are blanked, so import-like text inside a
//     docstring or string never matches;
//   - physical lines are joined into logical lines across backslash
//     continuations and open brackets, so parenthesized multi-line name
//     lists are read whole;
//   - logical lines are split on `;`, and indented statements (inside
//     `if TYPE_CHECKING:`, `try:`, or a function body) count — a
//     conditional import is still an import dependency.
//
// Names drop an `as` alias; `*` and anything that is not a plain ASCII
// identifier are ignored. A malformed base (an empty dotted component, as
// in `pkg..x` or `.a.`) is skipped. LF, CRLF, and lone-CR line endings
// are recognized. Limitations, documented rather than silently wrong:
// non-ASCII identifiers are skipped; a `;` inside brackets splits a
// statement (no supported import idiom does that); a relative base
// written without a space before `import` (`from .import x`) is not
// matched; and syntax the lexer does not model (nested quotes in Python
// 3.12 f-strings, an unbalanced bracket in a file with a syntax error) can
// mis-join lines.
// The mapping's anchoring rule keeps such misreads from producing edges
// unless they also match a recorded include and a module directory.
func scanPythonMemberImports(source string) []memberImport {
	seen := make(map[memberImport]bool)
	var members []memberImport
	for _, logical := range pythonLogicalLines(source) {
		for _, statement := range strings.Split(logical, ";") {
			match := pyFromImportRegex.FindStringSubmatch(statement)
			if match == nil {
				continue
			}
			base := match[1]
			if dotted := strings.TrimLeft(base, "."); dotted != "" &&
				(strings.HasSuffix(dotted, ".") || strings.Contains(dotted, "..")) {
				continue // malformed: empty dotted component
			}
			names := strings.TrimSpace(match[2])
			names = strings.TrimSuffix(strings.TrimPrefix(names, "("), ")")
			for _, entry := range strings.Split(names, ",") {
				fields := strings.Fields(entry)
				if len(fields) == 0 {
					continue // trailing comma
				}
				name := fields[0]
				plain := len(fields) == 1
				aliased := len(fields) == 3 && fields[1] == "as"
				if !plain && !aliased {
					continue
				}
				if !pyIdentifierRegex.MatchString(name) {
					continue // `*` or malformed
				}
				member := memberImport{Base: base, Name: name}
				if !seen[member] {
					seen[member] = true
					members = append(members, member)
				}
			}
		}
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].Base != members[j].Base {
			return members[i].Base < members[j].Base
		}
		return members[i].Name < members[j].Name
	})
	return members
}

// pythonLogicalLines splits Python source into logical lines with
// comments removed and string-literal contents blanked (each literal is
// replaced by an empty `""`). Backslash-newline and newlines inside open
// brackets join physical lines with a single space.
func pythonLogicalLines(source string) []string {
	var lines []string
	var current strings.Builder
	depth := 0
	flush := func() {
		if strings.TrimSpace(current.String()) != "" {
			lines = append(lines, current.String())
		}
		current.Reset()
	}

	for i := 0; i < len(source); i++ {
		c := source[i]
		switch {
		case c == '#':
			for i+1 < len(source) && source[i+1] != '\n' && source[i+1] != '\r' {
				i++
			}
		case c == '\\' && i+1 < len(source) && source[i+1] == '\n':
			current.WriteByte(' ')
			i++
		case c == '\\' && i+2 < len(source) && source[i+1] == '\r' && source[i+2] == '\n':
			current.WriteByte(' ')
			i += 2
		case c == '"' || c == '\'':
			i = skipPythonString(source, i)
			current.WriteString(`""`)
		case c == '(' || c == '[' || c == '{':
			depth++
			current.WriteByte(c)
		case c == ')' || c == ']' || c == '}':
			if depth > 0 {
				depth--
			}
			current.WriteByte(c)
		case c == '\n' || (c == '\r' && (i+1 >= len(source) || source[i+1] != '\n')):
			// LF, or a lone CR (classic Mac line ending, which Python
			// accepts). The CR of a CRLF pair is dropped below and its LF
			// ends the line.
			if depth > 0 {
				current.WriteByte(' ')
				continue
			}
			flush()
		case c == '\r':
			// Dropped: a CRLF newline is handled by its '\n'.
		default:
			current.WriteByte(c)
		}
	}
	flush()
	return lines
}

// skipPythonString returns the index of the last byte of the string
// literal whose opening quote is at start. Triple-quoted literals end at
// the matching triple quote; single-quoted literals end at the matching
// quote or, if unterminated, before the end of the physical line.
// Backslash escapes are honored. An unterminated triple-quoted literal
// runs to the end of the source.
func skipPythonString(source string, start int) int {
	quote := source[start]
	triple := start+2 < len(source) && source[start+1] == quote && source[start+2] == quote
	i := start + 1
	if triple {
		i = start + 3
	}
	for i < len(source) {
		switch c := source[i]; {
		case c == '\\':
			i += 2
			continue
		case triple && c == quote && i+2 < len(source) && source[i+1] == quote && source[i+2] == quote:
			return i + 2
		case !triple && c == quote:
			return i
		case !triple && (c == '\n' || c == '\r'):
			return i - 1
		}
		i++
	}
	return len(source) - 1
}
