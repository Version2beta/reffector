"""Deterministic program/evidence indexing and compact model-context views."""
from __future__ import annotations

import hashlib
import json
from collections import Counter, defaultdict
from typing import Any

SCHEMA_VERSION = 1


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode("utf-8")


def stable_id(prefix: str, value: Any) -> str:
    return f"{prefix}:{hashlib.sha256(canonical_json(value)).hexdigest()[:24]}"


def _location(record: dict[str, Any]) -> dict[str, Any]:
    value = record.get("location")
    return value if isinstance(value, dict) else {}


def _normalize_site(record: dict[str, Any], repo_file: str | None, audit: dict[str, Any] | None, raw_ref: str) -> dict[str, Any]:
    location = _location(record)
    audit = audit if isinstance(audit, dict) else None
    operations = record.get("operations", [])
    normalized_ops = []
    for operation in operations if isinstance(operations, list) else []:
        if isinstance(operation, dict):
            normalized_ops.append({
                "api": operation.get("api"),
                "effects": operation.get("effects", []),
                "possible_effects": operation.get("possible_effects", []),
                "uncertain": operation.get("uncertain", False),
            })
    return {
        "id": record.get("id"),
        "package": record.get("package"),
        "function": record.get("function"),
        "kind": record.get("kind"),
        "api": record.get("api"),
        "apis": record.get("apis", []),
        "operations": normalized_ops,
        "effects": record.get("effects", []),
        "possible_effects": audit.get("possible_effects", record.get("possible_effects", [])) if audit else record.get("possible_effects", []),
        "uncertain": bool(record.get("uncertain", False) or (audit and audit.get("uncertain", False))),
        "uncertainty_reasons": audit.get("uncertainty_reasons", record.get("uncertainty_reasons", [])) if audit else record.get("uncertainty_reasons", []),
        "recognition": record.get("recognition"),
        "primary_reason": audit.get("primary_reason", record.get("primary_reason")) if audit else record.get("primary_reason"),
        "dispatch_kinds": audit.get("dispatch_kinds", record.get("dispatch_kinds", [])) if audit else record.get("dispatch_kinds", []),
        "unknown_api": audit.get("unknown_api", record.get("unknown_api", False)) if audit else record.get("unknown_api", False),
        "candidate_count": audit.get("vta_candidate_count", record.get("vta_candidate_count", 0)) if audit else record.get("vta_candidate_count", 0),
        "candidate_ids": [candidate.get("id") for candidate in (audit or record).get("vta_candidates", []) if isinstance(candidate, dict) and isinstance(candidate.get("id"), str)],
        "ssa_node_count": len((audit or record).get("ssa_nodes", [])) if isinstance((audit or record).get("ssa_nodes", []), list) else 0,
        "source_ref": {"file": repo_file, "line": location.get("line"), "column": location.get("column")},
        "raw_refs": [raw_ref] + ([f"raw-audit.jsonl#{audit.get('_raw_line')}" ] if audit and audit.get("_raw_line") else []),
    }


def build_evidence_index(
    effects: dict[str, Any],
    audit_index: list[dict[str, Any]],
    edge_records: list[dict[str, Any]],
    node_records: dict[str, dict[str, Any]],
    repo_file_for_location,
    revision: dict[str, Any],
    source_manifest_sha256: str,
) -> dict[str, Any]:
    """Build one lossless-by-reference normalized index over existing raw exports."""
    audit_by_id: dict[str, dict[str, Any]] = {}
    for item in audit_index:
        record = item.get("record")
        if isinstance(record, dict) and record.get("record_type") == "site" and isinstance(record.get("id"), str):
            indexed = dict(record)
            indexed["_raw_line"] = item.get("raw_line")
            audit_by_id[record["id"]] = indexed

    records: list[dict[str, Any]] = []
    site_to_package: dict[str, str] = {}
    for ordinal, raw in enumerate(effects.get("sites", []), start=1):
        if not isinstance(raw, dict) or not isinstance(raw.get("id"), str):
            continue
        identifier = raw["id"]
        file = repo_file_for_location(_location(raw).get("file"))
        audit = audit_by_id.get(identifier)
        if isinstance(raw.get("package"), str):
            site_to_package[identifier] = raw["package"]
        normalized = _normalize_site(raw, file, audit, f"raw-effects.json#/sites/{ordinal - 1}")
        records.append({
            "ref": f"effect-site:{identifier}",
            "aliases": [identifier],
            "record_type": "effect_site",
            "package": raw.get("package"),
            "file": file,
            "source_ref": normalized["source_ref"],
            "data": normalized,
        })

    for item in audit_index:
        raw = item.get("record")
        if not isinstance(raw, dict):
            continue
        record_type = raw.get("record_type")
        if record_type == "site" and isinstance(raw.get("id"), str):
            file = next(iter(item.get("repo_files", [])), None)
            location = _location(raw)
            candidate_records = raw.get("vta_candidates", [])
            candidates = [candidate for candidate in candidate_records if isinstance(candidate, dict)] if isinstance(candidate_records, list) else []
            normalized = {
                "id": raw["id"], "package": raw.get("package"), "function": raw.get("function"), "kind": raw.get("kind"),
                "api": raw.get("api"), "apis": raw.get("apis", []), "effects": raw.get("effects", []),
                "possible_effects": raw.get("possible_effects", []), "uncertain": raw.get("uncertain", False),
                "primary_reason": raw.get("primary_reason"), "uncertainty_reasons": raw.get("uncertainty_reasons", []),
                "dispatch_kinds": raw.get("dispatch_kinds", []), "unknown_api": raw.get("unknown_api", False),
                "candidate_count": raw.get("vta_candidate_count", len(candidates)),
                "candidate_ids": [candidate.get("id") for candidate in candidates if isinstance(candidate.get("id"), str)],
                "candidate_records": candidates,
                "ssa_nodes": raw.get("ssa_nodes", []),
                "source_ref": {"file": file, "line": location.get("line"), "column": location.get("column")},
                "raw_refs": [f"raw-audit.jsonl#{item.get('raw_line')}"],
            }
            records.append({"ref": f"audit-site:{raw['id']}", "aliases": [raw["id"]], "record_type": "audit_site", "package": raw.get("package"), "file": file, "source_ref": normalized["source_ref"], "data": normalized})
        elif record_type == "function":
            locations = []
            for location in raw.get("locations", []) if isinstance(raw.get("locations"), list) else []:
                if isinstance(location, dict):
                    path = repo_file_for_location(location.get("file"))
                    if path:
                        locations.append({"file": path, "line": location.get("line"), "column": location.get("column")})
            key = {"package": raw.get("package"), "function": raw.get("function"), "locations": locations}
            ref = stable_id("function-audit", key)
            normalized = {
                "record_type": "function", "package": raw.get("package"), "function": raw.get("function"),
                "effects": raw.get("effects", []), "possible_effects": raw.get("possible_effects", []),
                "uncertain": raw.get("uncertain", False), "source_locations": locations,
                "raw_refs": [f"raw-audit.jsonl#{item.get('raw_line')}"],
            }
            records.append({"ref": ref, "aliases": [], "record_type": "function_audit", "package": raw.get("package"), "file": locations[0]["file"] if locations else None, "source_ref": locations, "data": normalized})

    raw_sites_by_id = {record.get("id"): record for record in effects.get("sites", []) if isinstance(record, dict) and isinstance(record.get("id"), str)}
    for ordinal, raw_unresolved in enumerate(effects.get("unresolved_sites", []), start=1):
        identifier = raw_unresolved.get("id") if isinstance(raw_unresolved, dict) else raw_unresolved if isinstance(raw_unresolved, str) else None
        source_record = audit_by_id.get(identifier) or raw_sites_by_id.get(identifier)
        file = None
        if isinstance(source_record, dict):
            file = repo_file_for_location(_location(source_record).get("file"))
        data = dict(raw_unresolved) if isinstance(raw_unresolved, dict) else {"id": identifier}
        data["raw_ref"] = f"raw-effects.json#/unresolved_sites/{ordinal - 1}"
        if isinstance(source_record, dict):
            data.setdefault("package", source_record.get("package"))
            data.setdefault("function", source_record.get("function"))
            data.setdefault("effects", source_record.get("effects", []))
            data.setdefault("possible_effects", source_record.get("possible_effects", []))
            data.setdefault("uncertain", source_record.get("uncertain", False))
        location = _location(source_record) if isinstance(source_record, dict) else {}
        source_ref = {"file": file, "line": location.get("line"), "column": location.get("column")}
        data["source_ref"] = source_ref
        ref = stable_id("unresolved-site", [identifier, ordinal])
        records.append({"ref": ref, "aliases": [identifier] if isinstance(identifier, str) else [], "record_type": "unresolved_site", "package": data.get("package"), "file": file, "source_ref": source_ref, "data": data})

    for ordinal, edge in enumerate(edge_records, start=1):
        if not isinstance(edge, dict) or not isinstance(edge.get("id"), str):
            continue
        caller = edge.get("caller") if isinstance(edge.get("caller"), dict) else {}
        callee = edge.get("callee") if isinstance(edge.get("callee"), dict) else {}
        data = {
            "id": edge.get("id"), "source_edge_id": edge.get("source_edge_id"),
            "caller_node": edge.get("caller_node"), "callee_node": edge.get("callee_node"),
            "caller": {key: caller.get(key) for key in ("package", "function", "variant") if key in caller},
            "callee": {key: callee.get(key) for key in ("package", "function", "variant") if key in callee},
            "caller_package": edge.get("caller_package"), "callee_package": edge.get("callee_package"),
            "file": edge.get("file"), "location": edge.get("location"), "kind": edge.get("kind"),
            "resolution": edge.get("resolution"), "candidate": edge.get("candidate", False),
            "synthetic": edge.get("synthetic", False), "provenance": "Reffector effect-site reverse evidence; not a complete call graph",
            "raw_ref": f"raw-effects.json#/edges/{ordinal - 1}",
        }
        aliases = [edge["id"]]
        if isinstance(edge.get("source_edge_id"), str):
            aliases.append(edge["source_edge_id"])
        records.append({"ref": f"call-edge:{edge['id']}", "aliases": aliases, "record_type": "call_edge", "package": edge.get("caller_package"), "file": edge.get("file"), "source_ref": edge.get("location"), "data": data})

    node_refs: set[str] = set()
    for edge in edge_records:
        if isinstance(edge, dict):
            node_refs.update(value for value in (edge.get("caller_node"), edge.get("callee_node")) if isinstance(value, str))
    for node_id, raw in sorted(node_records.items()):
        if not isinstance(raw, dict):
            continue
        records.append({"ref": f"node:{node_id}", "aliases": [node_id], "record_type": "node", "package": raw.get("package"), "file": None, "source_ref": None, "data": raw})

    for ordinal, group in enumerate(effects.get("groups", []), start=1):
        if not isinstance(group, dict):
            continue
        value = {"effect": group.get("effect"), "sites": sorted(set(str(item) for item in group.get("sites", []) if isinstance(item, (str, int)))), "possible_sites": sorted(set(str(item) for item in group.get("possible_sites", []) if isinstance(item, (str, int))))}
        ref = stable_id("effect-group", value)
        records.append({"ref": ref, "aliases": [], "record_type": "effect_group", "package": None, "file": None, "source_ref": None, "data": {**value, "raw_ref": f"raw-effects.json#/groups/{ordinal - 1}"}})

    for ordinal, reverse in enumerate(effects.get("reverse", []), start=1):
        if not isinstance(reverse, dict):
            continue
        site = reverse.get("site") or reverse.get("site_id")
        if isinstance(site, dict):
            site = site.get("id")
        if not isinstance(site, str):
            continue
        data = {
            "site": site,
            "origins": reverse.get("origins", []),
            "edges": reverse.get("edges", []),
            "nodes": reverse.get("nodes", []),
            "cycle": reverse.get("cycle", False),
            "revisit": reverse.get("revisit", False),
            "truncated": reverse.get("truncated", {}),
            "raw_ref": f"raw-effects.json#/reverse/{ordinal - 1}",
        }
        records.append({"ref": f"reverse:{site}", "aliases": [], "record_type": "reverse_index", "package": site_to_package.get(site), "file": None, "source_ref": None, "data": data})

    records.sort(key=lambda row: row["ref"])
    return {
        "schema_version": SCHEMA_VERSION,
        "revision": revision,
        "source_manifest_sha256": source_manifest_sha256,
        "raw_sources": {"effects": "raw-effects.json", "audit": "raw-audit.jsonl"},
        "record_count": len(records),
        "records": records,
        "coverage": effects.get("coverage", {}),
        "truncated": effects.get("truncated", {}),
        "scope": "normalized evidence references the preserved raw Reffector exports; edges are effect-site reverse evidence, not a complete call graph",
    }


def _minimal_site(data: dict[str, Any], ref: str) -> dict[str, Any]:
    return {key: data.get(key) for key in (
        "id", "package", "function", "kind", "api", "apis", "operations", "effects", "possible_effects",
        "uncertain", "uncertainty_reasons", "recognition", "primary_reason", "dispatch_kinds", "unknown_api",
        "candidate_count", "candidate_ids", "ssa_node_count", "source_ref", "raw_refs",
    ) if key in data} | {"evidence_ref": ref}


def _minimal_audit_site(data: dict[str, Any], ref: str) -> dict[str, Any]:
    return {key: data.get(key) for key in (
        "id", "package", "function", "kind", "api", "apis", "effects", "possible_effects", "uncertain",
        "primary_reason", "uncertainty_reasons", "dispatch_kinds", "unknown_api", "candidate_count", "candidate_ids", "source_ref", "raw_refs",
    ) if key in data} | {"evidence_ref": ref}


def _minimal_edge(data: dict[str, Any], ref: str) -> dict[str, Any]:
    return {key: data.get(key) for key in (
        "id", "source_edge_id", "caller_node", "callee_node", "caller", "callee", "caller_package", "callee_package",
        "file", "location", "kind", "resolution", "candidate", "synthetic", "provenance",
    ) if key in data} | {"evidence_ref": ref}


def reduce_component_evidence(
    evidence_index: dict[str, Any],
    package_id: str,
    file_paths: list[str],
    selected_packages: set[str],
) -> dict[str, Any]:
    files = set(file_paths)
    records = evidence_index.get("records", [])
    sites_by_id: dict[str, dict[str, Any]] = {}
    audit_sites: list[dict[str, Any]] = []
    function_audits: list[dict[str, Any]] = []
    edges: list[dict[str, Any]] = []
    nodes: dict[str, dict[str, Any]] = {}
    reverse_rows: list[dict[str, Any]] = []
    unresolved_rows: list[dict[str, Any]] = []
    refs: set[str] = set()
    matching_site_ids: set[str] = set()
    edge_node_ids: set[str] = set()
    effect_counts: Counter[str] = Counter()

    for row in records:
        kind = row.get("record_type")
        data = row.get("data", {})
        ref = row.get("ref")
        if kind == "effect_site" and row.get("file") in files and row.get("package") == package_id:
            minimal = _minimal_site(data, ref)
            sites_by_id[str(data.get("id"))] = minimal
            matching_site_ids.add(str(data.get("id")))
            refs.add(ref)
            for effect in data.get("effects", []):
                effect_counts[str(effect)] += 1
            for effect in data.get("possible_effects", []):
                effect_counts["possible:" + str(effect)] += 1
        elif kind == "audit_site" and row.get("file") in files and row.get("package") == package_id:
            audit_sites.append(_minimal_audit_site(data, ref))
            matching_site_ids.add(str(data.get("id")))
            refs.add(ref)
        elif kind == "unresolved_site" and (
            (row.get("file") in files and row.get("package") == package_id)
            or (row.get("file") is None and row.get("package") is None)
        ):
            unresolved_rows.append({key: data.get(key) for key in ("id", "package", "function", "effects", "possible_effects", "uncertain", "source_ref", "raw_ref") if key in data} | {"evidence_ref": ref})
            refs.add(ref)
        elif kind == "function_audit" and row.get("package") == package_id and any(location.get("file") in files for location in data.get("source_locations", []) if isinstance(location, dict)):
            function_audits.append({key: data.get(key) for key in ("package", "function", "effects", "possible_effects", "uncertain", "source_locations", "raw_refs")})
            refs.add(ref)
        elif kind == "call_edge" and (
            row.get("file") in files
            or (row.get("caller_package") in selected_packages and row.get("callee_package") in selected_packages and package_id in (row.get("caller_package"), row.get("callee_package")))
        ):
            edge = _minimal_edge(data, ref)
            edges.append(edge)
            refs.add(ref)
            edge_node_ids.update(value for value in (data.get("caller_node"), data.get("callee_node")) if isinstance(value, str))
        elif kind == "reverse_index" and data.get("site") in matching_site_ids:
            reverse_rows.append({
                "site": data.get("site"), "origin_ids": [origin.get("id") if isinstance(origin, dict) else origin for origin in data.get("origins", [])],
                "edge_count": len(data.get("edges", [])), "node_count": len(data.get("nodes", [])),
                "cycle": data.get("cycle", False), "revisit": data.get("revisit", False), "truncated": data.get("truncated", {}),
                "evidence_ref": ref,
            })
            refs.add(ref)

    # Add reverse summaries only after collecting all selected site IDs.
    for row in records:
        if row.get("record_type") != "reverse_index":
            continue
        data = row.get("data", {})
        if data.get("site") not in matching_site_ids:
            continue
        if any(item.get("site") == data.get("site") for item in reverse_rows):
            continue
        reverse_rows.append({
            "site": data.get("site"), "origin_ids": [origin.get("id") if isinstance(origin, dict) else origin for origin in data.get("origins", [])],
            "edge_count": len(data.get("edges", [])), "node_count": len(data.get("nodes", [])),
            "cycle": data.get("cycle", False), "revisit": data.get("revisit", False), "truncated": data.get("truncated", {}),
            "evidence_ref": row.get("ref"),
        })
        refs.add(row.get("ref"))

    # Audit records with an ID matching an operation site are merged as compact
    # uncertainty metadata, rather than repeated as a second full record.
    audit_by_id = {str(item.get("id")): item for item in audit_sites if item.get("id") is not None}
    for site_id, site in sites_by_id.items():
        audit = audit_by_id.pop(site_id, None)
        if audit:
            site["audit"] = audit
            site["uncertain"] = bool(site.get("uncertain") or audit.get("uncertain"))
    unmatched_audit_sites = list(audit_by_id.values())

    needed_nodes = set(edge_node_ids)
    for reverse in reverse_rows:
        needed_nodes.update(str(item) for item in reverse.get("origin_ids", []) if isinstance(item, str))
    for row in records:
        if row.get("record_type") == "node" and row.get("ref", "").removeprefix("node:") in needed_nodes:
            data = row.get("data", {})
            node_id = data.get("id")
            nodes[str(node_id)] = {key: data.get(key) for key in ("id", "package", "function", "variant", "selected", "synthetic", "uncertain", "direct_uncertainty", "inherited_uncertainty") if key in data}
            refs.add(row.get("ref"))

    for row in records:
        if row.get("record_type") != "effect_group":
            continue
        group = row.get("data", {})
        members = set(str(x) for x in group.get("sites", []) + group.get("possible_sites", []))
        if members & matching_site_ids:
            refs.add(row.get("ref"))

    all_site_records = list(sites_by_id.values()) + unmatched_audit_sites
    candidate_edges = [edge for edge in edges if edge.get("candidate") or edge.get("resolution") != "established"]
    uncertain_site_count = sum(bool(site.get("uncertain")) for site in all_site_records)
    coverage = evidence_index.get("coverage") if isinstance(evidence_index.get("coverage"), dict) else {}
    return {
        "schema_version": SCHEMA_VERSION,
        "scope": "deduplicated component view; complete records remain addressable in evidence-index.json",
        "graph_scope": "effect-site reverse evidence only, not a complete call graph; reachability does not establish runtime execution",
        "operation_sites": sorted(sites_by_id.values(), key=lambda item: str(item.get("id"))),
        "uncertainty_sites": sorted(unmatched_audit_sites, key=lambda item: str(item.get("id"))),
        "function_effect_summaries": sorted(function_audits, key=lambda item: (str(item.get("function")), str(item.get("source_locations")))),
        "call_relationships": sorted(edges, key=lambda item: str(item.get("id"))),
        "candidate_relationship_refs": [edge.get("evidence_ref") for edge in candidate_edges],
        "nodes": [nodes[key] for key in sorted(nodes)],
        "reverse_impact_summaries": sorted(reverse_rows, key=lambda item: str(item.get("site"))),
        "effect_site_counts": dict(sorted(effect_counts.items())),
        "coverage": {
            "selected_files": sorted(files), "operation_sites": len(sites_by_id),
            "audit_uncertainty_sites": len(all_site_records), "unresolved_export_rows": len(unresolved_rows), "uncertain_sites": uncertain_site_count,
            "function_effect_summaries": len(function_audits), "call_relationships": len(edges),
            "candidate_relationships": len(candidate_edges), "nodes": len(nodes),
            "reverse_records": len(reverse_rows), "evidence_refs_available": len(refs),
        },
        "unresolved_coverage": coverage.get("unresolved_sites_identified"),
        "unresolved_site_export_count": len(unresolved_rows),
        "unresolved_site_records": sorted(unresolved_rows, key=lambda item: str(item.get("id"))),
        "unresolved_export_note": "Raw unresolved-site rows remain separately identified from audit candidate/uncertainty records.",
        "available_evidence_refs": sorted(refs),
        "deferred_record_detail_refs": sorted(refs),
    }


def program_for_package(program_model: dict[str, Any], package_id: str, files: list[str]) -> dict[str, Any]:
    selected = set(files)
    declarations = [item for item in program_model.get("declarations", []) if item.get("package") == package_id]
    package = next((item for item in program_model.get("packages", []) if item.get("id") == package_id), {})
    related = [item for item in program_model.get("cross_component_relationships", []) if package_id in (item.get("caller_package"), item.get("callee_package"))]
    all_declarations = [item for item in program_model.get("declarations", []) if item.get("package") == package_id]
    symbol_directory = []
    for item in all_declarations:
        if item.get("kind") not in {"function", "type"}:
            continue
        summary = {key: item.get(key) for key in ("id", "kind", "name", "symbol", "receiver_type", "exported", "file", "start_line", "end_line", "signature", "type_kind") if key in item}
        if item.get("kind") == "type":
            summary["fields"] = [
                {key: field.get(key) for key in ("names", "type", "embedded") if key in field}
                for field in item.get("fields", [])
            ]
            summary["interface_methods"] = item.get("interface_methods", [])
        symbol_directory.append(summary)
    return {
        "package": {key: package.get(key) for key in ("id", "name", "files", "imports", "exported_entry_points", "analysis_coverage") if key in package},
        "source_membership": [item for item in program_model.get("files", []) if item.get("package") == package_id],
        "declarations": [item for item in declarations if item.get("file") in selected],
        "symbol_directory": symbol_directory,
        "cross_component_relationships": related,
        "relationship_scope": "Only Reffector effect-site reverse relationships and Go package imports are represented; this is not a complete call graph or runtime trace.",
    }


def reduce_request(request: dict[str, Any], program_model: dict[str, Any], evidence_index: dict[str, Any], selected_packages: set[str]) -> dict[str, Any]:
    result = dict(request)
    component = request.get("component", {})
    package_id = component.get("package")
    source = request.get("source", [])
    files = [item.get("path") for item in source if isinstance(item, dict) and isinstance(item.get("path"), str)]
    result["program"] = program_for_package(program_model, package_id, files)
    result["evidence"] = reduce_component_evidence(evidence_index, package_id, files, selected_packages)
    result["questions"] = [
        "What responsibility does this component implement, and what behavioral contracts does its source expose? Cite the functions, types, and exact source locations that support the interpretation.",
        "Which interfaces and dependencies define its boundaries? Trace relevant cross-component relationships without treating graph reachability as runtime execution.",
        "Where are state ownership, synchronization, resource lifecycle, and invariants located? Include pure transformations and control flow, not only recognized effects.",
        "Which source or analysis gaps leave behavior uncertain, possible, candidate, or unresolved? State coverage and limitations explicitly.",
    ]
    result["context_representation"] = {
        "schema_version": SCHEMA_VERSION,
        "program_model": "program-model.json",
        "evidence_index": "evidence-index.json",
        "reduction": "Deduplicated references and summaries; original records remain retrievable by available_evidence_refs.",
    }
    return result


def partition_groups(program_model: dict[str, Any], package_id: str, file_paths: list[str], relationships: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Partition declarations by receiver type and established/candidate call adjacency."""
    files = set(file_paths)
    declarations = [item for item in program_model.get("declarations", []) if item.get("package") == package_id and item.get("file") in files]
    symbol_declarations = [item for item in declarations if item.get("kind") == "function"]
    parent: dict[str, str] = {item["id"]: item["id"] for item in symbol_declarations}

    def find(value: str) -> str:
        while parent[value] != value:
            parent[value] = parent[parent[value]]
            value = parent[value]
        return value

    def union(left: str, right: str) -> None:
        a, b = find(left), find(right)
        if a != b:
            parent[max(a, b)] = min(a, b)

    by_symbol: dict[str, str] = {}
    by_simple: dict[str, list[str]] = defaultdict(list)
    for item in symbol_declarations:
        symbol = str(item.get("symbol"))
        by_symbol[symbol] = item["id"]
        by_simple[str(item.get("name"))].append(item["id"])
    # Methods of a receiver form a coherent type unit, even when declared in
    # different files. A matching type declaration joins that unit.
    by_receiver: dict[str, list[str]] = defaultdict(list)
    for item in symbol_declarations:
        receiver = item.get("receiver_type")
        if isinstance(receiver, str) and receiver:
            by_receiver[receiver].append(item["id"])
    for ids in by_receiver.values():
        for other in ids[1:]:
            union(ids[0], other)
    type_by_name = {item.get("name"): item["id"] for item in declarations if item.get("kind") == "type"}
    for receiver, ids in by_receiver.items():
        type_id = type_by_name.get(receiver)
        if type_id:
            for function_id in ids:
                parent.setdefault(type_id, type_id)
                union(function_id, type_id)
    for edge in relationships:
        caller = edge.get("caller", {}) if isinstance(edge.get("caller"), dict) else {}
        callee = edge.get("callee", {}) if isinstance(edge.get("callee"), dict) else {}
        if caller.get("package") == package_id and callee.get("package") == package_id:
            caller_name, callee_name = caller.get("function"), callee.get("function")
            left = by_symbol.get(str(caller_name)) or (by_simple.get(str(caller_name), [None])[0] if by_simple.get(str(caller_name)) else None)
            right = by_symbol.get(str(callee_name)) or (by_simple.get(str(callee_name), [None])[0] if by_simple.get(str(callee_name)) else None)
            if left in parent and right in parent:
                union(left, right)
    groups: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for item in symbol_declarations:
        groups[find(item["id"])].append(item)
    assigned = {decl["id"] for values in groups.values() for decl in values}
    # Types and package-level declarations without receiver methods remain
    # explicit units rather than disappearing from source coverage.
    for item in declarations:
        if item["id"] not in assigned:
            groups[item["id"]].append(item)
    result = []
    for group_id, members in sorted(groups.items()):
        ordered = sorted(members, key=lambda item: (item.get("file", ""), item.get("start_line", 0), item.get("id", "")))
        result.append({"id": stable_id("partition", [package_id, [item["id"] for item in ordered]]), "package": package_id, "declarations": ordered, "source_files": sorted({item["file"] for item in ordered})})
    return result
