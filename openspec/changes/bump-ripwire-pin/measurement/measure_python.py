#!/usr/bin/env python3
"""Measure ripwire's Python coupling accuracy against an ast ground truth.

openspec/changes/bump-ripwire-pin design D6 / tasks 5.1-5.2. Spec-phase
evidence, never built or run by CI. Run with `python3 -I` (isolated mode)
so nothing in the measured tree can shadow a standard-library module.

    python3 -I measure_python.py <ripwire-binary> <crawl-root> [--json]

<crawl-root> is the directory ripwire crawls (for a src-layout project, its
src/ directory), exactly as the adapter would choose it.

Ground truth: every .py file under the crawl root is parsed with ast (never
imported or executed), and every import statement is resolved to in-tree
files with Python's own rules:

  * `import a.b.c [as x]` -> the file of the deepest in-tree module among
    a.b.c, a.b, a (a/b/c.py or a/b/c/__init__.py).
  * `from X import n` (X absolute or relative) -> X's file (the package or
    module imported from always loads) plus X.n's file when X.n is an
    in-tree module or package (a member edge): the "wide form" upstream
    adopted in redhat-et/ripwire#533. `*` -> X only.
  * imports anywhere in the file count (module level, functions, classes,
    TYPE_CHECKING blocks): the static import graph, as ripwire reads it.
  * targets outside the tree (stdlib, third-party) are not edges.
  * directories ripwire's crawl denylist prunes are excluded from both
    sides: their absence is disclosed separately (denylist-pruned warning),
    not an accuracy error.

Measured (ripwire side):

  * directory grain, what vibe-check consumes: `--arch` <m> rows ca/ce vs
    ground-truth Ca/Ce per directory module -> % exact and mean absolute
    error. This is the Phase 0 spike's headline metric (pip @ 1000636 on
    0.6.9: Ca 81% exact / MAE 0.19, Ce 85% / MAE 0.21).
  * file grain, count-based: `--export=cc.json` per-file fan_out vs the
    ground-truth per-file out-degree. lost = sum(max(0, gt - rw)) / sum(gt);
    spurious = sum(max(0, rw - gt)) / sum(gt). A file that both loses and
    gains an edge nets to zero, so both figures are LOWER BOUNDS on the true
    edge-level rates. (The spike reconstructed edge identities from --deps
    <inc t=> text; since redhat-et/ripwire#533 one written import can
    resolve to two files, so that reconstruction no longer holds.)
"""

import ast
import json
import os
import subprocess
import sys
import xml.etree.ElementTree as ET

# ripwire's crawl denylist (pinned release): directories pruned by name.
DENYLIST = {"build", "dist", "out", "target", "vendor", "third_party", "captures"}


def python_files(root):
    """Yield crawl-root-relative paths of .py files, pruning like ripwire."""
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(
            d for d in dirnames if d not in DENYLIST and not d.startswith(".")
        )
        for name in sorted(filenames):
            if name.endswith(".py"):
                yield os.path.relpath(os.path.join(dirpath, name), root).replace(os.sep, "/")


def module_name(rel):
    """Dotted module name for a crawl-root-relative .py path."""
    parts = rel[:-3].split("/")
    if parts[-1] == "__init__":
        parts = parts[:-1]
    return ".".join(parts)


def ground_truth(root):
    """Return (files, edges): in-tree files and resolved (src, dst) file edges."""
    files = list(python_files(root))
    by_module = {module_name(f): f for f in files}
    edges = set()
    parse_errors = []

    for rel in files:
        try:
            with open(os.path.join(root, rel), "rb") as handle:
                tree = ast.parse(handle.read(), filename=rel)
        except (SyntaxError, ValueError) as exc:
            parse_errors.append(f"{rel}: {exc}")
            continue
        is_package = rel.endswith("/__init__.py") or rel == "__init__.py"
        own = module_name(rel)
        package = own if is_package else own.rpartition(".")[0]

        def deepest(dotted):
            parts = dotted.split(".")
            for i in range(len(parts), 0, -1):
                hit = by_module.get(".".join(parts[:i]))
                if hit:
                    return hit
            return None

        for node in ast.walk(tree):
            targets = []
            if isinstance(node, ast.Import):
                for alias in node.names:
                    targets.append(deepest(alias.name))
            elif isinstance(node, ast.ImportFrom):
                if node.level:
                    anchor = package.split(".") if package else []
                    up = node.level - 1
                    if up > len(anchor):
                        continue  # relative import escapes the tree
                    base_parts = anchor[: len(anchor) - up] if up else anchor
                    if node.module:
                        base_parts = base_parts + node.module.split(".")
                    base = ".".join(base_parts)
                else:
                    base = node.module or ""
                if base:
                    # The package (or module) imported from always loads.
                    targets.append(by_module.get(base) or deepest(base))
                for alias in node.names:
                    member = f"{base}.{alias.name}" if base else alias.name
                    if alias.name != "*" and member in by_module:
                        targets.append(by_module[member])
            for dst in targets:
                if dst and dst != rel:
                    edges.add((rel, dst))
    return files, edges, parse_errors


def run(ripwire, root, *args):
    out = subprocess.run(
        [ripwire, ".", *args, "--no-cache"],
        cwd=root, capture_output=True, check=True,
    )
    return out.stdout


def ripwire_dirs(ripwire, root, rules):
    xml = run(ripwire, root, f"--arch={rules}")
    arch = ET.fromstring(xml.decode())
    rows = {}
    for m in arch.iter("m"):
        path = m.get("path").removeprefix("./")
        rows[path] = (int(m.get("ca", 0)), int(m.get("ce", 0)), m.get("zone"))
    return rows


def ripwire_fan_out(ripwire, root):
    data = json.loads(run(ripwire, root, "--export=cc.json"))
    fan = {}

    def walk(node, prefix):
        path = f"{prefix}/{node['name']}" if prefix else node["name"]
        if node.get("type") == "File":
            fan[path.split("/", 1)[1]] = node["attributes"].get("fan_out", 0)
        for child in node.get("children", []):
            walk(child, path)

    for node in data["nodes"]:
        walk(node, "")
    return fan


def directory_metrics(files, edges):
    dirs = {os.path.dirname(f) or "." for f in files}
    ca = {d: set() for d in dirs}
    ce = {d: set() for d in dirs}
    for src, dst in edges:
        a, b = os.path.dirname(src) or ".", os.path.dirname(dst) or "."
        if a != b:
            ce[a].add(b)
            ca[b].add(a)
    return {d: (len(ca[d]), len(ce[d])) for d in dirs}


def main():
    if len(sys.argv) < 3:
        sys.exit(__doc__)
    ripwire, root = sys.argv[1], os.path.abspath(sys.argv[2])
    as_json = "--json" in sys.argv[3:]
    here = os.path.dirname(os.path.abspath(__file__))
    rules = os.path.join(here, "arch-rules.txt")

    version = subprocess.run([ripwire, "--version"], capture_output=True, text=True, check=True).stdout.strip()
    files, edges, parse_errors = ground_truth(root)
    gt_dirs = directory_metrics(files, edges)
    rw_dirs = ripwire_dirs(ripwire, root, rules)
    rw_fan = ripwire_fan_out(ripwire, root)

    # Directory grain: compare on directories holding ground-truth files.
    compared = sorted(d for d in gt_dirs if d in rw_dirs)
    missing = sorted(d for d in gt_dirs if d not in rw_dirs)
    ca_exact = sum(1 for d in compared if rw_dirs[d][0] == gt_dirs[d][0])
    ce_exact = sum(1 for d in compared if rw_dirs[d][1] == gt_dirs[d][1])
    ca_mae = sum(abs(rw_dirs[d][0] - gt_dirs[d][0]) for d in compared) / max(1, len(compared))
    ce_mae = sum(abs(rw_dirs[d][1] - gt_dirs[d][1]) for d in compared) / max(1, len(compared))
    unknown = sum(1 for d in compared if rw_dirs[d][2] == "unknown")

    # File grain, count-based.
    gt_out = {f: 0 for f in files}
    for src, _ in edges:
        gt_out[src] += 1
    total = sum(gt_out.values())
    lost = sum(max(0, gt_out[f] - rw_fan.get(f, 0)) for f in files)
    spurious = sum(max(0, rw_fan.get(f, 0) - gt_out[f]) for f in files)

    worst = sorted(
        compared,
        key=lambda d: -(abs(rw_dirs[d][0] - gt_dirs[d][0]) + abs(rw_dirs[d][1] - gt_dirs[d][1])),
    )[:10]
    result = {
        "ripwire": version,
        "crawl_root": root,
        "files": len(files),
        "parse_errors": parse_errors,
        "gt_edges": total,
        "dirs_compared": len(compared),
        "dirs_missing_from_arch": missing,
        "dirs_zone_unknown": unknown,
        "ca_exact_pct": round(100 * ca_exact / max(1, len(compared)), 1),
        "ca_mae": round(ca_mae, 2),
        "ce_exact_pct": round(100 * ce_exact / max(1, len(compared)), 1),
        "ce_mae": round(ce_mae, 2),
        "file_lost_pct_lower_bound": round(100 * lost / max(1, total), 2),
        "file_spurious_pct_lower_bound": round(100 * spurious / max(1, total), 2),
        "worst_dirs": [
            {"dir": d, "gt_ca_ce": gt_dirs[d], "rw_ca_ce": rw_dirs[d][:2]} for d in worst
            if gt_dirs[d] != rw_dirs[d][:2]
        ],
    }
    if as_json:
        print(json.dumps(result, indent=2, sort_keys=True))
        return
    print(f"ripwire: {version}")
    print(f"files={result['files']} gt_edges={total} dirs={len(compared)} "
          f"missing_from_arch={len(missing)} zone_unknown={unknown} parse_errors={len(parse_errors)}")
    print(f"Ca exact {result['ca_exact_pct']}% (MAE {result['ca_mae']})  "
          f"Ce exact {result['ce_exact_pct']}% (MAE {result['ce_mae']})")
    print(f"file edges (count-based lower bounds): lost {result['file_lost_pct_lower_bound']}%  "
          f"spurious {result['file_spurious_pct_lower_bound']}%")
    for w in result["worst_dirs"]:
        print(f"  {w['dir']}: gt ca/ce {w['gt_ca_ce']} vs ripwire {w['rw_ca_ce']}")


if __name__ == "__main__":
    main()
