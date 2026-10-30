#!/usr/bin/env python3
"""Build deterministic, bounded investigation packets from Reffector JSON."""
from __future__ import annotations

import argparse
import collections
import datetime
import hashlib
import json
import pathlib
import re

CATEGORIES = (
    "multi-frontier-reachability",
    "candidate-unresolved-relationships",
    "async-edges",
    "branching-convergence",
    "cross-component-shared-operation-site",
)
ID_PATTERNS = {
    "sites": re.compile(r"site:[0-9a-f]{64}\Z"),
    "nodes": re.compile(r"node:[0-9a-f]{64}\Z"),
    "edges": re.compile(r"link:[0-9a-f]{64}\Z"),
}
PROVENANCE_RESOLUTIONS = {"established", "candidate_vta", "candidate_function_value",
                          "unresolved_dynamic", "unresolved_cmd_receiver",
                          "unresolved_mutable_receiver", "unresolved_effect_site_unavailable"}
CONTROL_FLOW_ID = re.compile(r"control(?:-[a-z_]+)?:[0-9a-f]{64}\Z")
CONTROL_FLOW_STATUSES = {
    "complete", "partial", "unavailable_api_identity", "unavailable_site_nodes",
    "unavailable_function_body", "unavailable_mapping", "ambiguous_instruction",
    "unreachable", "unknown_postdominators", "unavailable", "truncated_functions",
    "truncated_blocks", "truncated_block_visits", "truncated_branches",
    "truncated_depth", "truncated_relationships", "omitted_json_bytes",
}
CONTROL_FLOW_RESOLUTIONS = {"operation", "established", "candidate_vta", "candidate_function_value"}
CONTROL_FLOW_STRUCTURAL_LIMITATION = (
    "SSA control-flow facts are structural evidence only; they do not establish execution, feasible paths, "
    "safety, authorization, or cleanup success."
)
CONTROL_FLOW_SITE_LIMITATION = "Rows cover only EffectReport sites retained by its site limit."
PROVENANCE_ID_PATTERNS = {
    "value": re.compile(r"value:[0-9a-f]{64}\Z"),
    "origin": re.compile(r"origin:[0-9a-f]{64}\Z"),
    "call": re.compile(r"call:[0-9a-f]{64}\Z"),
}


def load_json(path):
    with open(path, encoding="utf-8") as stream:
        return json.load(stream)


def canonical_json(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def canonical_component_bytes(packet):
    """Measure canonical object members separately from braces/commas framing."""
    members = {key: len(canonical_json(key)) + 1 + len(canonical_json(value))
               for key, value in packet.items()}
    return {"members": members, "framing": len(canonical_json(packet)) - sum(members.values()),
            "total": len(canonical_json(packet))}


def stable_candidate_id(site_id, edge_ids):
    material = site_id + "\n" + "\n".join(sorted(set(edge_ids)))
    return "candidate:" + sha256(material.encode("utf-8"))


def require_object(value, name):
    if not isinstance(value, dict):
        raise ValueError(f"{name} must be an object")
    return value


def require_array(value, name):
    if not isinstance(value, list):
        raise ValueError(f"{name} must be an array")
    return value


def validate_effect_report(report):
    require_object(report, "EffectReport")
    if report.get("schema_version") != 1:
        raise ValueError("EffectReport schema_version must be 1")
    indexes = {}
    for name in ("sites", "nodes", "edges"):
        index = {}
        for row in require_array(report.get(name), name):
            require_object(row, f"{name} row")
            ident = row.get("id")
            if not isinstance(ident, str) or not ID_PATTERNS[name].fullmatch(ident):
                raise ValueError(f"malformed {name} ID: {ident!r}")
            if ident in index:
                raise ValueError(f"duplicate {name} ID: {ident}")
            index[ident] = row
        indexes[name] = index

    sites, nodes, edges = (indexes[key] for key in ("sites", "nodes", "edges"))
    for site in sites.values():
        for key in ("package", "function", "kind"):
            if not isinstance(site.get(key), str):
                raise ValueError(f"site {site['id']} missing string {key}")
        require_object(site.get("location"), f"site {site['id']} location")
    for node in nodes.values():
        if not isinstance(node.get("package"), str) or not isinstance(node.get("function"), str):
            raise ValueError(f"node {node['id']} lacks package/function")
    for edge in edges.values():
        if edge.get("caller") not in nodes or edge.get("callee") not in nodes:
            raise ValueError(f"dangling endpoint on edge {edge['id']}")
        if not isinstance(edge.get("candidate"), bool) or not isinstance(edge.get("synthetic"), bool):
            raise ValueError(f"edge {edge['id']} has invalid candidate/synthetic flag")
        source = edge.get("source")
        if source is not None:
            require_object(source, f"edge {edge['id']} source")
            if not isinstance(source.get("id"), str) or not re.fullmatch(r"edge:[0-9a-f]{64}", source["id"]):
                raise ValueError(f"edge {edge['id']} has malformed source ID")
            if source.get("kind") not in ("call", "go", "defer"):
                raise ValueError(f"edge {edge['id']} has invalid source kind")
            if source.get("resolution") not in ("established", "candidate_vta", "candidate_function_value"):
                raise ValueError(f"edge {edge['id']} has invalid resolution")

    reverse_seen = set()
    for rev in require_array(report.get("reverse", []), "reverse"):
        require_object(rev, "reverse row")
        site_id = rev.get("site")
        if site_id not in sites or site_id in reverse_seen:
            raise ValueError(f"invalid/duplicate reverse site: {site_id}")
        reverse_seen.add(site_id)
        for ref in require_array(rev.get("nodes"), "reverse.nodes"):
            require_object(ref, "reverse node ref")
            if ref.get("id") not in nodes or not isinstance(ref.get("depth"), int) or ref["depth"] < 0:
                raise ValueError("invalid/dangling reverse node reference")
        for edge_id in require_array(rev.get("edges"), "reverse.edges"):
            if edge_id not in edges:
                raise ValueError(f"dangling reverse edge: {edge_id}")
        for node_id in require_array(rev.get("origins", []), "reverse.origins"):
            if node_id not in nodes:
                raise ValueError(f"dangling reverse origin: {node_id}")
        for flag in ("cycle", "revisit"):
            if not isinstance(rev.get(flag), bool):
                raise ValueError(f"reverse {flag} must be boolean")
        truncation = require_object(rev.get("truncated"), "reverse.truncated")
        if any(not isinstance(truncation.get(flag), bool) for flag in ("sites", "edges", "depth")):
            raise ValueError("reverse truncation flags must be boolean")

    require_object(report.get("coverage"), "coverage")
    truncation = require_object(report.get("truncated"), "truncated")
    if any(not isinstance(truncation.get(flag), bool) for flag in ("sites", "edges", "depth")):
        raise ValueError("report truncation flags must be boolean")
    return indexes


def validate_argument_provenance(value, report_sites):
    require_object(value, "argument provenance")
    if value.get("schema_version") != 1 or value.get("schema") != "reffector.argument-provenance.v1":
        raise ValueError("unsupported argument provenance schema")
    nodes, edges, sites, calls = {}, {}, {}, {}
    for field, index, prefix in (("nodes", nodes, "value"), ("edges", edges, "origin"),
                                 ("calls", calls, "call"), ("sites", sites, None)):
        for row in require_array(value.get(field), f"provenance.{field}"):
            require_object(row, f"provenance {field} row")
            ident = row.get("id")
            valid = (isinstance(ident, str) and
                     (PROVENANCE_ID_PATTERNS[prefix].fullmatch(ident) if prefix else
                      re.fullmatch(r"(?:site|operation):[0-9a-f]{64}", ident)))
            if not valid:
                raise ValueError(f"malformed provenance {field} ID: {ident!r}")
            if ident in index:
                raise ValueError(f"duplicate provenance {field} ID: {ident}")
            index[ident] = row
    if not isinstance(value.get("truncation"), list):
        raise ValueError("provenance.truncation must be an array")
    for edge in edges.values():
        if edge.get("from") not in nodes or edge.get("to") not in nodes:
            raise ValueError(f"dangling provenance edge endpoint: {edge['id']}")
        if edge.get("resolution") not in PROVENANCE_RESOLUTIONS:
            raise ValueError(f"invalid provenance edge resolution: {edge['id']}")
        if edge.get("call_ref") and edge["call_ref"] not in calls:
            raise ValueError(f"dangling provenance call reference: {edge['id']}")
    for call in calls.values():
        if call.get("resolution") not in PROVENANCE_RESOLUTIONS:
            raise ValueError(f"invalid provenance call resolution: {call['id']}")
        for field in ("arguments", "results"):
            for ref in require_array(call.get(field), f"call.{field}"):
                if ref and ref not in nodes:
                    raise ValueError(f"dangling provenance {field} reference: {call['id']}")
    for site in sites.values():
        if not isinstance(site.get("api"), str) or site.get("resolution") not in PROVENANCE_RESOLUTIONS:
            raise ValueError(f"invalid provenance site: {site['id']}")
        site_ref = site.get("site_ref")
        if site_ref is not None:
            if not isinstance(site_ref, str) or not ID_PATTERNS["sites"].fullmatch(site_ref) or site_ref not in report_sites:
                raise ValueError(f"invalid provenance report site join: {site_ref}")
            report_site = report_sites[site_ref]
            apis = set()
            if isinstance(report_site.get("api"), str):
                apis.add(report_site["api"])
            apis.update(api for api in report_site.get("apis", []) if isinstance(api, str))
            for operation in report_site.get("operations", []):
                if isinstance(operation, dict) and isinstance(operation.get("api"), str):
                    apis.add(operation["api"])
            if site["api"] not in apis:
                raise ValueError(f"provenance API {site['api']!r} does not match joined report site {site_ref}")
        if site.get("call_ref") and site["call_ref"] not in calls:
            raise ValueError(f"dangling provenance site call reference: {site['id']}")
        for role in require_array(site.get("roles"), "provenance site roles"):
            require_object(role, "provenance role")
            if not isinstance(role.get("role"), str) or not isinstance(role.get("index"), int):
                raise ValueError(f"invalid provenance role: {site['id']}")
            if role.get("value_ref") and role["value_ref"] not in nodes:
                raise ValueError(f"dangling provenance role value: {site['id']}")
    return {"nodes": nodes, "edges": edges, "sites": sites, "calls": calls}


def control_flow_site_apis(site):
    apis = set()
    if "api" in site:
        if not isinstance(site["api"], str):
            raise ValueError("EffectReport site API must be a string")
        if site["api"]:
            apis.add(site["api"])
    for api in require_array(site.get("apis", []), "EffectReport site APIs"):
        if not isinstance(api, str):
            raise ValueError("EffectReport site APIs must contain strings")
        if api:
            apis.add(api)
    for operation in require_array(site.get("operations", []), "EffectReport site operations"):
        require_object(operation, "EffectReport site operation")
        if not isinstance(operation.get("api"), str):
            raise ValueError("EffectReport site operation API must be a string")
        if operation["api"]:
            apis.add(operation["api"])
    return apis


def validate_control_flow(value, report_sites):
    def require_int(item, name, minimum=None):
        if not isinstance(item, int) or isinstance(item, bool) or (minimum is not None and item < minimum):
            raise ValueError(f"invalid control-flow integer: {name}")
        return item

    def require_resolution(item, name):
        if not isinstance(item, str) or item not in CONTROL_FLOW_RESOLUTIONS:
            raise ValueError(f"invalid control-flow resolution: {name}")

    def validate_location(item, name):
        if item is None:
            return
        require_object(item, name)
        if not isinstance(item.get("file"), str) or not item["file"]:
            raise ValueError(f"invalid control-flow location file: {name}")
        require_int(item.get("line"), f"{name}.line", 1)
        require_int(item.get("column"), f"{name}.column", 1)

    def validate_resolution_list(items, name):
        require_array(items, name)
        for item in items:
            require_resolution(item, name)

    require_object(value, "control-flow projection")
    if (not isinstance(value.get("schema_version"), int) or isinstance(value.get("schema_version"), bool) or
            value["schema_version"] != 1 or value.get("schema") != "reffector.control-flow.v1"):
        raise ValueError("unsupported control-flow schema")
    limits = require_object(value.get("limits"), "control-flow limits")
    limit_names = ("max_functions", "max_blocks_per_function", "max_block_visits", "max_branches",
                   "max_relationships", "max_depth", "max_json_bytes")
    for name in limit_names:
        if not isinstance(limits.get(name), int) or isinstance(limits.get(name), bool) or limits[name] <= 0:
            raise ValueError(f"control-flow limit {name} must be positive")
    truncated = require_object(value.get("truncated"), "control-flow truncated")
    if any(not isinstance(flag, bool) for flag in truncated.values()):
        raise ValueError("control-flow truncated values must be boolean")
    limitations = require_array(value.get("limitations"), "control-flow limitations")
    if not all(isinstance(item, str) for item in limitations):
        raise ValueError("control-flow limitations must contain strings")
    if CONTROL_FLOW_STRUCTURAL_LIMITATION not in limitations:
        raise ValueError("control-flow projection lacks its structural-only limitation")
    if truncated.get("effect_report_sites") and CONTROL_FLOW_SITE_LIMITATION not in limitations:
        raise ValueError("control-flow site truncation lacks its EffectReport limitation")
    rows = {}
    for row in require_array(value.get("rows"), "control-flow rows"):
        require_object(row, "control-flow row")
        ident, site_ref, api, status = (row.get(key) for key in ("id", "site", "api", "status"))
        if not isinstance(ident, str) or not CONTROL_FLOW_ID.fullmatch(ident):
            raise ValueError(f"malformed control-flow ID: {ident!r}")
        if ident in rows:
            raise ValueError(f"duplicate control-flow ID: {ident}")
        if not isinstance(site_ref, str) or not ID_PATTERNS["sites"].fullmatch(site_ref) or site_ref not in report_sites:
            raise ValueError(f"invalid/dangling control-flow site reference: {site_ref!r}")
        if not isinstance(api, str):
            raise ValueError(f"control-flow row {ident} API must be a string")
        site_apis = control_flow_site_apis(report_sites[site_ref])
        if api and api not in site_apis:
            raise ValueError(f"control-flow API {api!r} does not match joined report site {site_ref}")
        if not api and site_apis:
            raise ValueError(f"control-flow row {ident} omits an API available on joined report site {site_ref}")
        if not isinstance(status, str) or status not in CONTROL_FLOW_STATUSES:
            raise ValueError(f"invalid control-flow status: {status!r}")
        if not api and status != "unavailable_api_identity":
            raise ValueError(f"empty control-flow API requires unavailable_api_identity status: {ident}")
        if not isinstance(row.get("complete"), bool):
            raise ValueError(f"control-flow row {ident} complete must be boolean")
        require_int(row.get("block"), f"{ident}.block")
        require_int(row.get("instruction"), f"{ident}.instruction")
        function = require_object(row.get("function"), f"control-flow row {ident} function")
        if not isinstance(function.get("package"), str) or not isinstance(function.get("function"), str):
            raise ValueError(f"control-flow row {ident} function lacks package/function strings")
        if "function_variant" in row and not isinstance(row["function_variant"], str):
            raise ValueError(f"control-flow row {ident} variant must be a string")
        if "resolution" in row:
            require_resolution(row["resolution"], f"{ident}.resolution")
        if "resolutions" in row:
            validate_resolution_list(row["resolutions"], f"{ident}.resolutions")
            if row.get("resolution") and row["resolution"] not in row["resolutions"]:
                raise ValueError(f"control-flow row {ident} resolution absent from alternatives")
        for field in ("bypass_returns", "dominators"):
            for item in require_array(row.get(field, []), f"{ident}.{field}"):
                require_int(item, f"{ident}.{field}", 0)
        for field in ("markers",):
            if any(not isinstance(item, str) for item in require_array(row.get(field, []), f"{ident}.{field}")):
                raise ValueError(f"control-flow row {ident} {field} must contain strings")
        for branch in require_array(row.get("branches", []), f"{ident}.branches"):
            require_object(branch, f"{ident}.branch")
            for field in ("block", "true", "false"):
                require_int(branch.get(field), f"{ident}.branch.{field}", 0)
            if not isinstance(branch.get("condition_kind"), str):
                raise ValueError(f"control-flow row {ident} branch lacks condition kind")
            for field in ("condition_type", "condition_identity"):
                if field in branch and not isinstance(branch[field], str):
                    raise ValueError(f"control-flow row {ident} branch {field} must be a string")
            validate_location(branch.get("location"), f"{ident}.branch.location")
            for control in require_array(branch.get("controls", []), f"{ident}.branch.controls"):
                require_object(control, f"{ident}.branch.control")
                require_int(control.get("block"), f"{ident}.branch.control.block", 0)
                require_int(control.get("depth"), f"{ident}.branch.control.depth", 1)
        for field in ("bypass_return_evidence", "marker_evidence"):
            for evidence in require_array(row.get(field, []), f"{ident}.{field}"):
                require_object(evidence, f"{ident}.{field} row")
                require_int(evidence.get("block"), f"{ident}.{field}.block", 0)
                require_int(evidence.get("instruction"), f"{ident}.{field}.instruction", -1 if field == "marker_evidence" else 0)
                validate_location(evidence.get("location"), f"{ident}.{field}.location")
                if field == "bypass_return_evidence":
                    results = require_array(evidence.get("result_types", []), f"{ident}.result_types")
                    if any(not isinstance(item, str) for item in results):
                        raise ValueError(f"control-flow row {ident} result types must contain strings")
                elif not isinstance(evidence.get("kind"), str):
                    raise ValueError(f"control-flow row {ident} marker lacks kind")
        instruction_candidates = require_array(row.get("instruction_candidates", []), f"{ident}.instruction_candidates")
        for candidate in instruction_candidates:
            require_object(candidate, f"{ident}.instruction_candidate")
            require_int(candidate.get("block"), f"{ident}.instruction_candidate.block", 0)
            require_int(candidate.get("instruction"), f"{ident}.instruction_candidate.instruction", 0)
            if not isinstance(candidate.get("kind"), str):
                raise ValueError(f"control-flow row {ident} instruction candidate lacks kind")
            if "resolutions" in candidate:
                validate_resolution_list(candidate["resolutions"], f"{ident}.instruction_candidate.resolutions")
        if status == "ambiguous_instruction" and (row.get("block") != -1 or row.get("instruction") != -1 or
                                                   len(instruction_candidates) < 2):
            raise ValueError(f"ambiguous control-flow row {ident} requires multiple instruction candidates and -1 location")
        rows[ident] = row
    return {"rows": rows, "projection": value}


def control_flow_packet_summary(site, projection):
    if projection is None:
        return None
    apis = control_flow_site_apis(site)
    matched = [row for row in projection["rows"].values() if row["site"] == site["id"] and row["api"] in apis]
    return {"limitations": projection["projection"]["limitations"],
            "rows": [{"id": row["id"], "status": row["status"], "api": row["api"],
                      "resolution": row.get("resolution"), "resolutions": row.get("resolutions"),
                      "function": row.get("function"), "function_variant": row.get("function_variant"),
                      "block": row.get("block", -1), "instruction": row.get("instruction", -1),
                      "kind": row.get("kind"), "branches": row.get("branches"),
                      "bypass_returns": row.get("bypass_returns"),
                      "bypass_return_evidence": row.get("bypass_return_evidence"),
                      "marker_evidence": row.get("marker_evidence"),
                      "instruction_candidates": row.get("instruction_candidates"),
                      "complete": row["complete"]} for row in sorted(matched, key=lambda item: item["id"])],
            "truncated": projection["projection"]["truncated"]}


def provenance_site_matches(row, operation, site_ids):
    if row.get("site_ref"):
        return row["site_ref"] in site_ids
    apis = {operation.get("api")} if operation.get("api") else set(operation.get("apis", []))
    if row.get("api") not in apis:
        return False
    site_location, operation_location = row.get("location"), operation.get("location")
    if not isinstance(site_location, dict) or not isinstance(operation_location, dict):
        return False
    if site_location.get("file") != operation_location.get("file"):
        return False
    return all(site_location.get(field) == operation_location.get(field)
               for field in ("line", "column"))


def provenance_packet_summary(operation, provenance, site_ids):
    if not provenance:
        return None
    matched = [row for row in provenance["sites"].values()
               if provenance_site_matches(row, operation, site_ids)]
    node_index, edge_index, call_index = provenance["nodes"], provenance["edges"], provenance["calls"]
    summaries = []
    for row in sorted(matched, key=lambda item: item["id"]):
        seen, pending = set(), [role.get("value_ref") for role in row["roles"] if role.get("value_ref")]
        while pending:
            current = pending.pop()
            if current in seen:
                continue
            seen.add(current)
            pending.extend(edge["from"] for edge in edge_index.values() if edge["to"] == current)
        relevant_edges = [edge for edge in edge_index.values() if edge["from"] in seen and edge["to"] in seen]
        summaries.append({
            "site_id": row["id"], "site_ref": row.get("site_ref"), "api": row["api"],
            "resolution": row["resolution"],
            "roles": [{"role": role["role"], "index": role["index"], "value_ref": role.get("value_ref"),
                       "boundary": role.get("boundary")} for role in row["roles"]],
            "origins": [{"id": node_id, "kind": node_index[node_id]["kind"],
                         "label": node_index[node_id]["label"], "boundary": node_index[node_id].get("boundary")}
                        for node_id in sorted(seen)],
            "edges": [{"id": edge["id"], "from": edge["from"], "to": edge["to"],
                       "kind": edge["kind"], "resolution": edge["resolution"],
                       "call_ref": edge.get("call_ref")} for edge in sorted(relevant_edges, key=lambda item: item["id"])],
            "calls": [{"id": call_id, "resolution": call_index[call_id]["resolution"]}
                      for call_id in sorted({r.get("call_ref") for r in [row, *relevant_edges] if r.get("call_ref")})],
            "limitations": row.get("site_limitation"),
        })
    return {"sites": summaries, "truncation": provenance.get("truncation", [])}


def validate_audit(audit):
    site_rows = {}
    functions = {}
    total_sites = total_functions = 0
    for row in audit:
        require_object(row, "audit record")
        if row.get("schema_version") != 1:
            raise ValueError("audit schema_version must be 1")
        record_type = row.get("record_type")
        if record_type == "site":
            total_sites += 1
            site_id = row.get("id")
            if not isinstance(site_id, str) or not ID_PATTERNS["sites"].fullmatch(site_id):
                raise ValueError(f"malformed audit site ID: {site_id!r}")
            if site_id in site_rows:
                raise ValueError(f"duplicate audit site ID: {site_id}")
            for key in ("package", "function", "kind"):
                if not isinstance(row.get(key), str):
                    raise ValueError(f"audit site {site_id} missing string {key}")
            require_object(row.get("location"), f"audit site {site_id} location")
            if not isinstance(row.get("uncertain"), bool):
                raise ValueError(f"audit site {site_id} uncertain must be boolean")
            for key in ("effects", "possible_effects", "apis", "dispatch_kinds", "uncertainty_reasons", "vta_candidates", "operations", "ssa_nodes"):
                if key in row and not isinstance(row[key], list):
                    raise ValueError(f"audit site {site_id} {key} must be an array")
            site_rows[site_id] = row
        elif record_type == "function":
            total_functions += 1
            package, function = row.get("package"), row.get("function")
            if not isinstance(package, str) or not package or not isinstance(function, str) or not function:
                raise ValueError("audit function requires package and function strings")
            if not isinstance(row.get("effects"), list) or not isinstance(row.get("uncertain"), bool):
                raise ValueError("audit function has invalid effects/uncertain fields")
            key = (package, function)
            if key in functions:
                raise ValueError(f"duplicate audit function: {key}")
            functions[key] = row
        else:
            raise ValueError(f"unknown audit record_type: {record_type!r}")
    return site_rows, functions, total_sites, total_functions


def validate_program_model(program_model):
    if program_model is None:
        return {}
    require_object(program_model, "program model")
    if program_model.get("schema_version") != 1:
        raise ValueError("program-model schema_version must be 1")
    declarations = {}
    for declaration in require_array(program_model.get("declarations"), "program-model declarations"):
        require_object(declaration, "program declaration")
        for field in ("package", "name", "file"):
            if not isinstance(declaration.get(field), str):
                raise ValueError(f"program declaration missing {field}")
        for field in ("start_line", "end_line"):
            if not isinstance(declaration.get(field), int) or declaration[field] < 1:
                raise ValueError(f"program declaration has invalid {field}")
        if declaration["end_line"] < declaration["start_line"]:
            raise ValueError("program declaration end precedes start")
        declaration_id = declaration.get("id")
        if declaration_id is not None and not isinstance(declaration_id, str):
            raise ValueError(f"invalid program declaration ID: {declaration_id!r}")
        key = (declaration["package"], declaration["name"])
        declarations.setdefault(key, []).append(declaration)
    for values in declarations.values():
        values.sort(key=lambda item: (item["file"], item["start_line"], item["end_line"]))
    return declarations


def safe_source(file_name, source_root, manifest):
    if not source_root or manifest is None:
        return None, "source root/manifest not supplied"
    root = pathlib.Path(source_root).resolve()
    supplied = pathlib.Path(file_name)
    if supplied.is_absolute():
        try:
            relative = supplied.resolve().relative_to(root).as_posix()
        except (OSError, ValueError):
            return None, "source path outside supplied root"
    else:
        relative = supplied.as_posix()
    relative_path = pathlib.PurePosixPath(relative)
    if relative_path.is_absolute() or ".." in relative_path.parts:
        return None, "source path outside manifest"
    entry = manifest["files"].get(relative)
    if entry is None:
        return None, "outside supplied source manifest"
    path = (root / relative).resolve()
    if not path.is_relative_to(root):
        return None, "resolved source path escapes supplied root"
    try:
        content = path.read_bytes()
    except OSError:
        return None, "manifest file absent under supplied root"
    digest = sha256(content)
    if digest != entry.get("sha256"):
        return None, "source hash mismatch"
    return (relative, content, digest), None


def normalize_model_paths(value, source_root, manifest):
    """Normalize serialized location fields only; semantic strings remain untouched."""
    if isinstance(value, list):
        return [normalize_model_paths(item, source_root, manifest) for item in value]
    if not isinstance(value, dict):
        return value
    normalized = {}
    for key, item in value.items():
        if key in ("file", "path") and isinstance(item, str):
            resolved, error = safe_source(item, source_root, manifest)
            if error:
                normalized[key] = None
                normalized[key + "_status"] = "unavailable"
                normalized[key + "_reason"] = error
            else:
                normalized[key] = resolved[0]
                normalized[key + "_status"] = "repository_relative"
            continue
        normalized[key] = normalize_model_paths(item, source_root, manifest)
    return normalized


def graph_summary(edge, nodes):
    source = edge.get("source") or {}
    caller, callee = nodes[edge["caller"]], nodes[edge["callee"]]
    return {
        "id": edge["id"],
        "caller": {"id": caller["id"], "package": caller["package"], "function": caller["function"]},
        "callee": {"id": callee["id"], "package": callee["package"], "function": callee["function"]},
        "kind": source.get("kind", "synthetic"),
        "resolution": source.get("resolution", "synthetic"),
        "candidate": edge["candidate"],
        "synthetic": edge["synthetic"],
    }


def stable_reference_id(prefix, value):
    return f"{prefix}:" + sha256(canonical_json(value))


def _edge_class(edge):
    source = edge.get("source") or {}
    return {
        "kind": source.get("kind", "synthetic"),
        "resolution": source.get("resolution", "synthetic" if edge.get("synthetic") else "unresolved"),
        "candidate": bool(edge.get("candidate")),
        "synthetic": bool(edge.get("synthetic")),
    }


def compact_graph_evidence(candidate, nodes, edges):
    """Build a lossless reference-level compaction of one candidate's call graph."""
    node_ids = set(candidate["node_ids"])
    edge_ids = sorted(set(candidate["edge_ids"]))
    incoming = {node_id: [] for node_id in node_ids}
    outgoing = {node_id: [] for node_id in node_ids}
    classes = {}
    for edge_id in edge_ids:
        edge = edges[edge_id]
        incoming.setdefault(edge["callee"], []).append(edge_id)
        outgoing.setdefault(edge["caller"], []).append(edge_id)
        classes[edge_id] = _edge_class(edge)
    for refs in (*incoming.values(), *outgoing.values()):
        refs.sort()

    frontiers = set(candidate["frontiers"])
    reverse = candidate["reverse"]
    targets = set(reverse.get("origins", []))
    targets.update(node_id for node_id in node_ids
                   if nodes[node_id]["package"] == candidate["operation"]["package"]
                   and nodes[node_id]["function"] == candidate["operation"]["function"])
    branching = {node_id for node_id in node_ids if len(outgoing.get(node_id, [])) > 1}
    converging = {node_id for node_id in node_ids if len(incoming.get(node_id, [])) > 1}

    anchors = frontiers | targets | branching | converging
    for edge_id in edge_ids:
        edge = edges[edge_id]
        cls = classes[edge_id]
        crosses_package = nodes[edge["caller"]]["package"] != nodes[edge["callee"]]["package"]
        if (cls["candidate"] or cls["resolution"] != "established" or cls["synthetic"]
                or cls["kind"] in ("go", "defer") or crosses_package):
            anchors.add(edge["caller"])
            anchors.add(edge["callee"])

    def collapsible(edge_id):
        cls = classes[edge_id]
        edge = edges[edge_id]
        return (cls["kind"] == "call" and cls["resolution"] == "established"
                and not cls["candidate"] and not cls["synthetic"]
                and nodes[edge["caller"]]["package"] == nodes[edge["callee"]]["package"])

    segments, direct_ids, consumed = [], set(), set()
    start_order = sorted(anchors) + sorted(node_ids - anchors)
    for start in start_order:
        for first in outgoing.get(start, []):
            if first in consumed or not collapsible(first):
                continue
            edge_path, node_path = [first], [start, edges[first]["callee"]]
            consumed.add(first)
            current = node_path[-1]
            while (current not in anchors and len(incoming.get(current, [])) == 1
                   and len(outgoing.get(current, [])) == 1):
                next_edge = outgoing[current][0]
                if next_edge in consumed or not collapsible(next_edge):
                    break
                consumed.add(next_edge)
                edge_path.append(next_edge)
                current = edges[next_edge]["callee"]
                node_path.append(current)
            if len(edge_path) > 1:
                class_counts = collections.Counter(
                    (classes[edge_id]["kind"], classes[edge_id]["resolution"],
                     classes[edge_id]["candidate"], classes[edge_id]["synthetic"])
                    for edge_id in edge_path
                )
                segment = {
                    "id": stable_reference_id("segment", {"candidate": candidate["id"], "edges": edge_path}),
                    "from": node_path[0], "to": node_path[-1],
                    "edge_count": len(edge_path),
                    "intermediate_node_count": max(0, len(node_path) - 2),
                    "edge_refs": edge_path,
                    "relationship_classes": [
                        {"kind": kind, "resolution": resolution, "candidate": is_candidate,
                         "synthetic": synthetic, "count": count}
                        for (kind, resolution, is_candidate, synthetic), count in sorted(class_counts.items())
                    ],
                }
                segments.append(segment)
            else:
                direct_ids.add(first)
    direct_ids.update(set(edge_ids) - consumed)
    segments.sort(key=lambda item: item["id"])

    adjacency = {node_id: [edges[eid]["callee"] for eid in outgoing.get(node_id, [])] for node_id in node_ids}
    reverse_adjacency = {node_id: [edges[eid]["caller"] for eid in incoming.get(node_id, [])] for node_id in node_ids}

    def reachable(starts, graph):
        seen, pending = set(starts) & node_ids, sorted(set(starts) & node_ids, reverse=True)
        while pending:
            current = pending.pop()
            for neighbor in graph.get(current, []):
                if neighbor not in seen:
                    seen.add(neighbor)
                    pending.append(neighbor)
        return seen

    from_frontiers = reachable(frontiers, adjacency)
    to_targets = reachable(targets, reverse_adjacency)

    def item_priority(item):
        on_structural_route = item["from"] in from_frontiers and item["to"] in to_targets
        established_call = (item["type"] == "segment" or
                            (item.get("kind") == "call" and item.get("resolution") == "established"
                             and not item.get("candidate") and not item.get("synthetic")
                             and not item.get("package_boundary")))
        if on_structural_route and established_call:
            return 0
        if item.get("candidate") or item.get("resolution") != "established":
            return 1
        if item.get("kind") in ("go", "defer") or item.get("package_boundary"):
            return 2
        if item["from"] in branching | converging or item["to"] in branching | converging:
            return 3
        if item["type"] == "segment":
            return 4
        return 5

    items = []
    for segment in segments:
        items.append({"type": "segment", "id": segment["id"], "from": segment["from"], "to": segment["to"],
                      "edge_refs": segment["edge_refs"], "edge_count": segment["edge_count"],
                      "intermediate_node_count": segment["intermediate_node_count"],
                      "relationship_classes": segment["relationship_classes"],
                      "resolution": "established", "candidate": False})
    for edge_id in sorted(direct_ids):
        edge, cls = edges[edge_id], classes[edge_id]
        items.append({"type": "edge", "id": edge_id, "from": edge["caller"], "to": edge["callee"],
                      "edge_refs": [edge_id], "edge_count": 1, "intermediate_node_count": 0,
                      **cls, "package_boundary": nodes[edge["caller"]]["package"] != nodes[edge["callee"]]["package"]})
    items.sort(key=lambda item: (item_priority(item), item["id"]))

    kind_counts = collections.Counter(cls["kind"] for cls in classes.values())
    resolution_counts = collections.Counter(cls["resolution"] for cls in classes.values())
    summary = {
        "node_count": len(node_ids), "edge_count": len(edge_ids),
        "frontier_count": len(frontiers), "frontier_node_refs": sorted(frontiers)[:16],
        "frontier_refs_omitted": max(0, len(frontiers) - 16),
        "target_count": len(targets), "target_node_refs": sorted(targets)[:16],
        "target_refs_omitted": max(0, len(targets) - 16),
        "branching_node_count": len(branching), "converging_node_count": len(converging),
        "branching_node_refs": sorted(branching)[:16], "converging_node_refs": sorted(converging)[:16],
        "candidate_edge_count": sum(1 for cls in classes.values() if cls["candidate"]),
        "unresolved_edge_count": sum(1 for cls in classes.values() if cls["resolution"] != "established"),
        "kind_counts": dict(sorted(kind_counts.items())),
        "resolution_counts": dict(sorted(resolution_counts.items())),
        "compact_segment_count": len(segments),
        "direct_edge_count": len(direct_ids),
        "frontier_to_target_structural_items": sum(1 for item in items
            if item["from"] in from_frontiers and item["to"] in to_targets),
        "cycle": bool(reverse.get("cycle")), "revisit": bool(reverse.get("revisit")),
        "truncated": reverse.get("truncated", {}),
        "interpretation": "Structural graph connectivity; not proof of a runtime-feasible path.",
    }
    represented = set(direct_ids)
    for segment in segments:
        represented.update(segment["edge_refs"])
    if represented != set(edge_ids):
        raise ValueError(f"compact graph lost relationships for candidate {candidate['id']}")
    return {"summary": summary, "segments": segments,
            "direct_edges": [item for item in items if item["type"] == "edge"],
            "items": items, "anchors": anchors, "branching": branching, "converging": converging,
            "targets": targets, "all_edge_refs": edge_ids}


def compact_audit_evidence(audit_evidence, candidate_id, max_values=8, max_function_refs=12):
    site = audit_evidence.get("site")
    functions = audit_evidence.get("functions", [])

    def values(row, field):
        source = row.get(field, []) if row else []
        strings = sorted({item for item in source if isinstance(item, str)})
        other = [item for item in source if not isinstance(item, str)]
        return {"values": strings[:max_values], "count": len(source),
                "omitted_count": max(0, len(strings) - max_values),
                "non_string_count": len(other),
                "non_string_refs": [stable_reference_id("audit-value", {field: item})
                                    for item in other[:max_values]],
                "omitted_non_string_count": max(0, len(other) - max_values)}

    site_summary = None
    if site:
        candidates = site.get("vta_candidates", [])
        site_summary = {
            "site_ref": site["id"], "uncertain": site.get("uncertain"),
            "effects": values(site, "effects"), "possible_effects": values(site, "possible_effects"),
            "uncertainty_reasons": values(site, "uncertainty_reasons"),
            "dispatch_kinds": values(site, "dispatch_kinds"),
            "vta_candidate_count": len(candidates),
            "vta_candidate_refs": [stable_reference_id("audit-target", item) for item in candidates[:max_values]],
            "omitted_vta_candidate_count": max(0, len(candidates) - max_values),
        }
    function_refs = [{"id": stable_reference_id("audit-function", [row["package"], row["function"]]),
                      "package": row["package"], "function": row["function"]}
                     for row in sorted(functions, key=lambda item: (item["package"], item["function"]))]
    function_effects = [effect for row in functions for effect in row.get("effects", [])]
    effect_counts = collections.Counter(effect for effect in function_effects if isinstance(effect, str))
    other_effects = [effect for effect in function_effects if not isinstance(effect, str)]
    return {
        "site": site_summary,
        "functions": {"count": len(functions),
                       "uncertain_count": sum(1 for row in functions if row.get("uncertain")),
                       "effect_counts": dict(sorted(effect_counts.items())),
                       "non_string_effect_count": len(other_effects),
                       "non_string_effect_refs": [stable_reference_id("audit-value", effect)
                                                   for effect in other_effects[:max_function_refs]],
                       "omitted_non_string_effect_count": max(0, len(other_effects) - max_function_refs),
                       "function_refs": function_refs[:max_function_refs],
                       "omitted_function_ref_count": max(0, len(function_refs) - max_function_refs)},
        "evidence_store_ref": {"candidate_id": candidate_id, "field": "audit_evidence"},
    }


def compact_provenance_summary(summary, candidate_id, provenance_nodes=None, max_details=16):
    if summary is None:
        return None
    sites = summary.get("sites", [])
    origins = [origin for site in sites for origin in site.get("origins", [])]
    edges = [edge for site in sites for edge in site.get("edges", [])]
    calls = [call for site in sites for call in site.get("calls", [])]
    provenance_nodes = provenance_nodes or {}
    return {
        "sites": [{"site_id": site["site_id"], "site_ref": site.get("site_ref"), "api": site["api"],
                   "resolution": site["resolution"], "roles": site["roles"],
                   "origin_refs": [origin["id"] for origin in site.get("origins", [])],
                   "edge_refs": [edge["id"] for edge in site.get("edges", [])],
                   "call_refs": [call["id"] for call in site.get("calls", [])],
                   "limitations": site.get("limitations")} for site in sites],
        "origin_count": len(origins),
        "origins": [{**{key: origin.get(key) for key in ("id", "kind", "label", "boundary")},
                      "location": provenance_nodes.get(origin["id"], {}).get("location")}
                    for origin in origins[:max_details]],
        "omitted_origin_count": max(0, len(origins) - max_details),
        "edge_count": len(edges),
        "edges": edges[:max_details], "omitted_edge_count": max(0, len(edges) - max_details),
        "call_count": len(calls), "calls": calls[:max_details],
        "omitted_call_count": max(0, len(calls) - max_details),
        "truncation": summary.get("truncation", []),
        "evidence_store_ref": {"candidate_id": candidate_id, "field": "provenance_site_refs",
                                "projection": "argument_provenance"},
    }


def compact_control_flow_summary(summary, candidate_id, site_id, max_evidence=8):
    if summary is None:
        return None
    rows = []
    for row in summary["rows"]:
        block = row.get("block", -1)
        controlling = []
        for branch in row.get("branches") or []:
            controls = [control for control in branch.get("controls", []) if control.get("block") == block]
            if controls:
                controlling.append({key: branch.get(key) for key in
                                    ("block", "condition_kind", "condition_type", "condition_identity", "location")}
                                   | {"controls": controls})
        marker_kinds = collections.Counter(row.get("markers") or [])
        rows.append({
            "id": row["id"], "site": site_id, "api": row["api"],
            "status": row["status"], "complete": row["complete"],
            "resolution": row.get("resolution"), "resolutions": row.get("resolutions"),
            "function": row.get("function"), "function_variant": row.get("function_variant"),
            "block": block, "instruction": row.get("instruction", -1),
            "controlling_branches": controlling[:max_evidence],
            "controlling_branch_count": len(controlling),
            "omitted_controlling_branch_count": max(0, len(controlling) - max_evidence),
            "branch_count": len(row.get("branches") or []),
            "bypass_return_count": len(row.get("bypass_return_evidence") or []),
            "bypass_return_evidence": (row.get("bypass_return_evidence") or [])[:max_evidence],
            "omitted_bypass_return_evidence_count": max(
                0, len(row.get("bypass_return_evidence") or []) - max_evidence),
            "marker_counts": dict(sorted(marker_kinds.items())),
            "marker_evidence": (row.get("marker_evidence") or [])[:max_evidence],
            "omitted_marker_evidence_count": max(
                0, sum(marker_kinds.values()) - max_evidence),
            "instruction_candidates": row.get("instruction_candidates"),
        })
    return {"rows": rows, "truncated": summary["truncated"],
            "limitations": summary["limitations"],
            "evidence_store_ref": {"candidate_id": candidate_id, "field": "control_flow_row_refs",
                                    "projection": "control_flow"}}


def _compact_node_ref(node_id, nodes):
    node = nodes[node_id]
    return {"id": node_id, "package": node["package"], "function": node["function"]}


def _base_compact_graph(candidate, compact_graph, nodes):
    summary = compact_graph["summary"]
    initial_refs = sorted(set(summary["frontier_node_refs"]) | set(summary["target_node_refs"]))
    return {
        "summary": summary, "node_refs": initial_refs,
        "nodes": [_compact_node_ref(node_id, nodes) for node_id in initial_refs if node_id in nodes],
        "edge_refs": [], "edges": [], "segments": [],
        "cycle": summary["cycle"], "revisit": summary["revisit"], "truncated": summary["truncated"],
        "connectivity_complete": summary["edge_count"] == 0,
        "relationship_refs_complete": summary["edge_count"] == 0,
        "evidence_store_ref": {"candidate_id": candidate["id"], "field": "compact_graph_catalog"},
    }


def _refresh_compact_graph(packet_graph, total_edge_count):
    direct_refs = set(packet_graph.get("edge_refs", []))
    segment_refs = set()
    segment_edge_count = 0
    for segment in packet_graph.get("segments", []):
        segment_edge_count += segment["edge_count"]
        segment_refs.update(segment.get("edge_refs", []))
    explicit_refs = direct_refs | segment_refs
    represented_count = len(direct_refs) + segment_edge_count
    packet_graph["connectivity_complete"] = represented_count == total_edge_count
    packet_graph["relationship_refs_complete"] = len(explicit_refs) == total_edge_count
    packet_graph["deferred_relationship_count"] = max(0, total_edge_count - represented_count)
    packet_graph["deferred_relationship_ref_count"] = max(0, total_edge_count - len(explicit_refs))


def _add_compact_graph_item(packet_graph, item, nodes, candidate_id, include_edge_refs=True):
    for node_id in (item["from"], item["to"]):
        if node_id not in packet_graph["node_refs"]:
            packet_graph["node_refs"].append(node_id)
            packet_graph["nodes"].append(_compact_node_ref(node_id, nodes))
    packet_graph["node_refs"].sort()
    packet_graph["nodes"].sort(key=lambda node: node["id"])
    if item["type"] == "edge":
        packet_graph["edges"].append({"id": item["id"], "caller": item["from"], "callee": item["to"],
                                      "kind": item["kind"], "resolution": item["resolution"],
                                      "candidate": item["candidate"], "synthetic": item["synthetic"],
                                      **({"package_boundary": True} if item.get("package_boundary") else {})})
        packet_graph["edge_refs"].append(item["id"])
    else:
        segment = {key: item[key] for key in
                   ("id", "from", "to", "edge_count", "intermediate_node_count", "relationship_classes")}
        if include_edge_refs:
            segment["edge_refs"] = item["edge_refs"]
        else:
            segment["edge_refs_deferred"] = True
            segment["evidence_store_ref"] = {"candidate_id": candidate_id,
                                             "field": "compact_graph_catalog.segments",
                                             "segment_id": item["id"]}
        packet_graph["segments"].append(segment)
    _refresh_compact_graph(packet_graph, packet_graph["summary"]["edge_count"])


def _compact_operation(operation):
    fields = ("id", "api", "apis", "effects", "kind", "package", "function", "location",
              "uncertain", "possible_effects", "uncertainty_reasons", "dispatch_kinds")
    compact = {key: operation[key] for key in fields if key in operation}
    alternatives = []
    for row in operation.get("operations", []):
        if not isinstance(row, dict):
            continue
        alternative_fields = ("api", "kind", "effect", "effects", "resolution", "location", "uncertain",
                              "possible_effects", "uncertainty_reasons", "dispatch_kinds")
        alternative = {key: row[key] for key in alternative_fields if key in row}
        if alternative:
            alternatives.append(alternative)
    if alternatives:
        compact["operations"] = alternatives
    return compact


def _make_minimum_packet(candidate, report, compact_graph, source_items, unavailable,
                         audit_evidence, provenance_summary, provenance_nodes,
                         control_flow_summary, nodes):
    operation = candidate["operation"]
    operation_sources = [item for item in source_items if "operation_site" in item["roles"]]
    source_refs = [item["ref"] for item in operation_sources]
    unavailable_operation = [item for item in unavailable if item.get("role") == "operation_site"]
    graph = _base_compact_graph(candidate, compact_graph, nodes)
    frontiers = candidate["frontiers"]
    packet = {
        "candidate_id": candidate["id"], "question": candidate["question"],
        "operation_ref": candidate["site_id"], "operation": _compact_operation(operation),
        "categories": candidate["categories"],
        "frontier_candidates": frontiers[:16], "frontier_count": len(frontiers),
        "frontier_refs_omitted": max(0, len(frontiers) - 16),
        "graph": graph, "source_refs": source_refs, "source_excerpts": [],
        "unavailable_source": unavailable_operation[:8],
        "unavailable_source_count": len(unavailable),
        "audit_evidence": compact_audit_evidence(audit_evidence, candidate["id"]),
        "coverage": {
            "operation_sites_identified": report["coverage"].get("operation_sites_identified"),
            "report_truncated": report["truncated"],
            "graph_truncated": candidate["reverse"].get("truncated", {}),
            "analysis_complete": False,
            "evidence_note": "This is a bounded presentation of canonical evidence, not a complete investigation. Graph connectivity is structural and does not establish a runtime-feasible path.",
            "evidence_store_ref": {"candidate_id": candidate["id"], "field": "candidates"},
        },
    }
    compact_provenance = compact_provenance_summary(provenance_summary, candidate["id"], provenance_nodes)
    if compact_provenance is not None:
        packet["argument_provenance"] = compact_provenance
    compact_control_flow = compact_control_flow_summary(control_flow_summary, candidate["id"],
                                                         candidate["site_id"])
    if compact_control_flow is not None:
        packet["control_flow"] = compact_control_flow
    return packet


def _reduction_report(packet, state):
    candidate = state["candidate"]
    graph = packet["graph"]
    compact_graph = state["compact_graph"]
    total_nodes = compact_graph["summary"]["node_count"]
    total_edges = compact_graph["summary"]["edge_count"]
    represented_edges = set(graph.get("edge_refs", []))
    collapsed_edge_count = 0
    collapsed_node_count = 0
    for segment in graph.get("segments", []):
        represented_edges.update(segment.get("edge_refs", []))
        collapsed_edge_count += segment["edge_count"]
        collapsed_node_count += segment["intermediate_node_count"]
    represented_edge_count = len(graph.get("edge_refs", [])) + collapsed_edge_count
    included_node_refs = len(set(graph.get("node_refs", [])))
    represented_node_count = min(total_nodes, included_node_refs + collapsed_node_count)
    source_refs = packet.get("source_refs", [])
    source_ref_ids = {ref["id"] for ref in source_refs}
    included_excerpt_ids = {excerpt["id"] for excerpt in packet.get("source_excerpts", [])}
    all_sources = state["source_items"]
    # Only the complete source inventory is immutable during a reduction. Packet
    # reference/excerpt selections above must be recomputed after every trial.
    source_ref_ids_all = state.get("source_ref_ids_all")
    if source_ref_ids_all is None:
        source_ref_ids_all = {item["ref"]["id"] for item in all_sources}

    provenance_full = state["provenance_summary"]
    provenance_packet = packet.get("argument_provenance")
    provenance_sites = provenance_full.get("sites", []) if provenance_full else []
    total_origins = sum(len(site.get("origins", [])) for site in provenance_sites)
    total_origin_edges = sum(len(site.get("edges", [])) for site in provenance_sites)
    total_calls = sum(len(site.get("calls", [])) for site in provenance_sites)
    if provenance_packet:
        if "origin_count" in provenance_packet:
            included_origins = len(provenance_packet.get("origins", []))
            included_origin_edges = len(provenance_packet.get("edges", []))
            included_calls = len(provenance_packet.get("calls", []))
        else:
            included_origins = sum(len(site.get("origins", [])) for site in provenance_packet.get("sites", []))
            included_origin_edges = sum(len(site.get("edges", [])) for site in provenance_packet.get("sites", []))
            included_calls = sum(len(site.get("calls", [])) for site in provenance_packet.get("sites", []))
    else:
        included_origins = included_origin_edges = included_calls = 0

    control_full = state["control_flow_summary"]
    control_packet = packet.get("control_flow")
    total_cf_rows = len(control_full.get("rows", [])) if control_full else 0
    total_cf_branches = sum(len(row.get("branches") or []) for row in control_full.get("rows", [])) if control_full else 0
    total_cf_bypass = sum(len(row.get("bypass_return_evidence") or [])
                          for row in control_full.get("rows", [])) if control_full else 0
    total_cf_markers = sum(len(row.get("markers") or []) for row in control_full.get("rows", [])) if control_full else 0
    if control_packet:
        included_cf_rows = len(control_packet.get("rows", []))
        if control_packet.get("rows") and "branch_count" in control_packet["rows"][0]:
            included_cf_branches = sum(len(row.get("controlling_branches", []))
                                       for row in control_packet["rows"])
        else:
            included_cf_branches = sum(len(row.get("branches") or []) for row in control_packet.get("rows", []))
        included_cf_bypass = sum(len(row.get("bypass_return_evidence") or [])
                                 for row in control_packet.get("rows", []))
        included_cf_markers = sum(len(row.get("marker_evidence") or [])
                                  for row in control_packet.get("rows", []))
    else:
        included_cf_rows = included_cf_branches = included_cf_bypass = included_cf_markers = 0

    audit_packet = packet.get("audit_evidence", {})
    audit_is_full = "evidence_store_ref" not in audit_packet
    audit_functions = audit_packet.get("functions", [])
    included_audit_functions = len(audit_functions) if isinstance(audit_functions, list) else 0
    audit_full = state["audit_evidence"]
    audit_function_count = len(audit_full.get("functions", []))

    return {
        "strategy": "expanded" if state["strategy"] == "expanded" else "progressive",
        "steps": list(state["steps"]),
        "original_expanded_bytes": state["original_expanded_bytes"],
        "final_serialized_bytes": 0,
        "net_bytes_saved": 0,
        "graph": {
            "total_node_refs": total_nodes, "included_node_refs": included_node_refs,
            "collapsed_intermediate_nodes": collapsed_node_count,
            "represented_node_count": represented_node_count,
            "deferred_node_refs": max(0, total_nodes - included_node_refs),
            "total_edge_refs": total_edges, "included_direct_edge_refs": len(graph.get("edge_refs", [])),
            "included_collapsed_edge_count": collapsed_edge_count,
            "included_relationship_refs": len(represented_edges),
            "deferred_relationship_refs": max(0, total_edges - len(represented_edges)),
            "deferred_relationship_count": max(0, total_edges - represented_edge_count),
            "collapsed_segment_count": len(graph.get("segments", [])),
            "candidate_edge_count": compact_graph["summary"]["candidate_edge_count"],
            "unresolved_edge_count": compact_graph["summary"]["unresolved_edge_count"],
            "connectivity_complete": graph.get("connectivity_complete", represented_edge_count == total_edges),
            "relationship_refs_complete": graph.get("relationship_refs_complete", len(represented_edges) == total_edges),
            "evidence_store_ref": {"candidate_id": candidate["id"],
                                    "candidate_field": "edge_ids", "catalog": "relationship_catalog"},
            "segment_store_ref": {"candidate_id": candidate["id"],
                                   "candidate_field": "compact_graph_catalog"},
        },
        "source": {
            "total_excerpt_refs": len(all_sources),
            "included_excerpt_count": len(included_excerpt_ids),
            "included_excerpt_refs_field": "source_excerpts",
            "deferred_excerpt_count": max(0, len(source_ref_ids_all) - len(included_excerpt_ids)),
            "included_source_ref_count": len(source_ref_ids & source_ref_ids_all),
            "deferred_source_ref_count": max(0, len(source_ref_ids_all) - len(source_ref_ids)),
            "unavailable_source_count": len(state["unavailable_source"]),
            "evidence_store_ref": {"candidate_id": candidate["id"], "candidate_field": "source_refs",
                                    "retrieval": "manifest path, line range, and sha256"},
        },
        "audit": {
            "site_summary_included": bool(audit_packet.get("site")),
            "function_summary_count": (audit_packet.get("functions", {}).get("count", 0)
                                       if isinstance(audit_packet.get("functions"), dict) else included_audit_functions),
            "full_function_records_included": included_audit_functions if audit_is_full else 0,
            "deferred_function_record_count": max(0, audit_function_count - included_audit_functions),
            "evidence_store_ref": {"candidate_id": candidate["id"], "candidate_field": "audit_evidence"},
        },
        "argument_provenance": {
            "site_count": len(provenance_sites), "included_origin_details": included_origins,
            "deferred_origin_details": max(0, total_origins - included_origins),
            "included_edge_details": included_origin_edges,
            "deferred_edge_details": max(0, total_origin_edges - included_origin_edges),
            "included_call_details": included_calls,
            "deferred_call_details": max(0, total_calls - included_calls),
            "truncation": provenance_full.get("truncation", []) if provenance_full else [],
            "evidence_store_ref": {"candidate_id": candidate["id"], "candidate_field": "provenance_site_refs",
                                    "projection": "argument_provenance"},
        },
        "control_flow": {
            "total_row_count": total_cf_rows, "included_row_count": included_cf_rows,
            "total_branch_count": total_cf_branches, "included_branch_detail_count": included_cf_branches,
            "deferred_branch_detail_count": max(0, total_cf_branches - included_cf_branches),
            "total_bypass_return_count": total_cf_bypass,
            "included_bypass_return_detail_count": included_cf_bypass,
            "deferred_bypass_return_detail_count": max(0, total_cf_bypass - included_cf_bypass),
            "total_marker_count": total_cf_markers,
            "included_marker_detail_count": included_cf_markers,
            "deferred_marker_detail_count": max(0, total_cf_markers - included_cf_markers),
            "truncated": control_full.get("truncated", {}) if control_full else {},
            "evidence_store_ref": {"candidate_id": candidate["id"], "candidate_field": "control_flow_row_refs",
                                    "projection": "control_flow"},
        },
        "analysis_truncation": {
            "effect_report": state["report_truncated"],
            "reverse_graph": candidate["reverse"].get("truncated", {}),
            "argument_provenance": provenance_full.get("truncation", []) if provenance_full else [],
            "control_flow": control_full.get("truncated", {}) if control_full else {},
        },
        "evidence_store_ref": {"candidate_id": candidate["id"], "candidate_field": "candidates"},
    }


def _checked_reduction_size(packet, expected_size):
    """Independently verify completed packets; never trust a stale size report."""
    actual_size = len(canonical_json(packet))
    report = packet["reduction_report"]
    if (actual_size != expected_size or report["final_serialized_bytes"] != actual_size or
            report["net_bytes_saved"] != report["original_expanded_bytes"] - actual_size):
        raise ValueError("packet canonical size does not match reduction report")
    return actual_size


def _refresh_reduction_report(packet, state, normalize=True, verify=True):
    if normalize:
        normalized = normalize_model_paths(packet, state.get("source_root"), state.get("manifest"))
        packet.clear()
        packet.update(normalized)
    packet["reduction_report"] = _reduction_report(packet, state)
    report = packet["reduction_report"]
    # Only these two integers change during convergence. Measure all other UTF-8
    # bytes (including escaping, keys and delimiters) once. JSON integers have
    # exactly their decimal spelling, including a minus sign when savings < 0.
    # This is local to this refresh: no trial/rollback or mutable evidence is cached.
    fixed_bytes = (len(canonical_json(packet)) - len(str(report["final_serialized_bytes"]))
                   - len(str(report["net_bytes_saved"])))
    for _ in range(8):
        size = (fixed_bytes + len(str(report["final_serialized_bytes"]))
                + len(str(report["net_bytes_saved"])))
        saved = report["original_expanded_bytes"] - size
        if report["final_serialized_bytes"] == size and report["net_bytes_saved"] == saved:
            return _checked_reduction_size(packet, size) if verify else size
        report["final_serialized_bytes"] = size
        report["net_bytes_saved"] = saved
    # Preserve the original eight-pass trial fallback. A completed packet must
    # additionally pass the independent report/size check, even at this boundary.
    if verify:
        return _checked_reduction_size(packet, report["final_serialized_bytes"])
    return len(canonical_json(packet))


def _add_source_item(packet, item):
    if any(ref["id"] == item["ref"]["id"] for ref in packet["source_refs"]):
        return False
    packet["source_refs"].append(item["ref"])
    packet["source_excerpts"].append(item["excerpt"])
    return True


def _try_packet_addition(packet, state, max_packet_bytes, label, rollback):
    added_step = label not in state["steps"]
    if added_step:
        state["steps"].append(label)
    # Graph trials add only IDs, package/function strings, relationship classes and
    # counts; none introduces file/path fields. The minimum packet was normalized
    # before trials began. Other additions can contain new locations and must still
    # pass through the full normalizer (including containment and hash checks).
    graph_trial = label in ("caller_relationships", "compact_call_chain_segments",
                            "compact_segment_references")
    size = _refresh_reduction_report(packet, state, normalize=not graph_trial, verify=False)
    if size <= max_packet_bytes:
        return True
    if added_step:
        state["steps"].pop()
    rollback()
    _refresh_reduction_report(packet, state, normalize=not graph_trial, verify=False)
    return False


def _graph_snapshot(packet_graph):
    return {key: list(packet_graph[key]) for key in ("node_refs", "nodes", "edge_refs", "edges", "segments")}


def _restore_graph(packet_graph, snapshot):
    packet_graph.update(snapshot)
    _refresh_compact_graph(packet_graph, packet_graph["summary"]["edge_count"])


def _progressively_reduce_packet(candidate, report, original_expanded_bytes,
                                 compact_graph, source_items, unavailable, audit_evidence,
                                 provenance_summary, provenance_nodes, control_flow_summary,
                                 nodes, max_packet_bytes, source_root, manifest):
    packet = _make_minimum_packet(candidate, report, compact_graph, source_items, unavailable,
                                  audit_evidence, provenance_summary, provenance_nodes,
                                  control_flow_summary, nodes)
    state = {"candidate": candidate, "compact_graph": compact_graph,
             "source_items": source_items, "unavailable_source": unavailable,
             "audit_evidence": audit_evidence, "provenance_summary": provenance_summary,
             "control_flow_summary": control_flow_summary, "report_truncated": report["truncated"],
             # source_items and their ref IDs are fixed for this invocation; only
             # packet selections change. Never share this snapshot across candidates
             # or invocations, and never cache selected packet IDs here.
             "source_ref_ids_all": frozenset(item["ref"]["id"] for item in source_items),
             "original_expanded_bytes": original_expanded_bytes, "strategy": "progressive",
             "source_root": source_root, "manifest": manifest,
             "steps": ["minimum_investigation_packet"]}
    minimum_size = _refresh_reduction_report(packet, state)
    if minimum_size > max_packet_bytes:
        return packet, minimum_size, False

    operation_sources = [item for item in source_items if "operation_site" in item["roles"]]
    for item in operation_sources:
        old_refs, old_excerpts = list(packet["source_refs"]), list(packet["source_excerpts"])
        if not _add_source_item(packet, item):
            continue
        _try_packet_addition(packet, state, max_packet_bytes, "operation_local_source",
                             lambda: packet.update({"source_refs": old_refs, "source_excerpts": old_excerpts}))

    if provenance_summary is not None:
        old = packet.get("argument_provenance")
        packet["argument_provenance"] = provenance_summary
        _try_packet_addition(packet, state, max_packet_bytes, "expanded_argument_origins",
                             lambda: packet.__setitem__("argument_provenance", old))

    if control_flow_summary is not None:
        old = packet.get("control_flow")
        packet["control_flow"] = control_flow_summary
        _try_packet_addition(packet, state, max_packet_bytes, "expanded_control_flow_details",
                             lambda: packet.__setitem__("control_flow", old))

    for item in compact_graph["items"]:
        if item["type"] == "segment":
            full_segment = {"type": "segment", **item}
            snapshot = _graph_snapshot(packet["graph"])
            _add_compact_graph_item(packet["graph"], full_segment, nodes, candidate["id"], True)
            included = _try_packet_addition(packet, state, max_packet_bytes, "compact_call_chain_segments",
                                            lambda: _restore_graph(packet["graph"], snapshot))
            if not included:
                header_snapshot = _graph_snapshot(packet["graph"])
                header_segment = {"type": "segment", **{key: value for key, value in item.items() if key != "edge_refs"}}
                _add_compact_graph_item(packet["graph"], header_segment, nodes, candidate["id"], False)
                _try_packet_addition(packet, state, max_packet_bytes, "compact_segment_references",
                                     lambda: _restore_graph(packet["graph"], header_snapshot))
        else:
            snapshot = _graph_snapshot(packet["graph"])
            _add_compact_graph_item(packet["graph"], item, nodes, candidate["id"])
            _try_packet_addition(packet, state, max_packet_bytes, "caller_relationships",
                                 lambda: _restore_graph(packet["graph"], snapshot))

    for item in sorted(source_items, key=lambda entry: (entry["priority"], entry["ref"]["id"])):
        if "operation_site" in item["roles"]:
            continue
        old_refs, old_excerpts = list(packet["source_refs"]), list(packet["source_excerpts"])
        if not _add_source_item(packet, item):
            continue
        _try_packet_addition(packet, state, max_packet_bytes, "boundary_and_extended_source",
                             lambda: packet.update({"source_refs": old_refs, "source_excerpts": old_excerpts}))

    old_audit = packet["audit_evidence"]
    packet["audit_evidence"] = audit_evidence
    _try_packet_addition(packet, state, max_packet_bytes, "extended_audit_evidence",
                         lambda: packet.__setitem__("audit_evidence", old_audit))
    final_size = _refresh_reduction_report(packet, state)
    return packet, final_size, final_size <= max_packet_bytes


EVENT_FIELDS = {
    "reffector_invocation": {"invocation_id", "target_revision", "argv", "status", "exit_code", "binary_sha256"},
    "source_read": {"source_path", "start_line", "end_line", "status", "reason", "tool_call_id",
                    "requested_start_line", "requested_line_count"},
    "source_search": {"query", "scope", "status", "matches", "tool_call_id"},
    "candidate": {"candidate_id", "status"},
    "reproduction": {"candidate_id", "attempt", "outcome"},
    "operation_group": {"group_id", "operation_ids"},
    "termination": {"reason", "wall_duration_ms", "active_duration_ms", "active_duration_status"},
    "tool_invocation": {"tool_call_id", "tool_name"},
    "tool_result": {"tool_call_id", "tool_name", "status"},
    "test_command": {"tool_call_id", "command", "argv", "exit_status"},
    "session_metadata": {"kind", "provider", "model", "modelId", "thinkingLevel", "usage", "usage_kind", "timestamp", "session_id", "role", "response_id",
                         "run_id", "started_at", "ended_at", "wall_duration_ms", "source_revision", "analyzer_revision",
                         "process_exit_code", "timed_out"},
    "supervisor_decision": {"status", "decision", "timestamp", "rationale", "scope_adequate", "another_phase_warranted"},
    "worker_lifecycle": {"status"},
}


def validate_event_ledger(events):
    """Validate externally supplied observations; never infer absent activity."""
    result = {"availability": "provided" if events is not None else "not_provided",
              "completeness": "only supplied ledger entries; not independently verified",
              "reffector_invocations": [], "source_reads": [], "source_searches": [],
              "candidates": [], "reproduction_attempts": [], "operation_groups": [],
              "termination": None, "recorded_counts": None}
    for number, event in enumerate(events or (), 1):
        require_object(event, f"event {number}")
        kind = event.get("type")
        if kind not in EVENT_FIELDS:
            raise ValueError(f"event {number} has unsupported type")
        if set(event) - (EVENT_FIELDS[kind] | {"type", "evidence_ref", "evidence_refs", "session_id", "worker_id"}):
            raise ValueError(f"event {number} has unsupported fields")
        row = {key: value for key, value in event.items() if key != "type"}
        for field in ("session_id", "worker_id"):
            if field in row and (not isinstance(row[field], str) or not row[field]):
                raise ValueError(f"event {number} invalid {field}")
        if "evidence_refs" in row and not isinstance(row["evidence_refs"], list):
            raise ValueError(f"event {number} invalid evidence_refs")
        for evidence_ref in ([row["evidence_ref"]] if "evidence_ref" in row else []) + row.get("evidence_refs", []):
            has_line = isinstance(evidence_ref, dict) and isinstance(evidence_ref.get("line"), int) and not isinstance(evidence_ref.get("line"), bool) and evidence_ref["line"] >= 1
            pointer = evidence_ref.get("json_pointer") if isinstance(evidence_ref, dict) else None
            has_pointer = isinstance(pointer, str) and (pointer == "" or pointer.startswith("/")) and not re.search(r"~(?![01])", pointer)
            pointer_invalid_when_present = isinstance(evidence_ref, dict) and "json_pointer" in evidence_ref and not has_pointer
            if (not isinstance(evidence_ref, dict) or set(evidence_ref) - {"input", "line", "event_id", "json_pointer"} or
                    not isinstance(evidence_ref.get("input"), str) or not evidence_ref["input"] or
                    pointer_invalid_when_present or not (has_line or has_pointer) or
                    ("event_id" in evidence_ref and not isinstance(evidence_ref["event_id"], str))):
                raise ValueError(f"event {number} invalid evidence_ref")
        if kind == "source_read":
            source_path = row.get("source_path")
            if not isinstance(source_path, str) or not source_path or row.get("status") not in ("success", "failed", "requested", "unavailable"):
                raise ValueError(f"event {number} invalid source_read")
            relative = pathlib.PurePosixPath(source_path)
            windows = pathlib.PureWindowsPath(source_path)
            if (source_path == "." or relative.is_absolute() or windows.drive or ".." in relative.parts or
                    relative.as_posix() != source_path or windows.as_posix() != source_path):
                raise ValueError(f"event {number} source_path must be canonical and repository-relative")
            if "tool_call_id" in row and (not isinstance(row["tool_call_id"], str) or not row["tool_call_id"]):
                raise ValueError(f"event {number} invalid source_read tool_call_id")
            if "reason" in row and not isinstance(row["reason"], str):
                raise ValueError(f"event {number} invalid source_read reason")
            for field in ("requested_start_line", "requested_line_count"):
                if field in row and row[field] is not None and (not isinstance(row[field], int) or isinstance(row[field], bool) or row[field] < 1):
                    raise ValueError(f"event {number} invalid {field}")
            if ("start_line" in row) != ("end_line" in row):
                raise ValueError(f"event {number} source range requires both lines")
            for field in ("start_line", "end_line"):
                if field in row and (not isinstance(row[field], int) or isinstance(row[field], bool) or row[field] < 1):
                    raise ValueError(f"event {number} invalid {field}")
            if "start_line" in row and row["start_line"] > row["end_line"]:
                raise ValueError(f"event {number} reversed source range")
            result["source_reads"].append(row)
        elif kind == "reffector_invocation":
            required_strings = ("invocation_id", "target_revision", "status")
            if any(not isinstance(row.get(field), str) or not row[field] for field in required_strings):
                raise ValueError(f"event {number} missing required invocation fields")
            if not isinstance(row.get("argv"), list) or not row["argv"] or any(not isinstance(arg, str) for arg in row["argv"]):
                raise ValueError(f"event {number} invalid invocation argv")
            if "exit_code" in row and (not isinstance(row["exit_code"], int) or isinstance(row["exit_code"], bool)):
                raise ValueError(f"event {number} invalid invocation exit_code")
            if "binary_sha256" in row and (not isinstance(row["binary_sha256"], str) or not re.fullmatch(r"[0-9a-fA-F]{64}", row["binary_sha256"])):
                raise ValueError(f"event {number} invalid binary_sha256")
            result["reffector_invocations"].append(row)
        elif kind == "termination":
            if result["termination"] is not None or not isinstance(row.get("reason"), str) or not row["reason"]:
                raise ValueError(f"event {number} invalid/duplicate termination")
            if row.get("active_duration_status") not in ("measured", "unavailable"):
                raise ValueError(f"event {number} invalid active_duration_status")
            if (row["active_duration_status"] == "measured") != ("active_duration_ms" in row):
                raise ValueError(f"event {number} active duration/status mismatch")
            for field in ("wall_duration_ms", "active_duration_ms"):
                if field in row and (not isinstance(row[field], int) or isinstance(row[field], bool) or row[field] < 0):
                    raise ValueError(f"event {number} invalid {field}")
            result["termination"] = row
        elif kind in ("tool_invocation", "tool_result", "test_command", "session_metadata", "supervisor_decision", "worker_lifecycle"):
            required = {"tool_invocation": ("tool_call_id", "tool_name"),
                        "tool_result": ("tool_call_id", "tool_name", "status"),
                        "test_command": ("tool_call_id", "command", "exit_status"),
                        "session_metadata": ("kind",),
                        "supervisor_decision": ("status",),
                        "worker_lifecycle": ("status",)}[kind]
            if any(not isinstance(row.get(field), str) or not row[field] for field in required):
                raise ValueError(f"event {number} missing required {kind} fields")
            if kind == "tool_result" and row["status"] not in ("success", "failed", "unavailable"):
                raise ValueError(f"event {number} invalid tool result status")
            if kind == "worker_lifecycle" and row["status"] not in ("started", "completed", "aborted", "failed"):
                raise ValueError(f"event {number} invalid worker lifecycle status")
            if kind == "test_command" and (not isinstance(row.get("argv"), list) or
                    any(not isinstance(arg, str) for arg in row["argv"])):
                raise ValueError(f"event {number} invalid test command argv")
            if kind == "session_metadata":
                for field in ("provider", "model", "modelId", "thinkingLevel", "session_id", "role", "response_id",
                              "run_id", "started_at", "ended_at", "source_revision", "analyzer_revision", "usage_kind"):
                    if field in row and (not isinstance(row[field], str) or not row[field]):
                        raise ValueError(f"event {number} invalid session metadata {field}")
                if "timestamp" in row and (not isinstance(row["timestamp"], (str, int, float)) or isinstance(row["timestamp"], bool)):
                    raise ValueError(f"event {number} invalid session timestamp")
                if "wall_duration_ms" in row and (not isinstance(row["wall_duration_ms"], int) or
                        isinstance(row["wall_duration_ms"], bool) or row["wall_duration_ms"] < 0):
                    raise ValueError(f"event {number} invalid run wall duration")
                if "process_exit_code" in row and (not isinstance(row["process_exit_code"], int) or isinstance(row["process_exit_code"], bool)):
                    raise ValueError(f"event {number} invalid runner process exit code")
                if "timed_out" in row and not isinstance(row["timed_out"], bool):
                    raise ValueError(f"event {number} invalid runner timeout flag")
            if kind == "supervisor_decision":
                if row["status"] not in ("missing", "recorded"):
                    raise ValueError(f"event {number} invalid supervisor decision status")
                if row["status"] == "recorded":
                    timestamp = row.get("timestamp")
                    timestamp_valid = False
                    if isinstance(timestamp, str):
                        try:
                            datetime.datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
                            timestamp_valid = True
                        except ValueError:
                            pass
                    if row.get("decision") not in ("continue", "stop") or not timestamp_valid or not isinstance(row.get("rationale"), str) or not row["rationale"].strip():
                        raise ValueError(f"event {number} malformed recorded supervisor decision")
                    if any(not isinstance(row.get(field), bool) for field in ("scope_adequate", "another_phase_warranted")):
                        raise ValueError(f"event {number} recorded supervisor decision requires boolean scope facts")
                    if "evidence_ref" not in row:
                        raise ValueError(f"event {number} recorded supervisor decision requires evidence_ref")
            result.setdefault({"tool_invocation": "tool_invocations", "tool_result": "tool_results",
                               "test_command": "test_commands", "session_metadata": "session_metadata",
                               "supervisor_decision": "supervisor_decisions",
                               "worker_lifecycle": "worker_lifecycle"}[kind], []).append(row)
        else:
            required_strings = {
                "source_search": ("query", "scope", "status"),
                "candidate": ("candidate_id", "status"),
                "reproduction": ("candidate_id", "attempt", "outcome"),
                "operation_group": ("group_id",),
            }[kind]
            if any(not isinstance(row.get(field), str) or not row[field] for field in required_strings):
                raise ValueError(f"event {number} missing required string fields")
            if kind == "source_search" and "tool_call_id" in row and (not isinstance(row["tool_call_id"], str) or not row["tool_call_id"]):
                raise ValueError(f"event {number} invalid source_search tool_call_id")
            if kind == "source_search" and "matches" in row and (not isinstance(row["matches"], int) or isinstance(row["matches"], bool) or row["matches"] < 0):
                raise ValueError(f"event {number} invalid search matches")
            if kind == "operation_group" and (not isinstance(row.get("operation_ids"), list) or any(not isinstance(item, str) or not item for item in row["operation_ids"])):
                raise ValueError(f"event {number} invalid operation_ids")
            target = {"source_search": "source_searches", "candidate": "candidates",
                      "reproduction": "reproduction_attempts", "operation_group": "operation_groups"}[kind]
            result[target].append(row)
    if events is not None:
        successful_paths = {event["source_path"] for event in result["source_reads"] if event["status"] == "success"}
        result["recorded_counts"] = {
            "reffector_invocations": len(result["reffector_invocations"]),
            "source_read_events": len(result["source_reads"]),
            "successful_source_read_events": sum(event["status"] == "success" for event in result["source_reads"]),
            "failed_source_read_events": sum(event["status"] == "failed" for event in result["source_reads"]),
            "distinct_successfully_read_paths": len(successful_paths),
            "source_search_events": len(result["source_searches"]),
            "candidate_status_events": len(result["candidates"]),
            "distinct_candidate_ids": len({event["candidate_id"] for event in result["candidates"]}),
            "reproduction_attempts": len(result["reproduction_attempts"]),
            "distinct_operation_groups": len({event["group_id"] for event in result["operation_groups"]}),
            "tool_invocations": len(result.get("tool_invocations", [])),
            "tool_results": len(result.get("tool_results", [])),
            "test_commands": len(result.get("test_commands", [])),
            "worker_lifecycle_events": len(result.get("worker_lifecycle", [])),
            "supervisor_decision_records": len(result.get("supervisor_decisions", [])),
        }
    return result


def build(report, audit=(), max_candidates=100, max_packet_bytes=48000,
          source_root=None, manifest=None, program_model=None, argument_provenance=None,
          control_flow=None, event_ledger=None):
    if not isinstance(max_candidates, int) or max_candidates <= 0:
        raise ValueError("max_candidates must be positive")
    if not isinstance(max_packet_bytes, int) or max_packet_bytes <= 0:
        raise ValueError("max_packet_bytes must be positive")
    ledger = validate_event_ledger(event_ledger)
    indexes = validate_effect_report(report)
    sites, nodes, edges = (indexes[key] for key in ("sites", "nodes", "edges"))
    provenance = validate_argument_provenance(argument_provenance, sites) if argument_provenance is not None else None
    control_flow = validate_control_flow(control_flow, sites) if control_flow is not None else None
    audit_sites, audit_functions, audit_site_total, audit_function_total = validate_audit(audit)
    declarations = validate_program_model(program_model)
    if manifest is not None:
        require_object(manifest, "source manifest")
        require_object(manifest.get("files"), "source manifest files")
    if source_root and manifest is None:
        raise ValueError("source root requires a source manifest")
    reverse = {row["site"]: row for row in report.get("reverse", [])}
    candidates = []
    raw_node_refs = raw_edge_refs = 0
    unique_node_ids, unique_edge_ids = set(), set()

    for site in report["sites"]:
        rev = reverse.get(site["id"], {"nodes": [], "edges": [], "truncated": {}})
        node_refs = sorted({ref["id"] for ref in rev["nodes"]})
        edge_refs = sorted(set(rev["edges"]))
        raw_node_refs += len(rev["nodes"])
        raw_edge_refs += len(rev["edges"])
        unique_node_ids.update(node_refs)
        unique_edge_ids.update(edge_refs)
        in_degree = {node_id: 0 for node_id in node_refs}
        out_degree = {node_id: 0 for node_id in node_refs}
        for edge_id in edge_refs:
            edge = edges[edge_id]
            out_degree[edge["caller"]] = out_degree.get(edge["caller"], 0) + 1
            in_degree[edge["callee"]] = in_degree.get(edge["callee"], 0) + 1
        frontiers = sorted(node_id for node_id in node_refs
                           if out_degree.get(node_id, 0) and not in_degree.get(node_id, 0))
        categories = []
        site_audit = audit_sites.get(site["id"])
        if len(frontiers) > 1:
            categories.append(CATEGORIES[0])
        unresolved_edge = any(
            edges[eid]["candidate"] or (edges[eid].get("source") or {}).get("resolution") != "established"
            for eid in edge_refs
        )
        audit_uncertain = bool(site_audit and (
            site_audit.get("uncertain") or site_audit.get("vta_candidates")
            or site_audit.get("possible_effects") or site_audit.get("uncertainty_reasons")
        ))
        if unresolved_edge or site.get("uncertain") or audit_uncertain:
            categories.append(CATEGORIES[1])
        if any((edges[eid].get("source") or {}).get("kind") in ("go", "defer") for eid in edge_refs):
            categories.append(CATEGORIES[2])
        if any(degree > 1 for degree in out_degree.values()) or any(degree > 1 for degree in in_degree.values()):
            categories.append(CATEGORIES[3])
        packages = {nodes[node_id]["package"] for node_id in node_refs}
        if len(packages) > 1:
            categories.append(CATEGORIES[4])
        if not categories:
            continue

        owners = {(site["package"], site["function"])}
        owners.update((nodes[node_id]["package"], nodes[node_id]["function"]) for node_id in node_refs)
        joined_functions = [audit_functions[key] for key in sorted(owners) if key in audit_functions]
        questions = []
        operation = site.get("api") or ", ".join(site.get("apis", [])) or site["kind"]
        if CATEGORIES[0] in categories:
            questions.append(f"Which distinct graph-frontier caller paths lead to {operation}, and what context differs?")
        if CATEGORIES[1] in categories:
            questions.append(f"Which possible or unresolved alternatives affect {operation}, and what evidence distinguishes them?")
        if CATEGORIES[2] in categories:
            questions.append(f"What lifetime or ordering behavior does the go/defer relationship imply for {operation}?")
        if CATEGORIES[3] in categories:
            questions.append(f"Where do caller paths branch or converge around {operation}, and what does each path do?")
        if CATEGORIES[4] in categories:
            questions.append(f"Which distinct package-level caller groups share {operation}, and what behavior differs between them?")
        candidates.append({
            "id": stable_candidate_id(site["id"], edge_refs), "site_id": site["id"],
            "categories": categories, "frontiers": frontiers, "node_ids": node_refs,
            "edge_ids": edge_refs, "question": " ".join(questions), "operation": site,
            "audit_site": site_audit, "audit_functions": joined_functions,
            "reverse": rev,
        })

    candidates.sort(key=lambda item: item["id"])
    selected_candidate_ids = {candidate["id"] for candidate in candidates[:max_candidates]}
    node_catalog = {node_id: nodes[node_id] for node_id in sorted(unique_node_ids)}
    relationship_catalog = {edge_id: edges[edge_id] for edge_id in sorted(unique_edge_ids)}
    packets, deferred, inventory = [], [], []
    for candidate in candidates:
        compact_graph = compact_graph_evidence(candidate, nodes, edges)
        source_items_by_key, unavailable = {}, []
        owner = (candidate["operation"]["package"], candidate["operation"]["function"])
        frontier_functions = {(nodes[node_id]["package"], nodes[node_id]["function"])
                              for node_id in candidate["frontiers"]}
        anchor_functions = {(nodes[node_id]["package"], nodes[node_id]["function"])
                            for node_id in compact_graph["anchors"] if node_id in nodes}
        locations = [(candidate["operation"].get("location", {}), None, 0, "operation_site")]
        declaration_keys = {owner}
        declaration_keys.update((nodes[node_id]["package"], nodes[node_id]["function"])
                                for node_id in candidate["node_ids"])
        for package, function in sorted(declaration_keys):
            key = (package, function)
            if key == owner:
                priority, role = 1, "operation_declaration"
            elif key in frontier_functions:
                priority, role = 2, "frontier_declaration"
            elif key in anchor_functions:
                priority, role = 3, "graph_boundary_declaration"
            else:
                priority, role = 6, "intermediate_declaration"
            matches = declarations.get(key)
            if matches is None:
                matches = declarations.get((package, function.rsplit(".", 1)[-1]), [])
            if not matches:
                unavailable.append({"symbol": {"package": package, "function": function},
                                    "reason": "declaration absent from supplied program model", "role": role})
            for declaration in matches:
                locations.append(({"file": declaration["file"], "line": declaration["start_line"],
                                   "declaration": True}, declaration, priority, role))

        for location, declaration, priority, role in locations:
            resolved, error = safe_source(location.get("file", ""), source_root, manifest)
            if error:
                unavailable.append({"location": location, "reason": error, "role": role})
                continue
            relative, content, digest = resolved
            start = declaration["start_line"] if declaration else max(1, int(location.get("line", 1)) - 3)
            lines = content.decode("utf-8", errors="replace").splitlines()
            end = declaration["end_line"] if declaration else min(len(lines), int(location.get("line", 1)) + 3)
            key = (relative, start, end)
            if start < 1 or end > len(lines) or end < start:
                unavailable.append({"location": location, "reason": "source range outside manifest file", "role": role})
                continue
            reference = {"path": relative, "sha256": digest, "start_line": start, "end_line": end}
            ref_id = stable_reference_id("source", reference)
            source_ref = {**reference, "id": ref_id, "roles": [role]}
            excerpt = {**source_ref, "text": "\n".join(
                f"{number}: {lines[number - 1]}" for number in range(start, end + 1))}
            if key in source_items_by_key:
                item = source_items_by_key[key]
                if role not in item["roles"]:
                    item["roles"].append(role)
                    item["roles"].sort()
                item["priority"] = min(item["priority"], priority)
                item["ref"]["roles"] = list(item["roles"])
                item["excerpt"]["roles"] = list(item["roles"])
                continue
            source_items_by_key[key] = {"ref": source_ref, "excerpt": excerpt,
                                        "priority": priority, "roles": [role]}
        source_items = list(source_items_by_key.values())
        source_refs = [item["ref"] for item in source_items]
        excerpts = [item["excerpt"] for item in source_items]
        legacy_source_refs = [{key: value for key, value in ref.items() if key not in ("id", "roles")}
                              for ref in source_refs]
        legacy_excerpts = [{key: value for key, value in excerpt.items() if key not in ("id", "roles")}
                           for excerpt in excerpts]

        audit_evidence = {"site": candidate["audit_site"], "functions": candidate["audit_functions"]}
        graph_status = {"cycle": candidate["reverse"].get("cycle", False),
                        "revisit": candidate["reverse"].get("revisit", False),
                        "truncated": candidate["reverse"].get("truncated", {})}
        graph = {
            "node_refs": candidate["node_ids"],
            "nodes": [{"id": node_id, "package": nodes[node_id]["package"], "function": nodes[node_id]["function"]}
                      for node_id in candidate["node_ids"]],
            "edge_refs": candidate["edge_ids"],
            "edges": [graph_summary(edges[edge_id], nodes) for edge_id in candidate["edge_ids"]],
            **graph_status,
        }
        provenance_summary = provenance_packet_summary(candidate["operation"], provenance, {candidate["site_id"]})
        control_flow_summary = control_flow_packet_summary(candidate["operation"], control_flow)
        legacy_unavailable = [{key: value for key, value in item.items() if key != "role"}
                              for item in unavailable]
        legacy_packet = {
            "candidate_id": candidate["id"], "question": candidate["question"],
            "operation": candidate["operation"], "categories": candidate["categories"],
            "frontier_candidates": candidate["frontiers"], "graph": graph,
            "source_refs": legacy_source_refs, "source_excerpts": legacy_excerpts,
            "unavailable_source": legacy_unavailable, "audit_evidence": audit_evidence,
            "coverage": {"operation_sites_identified": report["coverage"].get("operation_sites_identified"),
                         "report_truncated": report["truncated"],
                         "evidence_note": "Full report coverage is retained at result level; packet coverage compacts repeated counts. Frontiers are graph candidates, not proof of external reachability; audit alternatives are possible/unresolved evidence."},
        }
        if provenance_summary is not None:
            legacy_packet["argument_provenance"] = provenance_summary
        if control_flow_summary is not None:
            legacy_packet["control_flow"] = control_flow_summary
        legacy_packet = normalize_model_paths(legacy_packet, source_root, manifest)
        original_expanded_bytes = len(canonical_json(legacy_packet))

        expanded_packet = {
            "candidate_id": candidate["id"], "question": candidate["question"],
            "operation": candidate["operation"], "categories": candidate["categories"],
            "frontier_candidates": candidate["frontiers"], "graph": graph,
            "source_refs": source_refs, "source_excerpts": excerpts,
            "unavailable_source": unavailable, "audit_evidence": audit_evidence,
            "coverage": {"operation_sites_identified": report["coverage"].get("operation_sites_identified"),
                         "report_truncated": report["truncated"],
                         "evidence_note": "Full report coverage is retained at result level; packet coverage compacts repeated counts. Frontiers are graph candidates, not proof of external reachability, and graph connectivity is structural evidence, not proof of a runtime-feasible path; audit alternatives are possible/unresolved evidence."},
        }
        if provenance_summary is not None:
            expanded_packet["argument_provenance"] = provenance_summary
        if control_flow_summary is not None:
            expanded_packet["control_flow"] = control_flow_summary
        expanded_packet = normalize_model_paths(expanded_packet, source_root, manifest)
        expanded_state = {"candidate": candidate, "compact_graph": compact_graph,
                          "source_items": source_items, "unavailable_source": unavailable,
                          "audit_evidence": audit_evidence, "provenance_summary": provenance_summary,
                          "control_flow_summary": control_flow_summary, "report_truncated": report["truncated"],
                          "original_expanded_bytes": original_expanded_bytes,
                          "source_root": source_root, "manifest": manifest,
                          "strategy": "expanded", "steps": ["full_expanded_evidence"]}
        expanded_bytes = _refresh_reduction_report(expanded_packet, expanded_state)

        packet, packet_bytes, minimum_bytes = expanded_packet, expanded_bytes, None
        if candidate["id"] not in selected_candidate_ids:
            status, reason = "deferred_max_candidates", "outside deterministic candidate-ID selection"
        elif expanded_bytes <= max_packet_bytes:
            status, reason = "emitted", None
        else:
            packet, packet_bytes, fits = _progressively_reduce_packet(
                candidate, report, original_expanded_bytes, compact_graph, source_items, unavailable,
                audit_evidence, provenance_summary, provenance["nodes"] if provenance else {},
                control_flow_summary, nodes, max_packet_bytes, source_root, manifest)
            if fits:
                minimum_bytes = None
                status, reason = "emitted", None
            else:
                minimum_bytes = packet_bytes
                status, reason = "deferred_over_budget", "minimum investigation packet exceeds byte limit"

        inventory_item = {key: value for key, value in candidate.items()
                          if key not in ("audit_site", "audit_functions", "reverse")}
        site_provenance = [row for row in (provenance["sites"].values() if provenance else [])
                           if provenance_site_matches(row, candidate["operation"], {candidate["site_id"]})]
        summarized_sites = (provenance_summary or {}).get("sites", [])
        recovery_refs = {"provenance_site_refs": [row["id"] for row in site_provenance],
                         "provenance_value_refs": sorted({node["id"] for summary in summarized_sites for node in summary["origins"]}),
                         "provenance_edge_refs": sorted({edge["id"] for summary in summarized_sites for edge in summary["edges"]}),
                         "provenance_call_refs": sorted({call["id"] for summary in summarized_sites for call in summary["calls"]})}
        control_flow_refs = control_flow_summary or {"rows": [], "truncated": {}}
        packet_graph = packet.get("graph", {})
        packet_edge_refs = set(packet_graph.get("edge_refs", []))
        for segment in packet_graph.get("segments", []):
            packet_edge_refs.update(segment.get("edge_refs", []))
        packet_node_refs = set(packet_graph.get("node_refs", []))
        deferred_nodes = candidate["node_ids"] if status != "emitted" else [
            node_id for node_id in candidate["node_ids"] if node_id not in packet_node_refs]
        deferred_edges = candidate["edge_ids"] if status != "emitted" else [
            edge_id for edge_id in candidate["edge_ids"] if edge_id not in packet_edge_refs]
        deferred_sources = source_refs if status != "emitted" else [
            ref for ref in source_refs if ref["id"] not in {item["id"] for item in packet.get("source_refs", [])}]
        compact_graph_catalog = {"summary": compact_graph["summary"],
                                 "direct_edges": compact_graph["direct_edges"],
                                 "segments": compact_graph["segments"]}
        inventory_item.update({
            "packet_status": status, "audit_evidence": audit_evidence, **recovery_refs,
            "control_flow_row_refs": [{"id": row["id"], "status": row["status"]}
                                       for row in control_flow_refs["rows"]],
            "control_flow_truncated": control_flow_refs["truncated"],
            "graph_status": graph_status, "unavailable_source": unavailable,
            "source_refs": source_refs,
            "deferred_unique_node_refs": deferred_nodes,
            "deferred_unique_edge_refs": deferred_edges,
            "deferred_source_refs": deferred_sources,
            "expanded_packet_bytes": original_expanded_bytes,
            "final_packet_bytes": packet_bytes if status == "emitted" else None,
            "minimum_packet_bytes": minimum_bytes,
            "reduction_strategy": packet.get("reduction_report", {}).get("strategy"),
        })
        if candidate["id"] in selected_candidate_ids:
            inventory_item["compact_graph_catalog"] = compact_graph_catalog
        inventory.append(inventory_item)
        if status == "emitted":
            packets.append(packet)
        else:
            deferred_item = {
                "candidate_id": candidate["id"], "reason": reason, "packet_status": status,
                "bytes_if_emitted": original_expanded_bytes, "unique_node_refs": candidate["node_ids"],
                "unique_edge_refs": candidate["edge_ids"], "graph_status": graph_status,
                "source_refs": source_refs, **recovery_refs,
                "unavailable_source": unavailable,
                "control_flow_row_refs": [{"id": row["id"], "status": row["status"]}
                                           for row in control_flow_refs["rows"]],
                "control_flow_truncated": control_flow_refs["truncated"],
            }
            if minimum_bytes is not None:
                deferred_item["minimum_packet_bytes"] = minimum_bytes
                deferred_item["reduction_report"] = packet.get("reduction_report")
            deferred.append(deferred_item)

    joined_site_ids = set(audit_sites) & set(sites)
    candidate_ids = {candidate["site_id"] for candidate in candidates}
    joined_function_keys = {
        (row["package"], row["function"])
        for candidate in candidates for row in candidate["audit_functions"]
    }
    report_fingerprint = sha256(canonical_json(report))
    manifest_fingerprint = (program_model or {}).get("source_manifest_sha256")
    if manifest is not None:
        manifest_fingerprint = manifest_fingerprint or sha256(canonical_json(manifest))
    revision = (program_model or {}).get("revision", {}).get("revision")
    audit_coverage = {
        "input_site_rows": audit_site_total,
        "input_function_rows": audit_function_total,
        "unique_site_records_joined_to_report": len(joined_site_ids),
        "site_ids_outside_report": sorted(set(audit_sites) - set(sites)),
        "site_ids_outside_report_count": len(set(audit_sites) - set(sites)),
        "in_report_sites_not_candidates": len(set(sites) - candidate_ids),
        "unique_function_records_joined_to_candidates": len(joined_function_keys),
        "function_records_not_joined": audit_function_total - len(joined_function_keys),
        "repeated_candidate_function_joins": sum(len(candidate["audit_functions"]) for candidate in candidates),
    }
    node_saved = raw_node_refs - len(unique_node_ids)
    edge_saved = raw_edge_refs - len(unique_edge_ids)
    result = {
        "schema_version": 1,
        "provenance": {"report_schema_version": report["schema_version"], "report_sha256": report_fingerprint,
                       "source_manifest_sha256": manifest_fingerprint, "source_revision": revision},
        "counts": {"sites": len(sites), "nodes": len(nodes), "edges": len(edges),
                   "candidates": len(candidates), "packets_emitted": len(packets),
                   "packets_deferred": len(deferred)},
        "audit_coverage": audit_coverage,
        "investigation_evidence": {**ledger,
                                   "packet_builder_source_reads": "separate; excerpts and unavailable-source markers are in packets",
                                   "external_activity_capture": "not automatic; only supplied events are represented"},
        "coverage": {"report": report["coverage"], "report_truncated": report["truncated"]},
        "graph_reduction": {
            "raw_site_node_refs": raw_node_refs, "unique_node_refs": len(unique_node_ids),
            "node_refs_saved": node_saved, "node_ref_reduction_ratio": node_saved / raw_node_refs if raw_node_refs else 0,
            "raw_site_edge_refs": raw_edge_refs, "unique_edge_refs": len(unique_edge_ids),
            "edge_refs_saved": edge_saved, "edge_ref_reduction_ratio": edge_saved / raw_edge_refs if raw_edge_refs else 0,
        },
        "candidates": inventory, "node_catalog": node_catalog,
        "relationship_catalog": relationship_catalog, "packets": packets, "deferred": deferred,
        "limits": {"max_candidates": max_candidates, "max_packet_bytes": max_packet_bytes,
                   "selection_policy": "first max_candidates after deterministic candidate-ID sort; packet size does not affect selection",
                   "metric": "UTF-8 bytes of canonical compact JSON packet (sorted keys, no trailing newline)"},
        "token_estimate": {"method": "canonical UTF-8 bytes divided by four; heuristic, not a provider tokenizer",
                           "packets": [{"candidate_id": packet["candidate_id"],
                                        "bytes": len(canonical_json(packet)),
                                        "tokens_heuristic": (len(canonical_json(packet)) + 3) // 4}
                                       for packet in packets]},
    }
    if argument_provenance is not None:
        result["argument_provenance"] = argument_provenance
    if control_flow is not None:
        result["control_flow"] = control_flow["projection"]
    result = normalize_model_paths(result, source_root, manifest)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("effects")
    parser.add_argument("--audit")
    parser.add_argument("--source-root")
    parser.add_argument("--manifest")
    parser.add_argument("--program-model")
    parser.add_argument("--provenance", help="Go argument-provenance JSON")
    parser.add_argument("--control-flow", help="Go control-flow projection JSON")
    parser.add_argument("--events", help="validated external investigation event ledger (JSONL)")
    parser.add_argument("--out", required=True)
    parser.add_argument("--max-candidates", type=int, default=100)
    parser.add_argument("--max-packet-bytes", type=int, default=48000)
    args = parser.parse_args()
    audit = []
    if args.audit:
        with open(args.audit, encoding="utf-8") as stream:
            for line_number, line in enumerate(stream, 1):
                try:
                    audit.append(json.loads(line))
                except json.JSONDecodeError as error:
                    raise SystemExit(f"audit line {line_number}: {error}") from error
    events = None
    if args.events:
        events = []
        with open(args.events, encoding="utf-8") as stream:
            for line_number, line in enumerate(stream, 1):
                try:
                    events.append(json.loads(line))
                except json.JSONDecodeError as error:
                    raise SystemExit(f"event line {line_number}: {error}") from error
    result = build(
        load_json(args.effects), audit, args.max_candidates, args.max_packet_bytes,
        args.source_root, load_json(args.manifest) if args.manifest else None,
        load_json(args.program_model) if args.program_model else None,
        load_json(args.provenance) if args.provenance else None,
        load_json(args.control_flow) if args.control_flow else None,
        events,
    )
    output_dir = pathlib.Path(args.out)
    output_dir.mkdir(parents=True, exist_ok=True)
    (output_dir / "investigation.json").write_text(json.dumps(result, sort_keys=True, indent=2) + "\n")
    with (output_dir / "investigations.md").open("w", encoding="utf-8") as stream:
        for packet in result["packets"]:
            stream.write(f"## {packet['candidate_id']}\n\n")
            stream.write(f"Question: {packet['question']}\n\n")
            stream.write(f"Site: `{packet['operation']['id']}`; categories: {', '.join(packet['categories'])}.\n\n")
            for excerpt in packet["source_excerpts"]:
                stream.write(f"Source `{excerpt['path']}` sha256 `{excerpt['sha256']}` lines "
                             f"{excerpt['start_line']}-{excerpt['end_line']}:\n\n")
                stream.write(f"```go\n{excerpt['text']}\n```\n\n")
    print(json.dumps(result["counts"], sort_keys=True))


if __name__ == "__main__":
    main()
