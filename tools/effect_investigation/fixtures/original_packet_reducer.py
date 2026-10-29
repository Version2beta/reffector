"""Frozen reducer oracle, copied verbatim before optimization.

Original investigate.py SHA-256:
555e60da1092dc01a489711fc665ac09b12333fa9a287b51419ec436d1a90871
Only unchanged construction helpers are imported from the live implementation.
"""
from investigate import (
    normalize_model_paths, canonical_json, _reduction_report, _make_minimum_packet,
    _add_source_item, _graph_snapshot, _restore_graph, _add_compact_graph_item,
)


def _refresh_reduction_report(packet, state):
    normalized = normalize_model_paths(packet, state.get("source_root"), state.get("manifest"))
    packet.clear()
    packet.update(normalized)
    packet["reduction_report"] = _reduction_report(packet, state)
    report = packet["reduction_report"]
    for _ in range(8):
        size = len(canonical_json(packet))
        saved = report["original_expanded_bytes"] - size
        if report["final_serialized_bytes"] == size and report["net_bytes_saved"] == saved:
            return size
        report["final_serialized_bytes"] = size
        report["net_bytes_saved"] = saved
    return len(canonical_json(packet))



def _try_packet_addition(packet, state, max_packet_bytes, label, rollback):
    added_step = label not in state["steps"]
    if added_step:
        state["steps"].append(label)
    size = _refresh_reduction_report(packet, state)
    if size <= max_packet_bytes:
        return True
    if added_step:
        state["steps"].pop()
    rollback()
    _refresh_reduction_report(packet, state)
    return False



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


