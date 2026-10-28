"""Deterministic source selection from AST/index metadata, independent of Reffector."""
from __future__ import annotations

import hashlib
import json
import re
from collections import defaultdict
from pathlib import Path, PurePosixPath
from typing import Any

_WORD = re.compile(r"[A-Za-z][A-Za-z0-9_]{2,}")


def _terms(value: Any) -> set[str]:
    if isinstance(value, dict):
        return set().union(*(_terms(key) | _terms(item) for key, item in value.items())) if value else set()
    if isinstance(value, list):
        return set().union(*(_terms(item) for item in value)) if value else set()
    return {token.lower() for token in _WORD.findall(str(value or ""))}


def select_source_context(
    question: dict[str, Any], target: dict[str, Any], source_model: dict[str, Any],
    source_manifest: dict[str, dict[str, Any]], source_root: Path, budget_bytes: int,
    max_import_depth: int = 1, max_neighbor_declarations: int = 8,
) -> dict[str, Any]:
    """Return exact hash-checked declaration excerpts plus explicit deferred refs."""
    if not isinstance(question, dict) or not isinstance(question.get("prompt"), str):
        raise ValueError("source selection requires a question prompt")
    if not isinstance(target, dict) or not isinstance(target.get("packages"), list):
        raise ValueError("source selection requires a package target")
    if not isinstance(source_model, dict) or not isinstance(source_model.get("declarations"), list):
        raise ValueError("source selection requires an AST declaration index")
    if not isinstance(source_manifest, dict) or not source_manifest:
        raise ValueError("source selection requires the canonical source manifest")
    if not isinstance(budget_bytes, int) or isinstance(budget_bytes, bool) or budget_bytes <= 0:
        raise ValueError("source selection budget must be positive")
    if max_import_depth < 0 or max_neighbor_declarations < 0:
        raise ValueError("source traversal limits must be nonnegative")

    packages = set(target["packages"])
    files = set(target.get("files", source_manifest))
    query = _terms(question["prompt"] + " " + str(question.get("id", "")))
    declarations = [row for row in source_model["declarations"]
                    if isinstance(row, dict) and row.get("package") in packages and row.get("file") in files]
    by_package: dict[str, list[dict[str, Any]]] = defaultdict(list)
    by_id = {}
    for row in declarations:
        by_package[str(row.get("package"))].append(row)
        by_id[row.get("id")] = row

    file_rows = {row.get("path"): row for row in source_model.get("files", []) if isinstance(row, dict)}
    import_adjacency: dict[str, set[str]] = defaultdict(set)
    for package, rows in by_package.items():
        imports = {imp for row in rows for imp in file_rows.get(row.get("file"), {}).get("imports", []) if isinstance(imp, str)}
        for other in packages:
            if other in imports:
                import_adjacency[package].add(other)

    hits_by_ref: dict[str, int] = {}
    for row in declarations:
        hits_by_ref[str(row.get("id"))] = len(query & _terms(row))
    matched_packages = {str(row.get("package")) for row in declarations if hits_by_ref.get(str(row.get("id")), 0) > 0}
    seeds = [row for row in declarations if hits_by_ref.get(str(row.get("id")), 0) > 0]
    # Preserve bounded structural coverage without making every exported function
    # in the four-package scope an unconditional source excerpt.
    for package in sorted(packages):
        entry_points = sorted(
            (row for row in declarations if row.get("package") == package
             and row.get("exported") and row.get("kind") == "function"),
            key=lambda row: (-hits_by_ref.get(str(row.get("id")), 0), row.get("file", ""),
                             row.get("start_line", 0), str(row.get("id"))),
        )
        seeds.extend(entry_points[:2])
    reached_packages = set(matched_packages)
    frontier = sorted(matched_packages)
    for _ in range(max_import_depth):
        next_frontier = []
        for package in frontier:
            for neighbor in sorted(import_adjacency.get(package, set())):
                if neighbor not in reached_packages:
                    reached_packages.add(neighbor)
                    next_frontier.append(neighbor)
        frontier = next_frontier
    neighborhood = []
    for package in sorted(reached_packages - matched_packages):
        choices = sorted((row for row in by_package.get(package, [])
                          if row.get("exported") or row.get("kind") == "type"),
                         key=lambda row: (-hits_by_ref.get(str(row.get("id")), 0), row.get("file", ""), row.get("start_line", 0), row.get("id", "")))
        neighborhood.extend(choices[:max_neighbor_declarations])
    candidates = {str(row.get("id")): row for row in [*seeds, *neighborhood] if isinstance(row.get("id"), str)}

    ranked = sorted(candidates.values(), key=lambda row: (
        -hits_by_ref.get(str(row.get("id")), 0),
        0 if row.get("kind") == "type" and row.get("type_kind") == "interface" else 1,
        0 if row.get("exported") else 1,
        0 if row.get("has_body") else 1,
        row.get("file", ""), row.get("start_line", 0), str(row.get("id")),
    ))
    selected = []
    deferred = []
    used = 0
    for declaration in ranked:
        ref = str(declaration["id"])
        path = declaration.get("file")
        info = source_manifest.get(path)
        start, end = declaration.get("start_line"), declaration.get("end_line")
        if not isinstance(info, dict) or not isinstance(start, int) or not isinstance(end, int) or start < 1 or end < start:
            raise ValueError(f"invalid declaration/source reference: {ref}")
        pure = PurePosixPath(str(path))
        if pure.is_absolute() or ".." in pure.parts:
            raise ValueError(f"unsafe source manifest path: {path}")
        actual = (source_root / pure).resolve(strict=True)
        root = source_root.resolve(strict=True)
        if not actual.is_relative_to(root):
            raise ValueError(f"source path escapes pinned source root: {path}")
        raw = actual.read_bytes()
        sha = hashlib.sha256(raw).hexdigest()
        if sha != info.get("sha256"):
            raise ValueError(f"source hash mismatch for {path}")
        lines = raw.decode("utf-8").splitlines()
        if end > len(lines):
            raise ValueError(f"AST declaration range exceeds source file: {ref}")
        text = "\n".join(lines[start - 1:end])
        size = len(text.encode("utf-8"))
        if used + size > budget_bytes:
            deferred.append({"ref": ref, "file": path, "start_line": start, "end_line": end, "reason": "source_excerpt_budget"})
            continue
        selected.append({"ref": ref, "kind": declaration.get("kind"), "symbol": declaration.get("symbol"),
                         "package": declaration.get("package"), "file": path, "start_line": start, "end_line": end,
                         "sha256": sha, "text": text, "bytes": size,
                         "reason": "lexical_match" if hits_by_ref.get(ref, 0) else
                                   "bounded_import_neighborhood" if declaration in neighborhood else "exported_entry_point",
                         "relevance_hits": hits_by_ref.get(ref, 0), "has_body": declaration.get("has_body", False),
                         "type_kind": declaration.get("type_kind"), "interface_methods": declaration.get("interface_methods", [])})
        used += size

    selected.sort(key=lambda row: (row["file"], row["start_line"], row["ref"]))
    return {
        "schema_version": 1,
        "question_id": question.get("id"),
        "unit": target.get("id"),
        "available_declaration_count": len(declarations),
        "candidate_declaration_count": len(candidates),
        "selected": selected,
        "deferred": deferred,
        "not_selected_refs": sorted(str(row.get("id")) for row in declarations if row.get("id") not in candidates),
        "budget_bytes": budget_bytes,
        "selected_bytes": used,
        "traversal": {"basis": "AST lexical symbols, package imports, exported entry points, interfaces; no Reffector graph used for shared source selection",
                       "max_import_depth": max_import_depth,
                       "max_neighbor_declarations_per_package": max_neighbor_declarations,
                       "reached_packages": sorted(reached_packages),
                       "relationship_gap": "AST index has no complete function call graph; imports are package dependencies, not calls."},
        "source_hashes": {path: source_manifest[path]["sha256"] for path in sorted({row.get("file") for row in selected})},
        "retrieval_log": [{"ref": row["ref"], "file": row["file"], "start_line": row["start_line"],
                           "end_line": row["end_line"], "sha256": row["sha256"], "bytes": row["bytes"]}
                          for row in selected],
        "source_coverage": {"selected_files": sorted({row["file"] for row in selected}),
                            "full_source_files_available": sorted(source_manifest),
                            "full_source_remains_retrievable": True,
                            "truncated": bool(deferred)},
        "uncertainty": ["Selected source is not semantically sufficient by virtue of lexical match or fit.",
                        "Effect-free declarations remain eligible; effect annotations do not filter this selector.",
                        "Import neighborhood is bounded and is not a complete call graph."],
    }
