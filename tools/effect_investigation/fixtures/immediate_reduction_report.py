"""Frozen metadata oracle before per-candidate source-ID reuse.

Copied verbatim from investigate.py SHA-256:
d7e69168756d079dab5cfd0bf431e8167628dc5de16582172468bb911919293b
"""


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
    source_ref_ids_all = {item["ref"]["id"] for item in all_sources}
    source_excerpt_ids_all = {item["ref"]["id"] for item in all_sources}

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
            "deferred_excerpt_count": max(0, len(source_excerpt_ids_all) - len(included_excerpt_ids)),
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
