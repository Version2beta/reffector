"""Deterministic, question-directed selection over canonical AST and evidence indexes."""
from __future__ import annotations

import re
from typing import Any

_TOKEN = re.compile(r"[A-Za-z][A-Za-z0-9_]{2,}")


def _terms(value: str) -> set[str]:
    return {item.lower() for item in _TOKEN.findall(value)}


def _text(value: Any) -> str:
    if isinstance(value, dict):
        return " ".join(_text(key) + " " + _text(child) for key, child in value.items())
    if isinstance(value, (list, tuple)):
        return " ".join(_text(child) for child in value)
    return str(value) if value is not None else ""


def select_context(
    question: dict[str, Any],
    target: dict[str, Any],
    evidence_index: dict[str, Any],
    source_index: dict[str, Any],
    budget_bytes: int,
) -> dict[str, Any]:
    """Select a deterministic byte-bounded view; canonical indexes are never modified.

    Every in-scope record is either selected or deferred. Records outside the
    target's package/file boundary are explicitly not inspected. Ranking is
    lexical relevance plus balanced structural coverage, never effect support.
    """
    if not isinstance(question, dict) or not isinstance(question.get("prompt"), str):
        raise ValueError("question must contain a prompt")
    if not isinstance(target, dict) or not isinstance(target.get("packages"), list):
        raise ValueError("target must declare package scope")
    if not isinstance(evidence_index, dict) or not isinstance(evidence_index.get("records"), list):
        raise ValueError("evidence index is missing records")
    if not isinstance(source_index, dict) or not isinstance(source_index.get("declarations"), list):
        raise ValueError("source index is missing declarations")
    if not isinstance(budget_bytes, int) or isinstance(budget_bytes, bool) or budget_bytes <= 0:
        raise ValueError("selection budget must be a positive byte count")

    packages = set(target["packages"])
    files = set(target.get("files", []))
    query = _terms(question["prompt"] + " " + str(question.get("id", "")))
    declarations = [row for row in source_index["declarations"] if isinstance(row, dict)]
    in_scope_declarations = [row for row in declarations if row.get("file") in files and row.get("package") in packages]
    declarations.sort(key=lambda row: (row.get("file", ""), row.get("line_start", 0), row.get("id", "")))
    declaration_rows = []
    for row in in_scope_declarations:
        text = _text(row)
        hits = len(query & _terms(text))
        declaration_rows.append({"ref": row.get("id"), "kind": row.get("kind"), "name": row.get("name"),
                                 "file": row.get("file"), "line_start": row.get("start_line"),
                                 "line_end": row.get("end_line"), "exported": row.get("exported"),
                                 "has_body": row.get("has_body"), "receiver_type": row.get("receiver_type"),
                                 "signature": row.get("signature"), "type_kind": row.get("type_kind"),
                                 "interface_methods": row.get("interface_methods", []),
                                 "relevance_hits": hits, "effect_status": "not_inferred_from_absence"})

    records = evidence_index["records"]
    eligible = []
    unsupported_refs: list[str] = []
    record_type_by_ref: dict[str, str] = {}
    not_inspected = []
    for row in records:
        if not isinstance(row, dict) or not isinstance(row.get("ref"), str):
            raise ValueError("evidence index contains an invalid reference")
        if row["ref"] in record_type_by_ref:
            raise ValueError(f"evidence index contains duplicate reference: {row['ref']}")
        record_kind = row.get("record_type")
        record_type_by_ref[row["ref"]] = record_kind if isinstance(record_kind, str) else "unsupported"
        file = row.get("file")
        package = row.get("package")
        data = row.get("data", {})
        in_scope = (file in files) or (package in packages)
        if row.get("record_type") == "call_edge":
            in_scope = in_scope or data.get("caller_package") in packages or data.get("callee_package") in packages
        if not in_scope:
            not_inspected.append(row["ref"])
            continue
        if row.get("record_type") not in {"effect_site", "audit_site", "call_edge", "unresolved_site"}:
            unsupported_refs.append(row["ref"])
            continue
        text = _text(row)
        hits = len(query & _terms(text))
        kind = row.get("record_type", "unknown")
        uncertain = bool(data.get("uncertain") or data.get("candidate") or data.get("possible_effects") or kind == "unresolved_site")
        # Keep the selector neutral: relevance leads, then broad row-type coverage,
        # then uncertainty/edge context so conflicting evidence is not hidden.
        score = (hits * 1000, 1 if kind in {"function_audit", "effect_site", "audit_site", "call_edge", "unresolved_site"} else 0,
                 1 if uncertain else 0)
        eligible.append((score, row["ref"], row, hits))
    conflict_groups: list[dict[str, Any]] = []
    by_identity: dict[str, list[tuple[Any, ...]]] = {}
    for item in eligible:
        row = item[2]
        data = row.get("data", {})
        identity = data.get("id")
        if not isinstance(identity, str):
            identity = next((alias for alias in row.get("aliases", []) if isinstance(alias, str)), None)
        if not isinstance(identity, str):
            continue
        signature = tuple(_text(data.get(field)) for field in ("effects", "possible_effects", "uncertain", "candidate_ids", "candidate_count"))
        by_identity.setdefault(identity, []).append((item[1], signature))
    for identity, items in sorted(by_identity.items()):
        signatures = {item[1] for item in items}
        if len(items) > 1 and len(signatures) > 1:
            refs = sorted(item[0] for item in items)
            conflict_groups.append({"identity": identity, "refs": refs})

    eligible.sort(key=lambda item: (-item[0][0], -item[0][1], -item[0][2], item[1]))
    # Reserve one in-scope representative per record type and for candidate/
    # uncertainty evidence before lexical ranking fills the remaining budget.
    seed_refs: set[str] = {ref for group in conflict_groups for ref in group["refs"]}
    for record_type in sorted({item[2].get("record_type") for item in eligible}):
        candidates = [item for item in eligible if item[2].get("record_type") == record_type]
        seed_refs.add(candidates[0][1])
    for label, predicate in (
        ("candidate", lambda data: bool(data.get("candidate") or data.get("candidate_ids") or data.get("candidate_count"))),
        ("uncertain", lambda data: bool(data.get("uncertain") or data.get("possible_effects") or data.get("uncertainty_reasons"))),
        ("unresolved", lambda data: bool(data.get("record_type") == "unresolved_site" or data.get("unresolved"))),
    ):
        match = next((item for item in eligible if predicate(item[2].get("data", {}))), None)
        if match is not None:
            seed_refs.add(match[1])
    eligible.sort(key=lambda item: (0 if item[1] in seed_refs else 1, -item[0][0], -item[0][1], -item[0][2], item[1]))

    selected: list[dict[str, Any]] = []
    deferred: list[str] = list(unsupported_refs)
    used = 0
    for _, ref, row, hits in eligible:
        # Exact serialized row size is an objective byte cap, not a token estimate.
        import json
        size = len(json.dumps(row, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8"))
        if used + size <= budget_bytes:
            selected.append({"ref": ref, "record_type": row.get("record_type"), "file": row.get("file"),
                             "source_ref": row.get("source_ref"), "relevance_hits": hits,
                             "uncertain_or_candidate": bool(row.get("data", {}).get("uncertain") or row.get("data", {}).get("candidate") or row.get("data", {}).get("possible_effects") or row.get("record_type") == "unresolved_site")})
            used += size
        else:
            deferred.append(ref)
    return {
        "schema_version": 1,
        "question_id": question.get("id"),
        "target": {"id": target.get("id"), "packages": sorted(packages), "files": sorted(files)},
        "selection_policy": "lexical relevance then balanced record-type/uncertainty coverage then stable reference; no support-based filtering",
        "budget_bytes": budget_bytes,
        "selected_bytes": used,
        "available_count": len(eligible) + len(unsupported_refs),
        "selected_count": len(selected),
        "deferred_count": len(deferred),
        "not_inspected_count": len(not_inspected),
        "selected": selected,
        "deferred_refs": deferred,
        "not_inspected_refs": sorted(not_inspected),
        "source_declarations": declaration_rows,
        "source_revision": evidence_index.get("revision"),
        "source_manifest_sha256": evidence_index.get("source_manifest_sha256"),
        "contradiction_groups_inspected": conflict_groups,
        "coverage": {"selected_fraction": len(selected) / (len(eligible) + len(unsupported_refs)) if eligible or unsupported_refs else 1.0,
                     "truncated": bool(deferred), "complete_index_scope": evidence_index.get("coverage", {}),
                     "selected_by_record_type": {kind: sum(item["record_type"] == kind for item in selected) for kind in sorted({item["record_type"] for item in selected})},
                     "deferred_by_record_type": {kind: sum(record_type_by_ref.get(ref, "unsupported") == kind for ref in deferred) for kind in sorted({record_type_by_ref.get(ref, "unsupported") for ref in deferred})}},
        "uncertainty": ["Lexical relevance is not semantic sufficiency.",
                        "Effect-free declarations remain in the source index; no recognized effect does not imply no behavior.",
                        "Reffector relationships are not a complete call graph or runtime trace.",
                        "Deferred and out-of-scope records were not inspected for question-specific meaning."],
    }


def retrieve_evidence_records(
    evidence_index: dict[str, Any], refs: list[str], expected_revision: dict[str, Any], budget_bytes: int,
) -> dict[str, Any]:
    """Read selected canonical records locally, with revision and byte limits enforced."""
    if not isinstance(evidence_index, dict) or not isinstance(evidence_index.get("records"), list):
        raise ValueError("evidence retrieval requires a canonical index")
    if evidence_index.get("revision") != expected_revision:
        raise ValueError("evidence revision does not match the pinned revision")
    if not isinstance(refs, list) or any(not isinstance(ref, str) or not ref for ref in refs):
        raise ValueError("evidence retrieval refs must be nonempty strings")
    if len(set(refs)) != len(refs):
        raise ValueError("evidence retrieval refs must be unique")
    if not isinstance(budget_bytes, int) or isinstance(budget_bytes, bool) or budget_bytes <= 0:
        raise ValueError("evidence retrieval budget must be a positive byte count")
    by_ref = {}
    for row in evidence_index["records"]:
        if not isinstance(row, dict) or not isinstance(row.get("ref"), str) or row["ref"] in by_ref:
            raise ValueError("canonical evidence index contains invalid or duplicate references")
        by_ref[row["ref"]] = row
    missing = [ref for ref in refs if ref not in by_ref]
    if missing:
        raise ValueError("unknown evidence reference(s): " + ", ".join(missing))
    import json
    retrieved = []
    used = 0
    for ref in refs:
        row = by_ref[ref]
        payload = {"ref": ref, "record_type": row.get("record_type"), "record": row.get("data")}
        size = len(json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8"))
        if used + size > budget_bytes:
            raise ValueError("evidence retrieval byte budget exceeded")
        used += size
        retrieved.append(payload)
    return {"revision": expected_revision, "budget_bytes": budget_bytes, "retrieved_bytes": used,
            "retrieved_refs": list(refs), "records": retrieved,
            "log": [{"ref": row["ref"], "record_type": row["record_type"], "bytes": len(json.dumps(row, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8"))} for row in retrieved]}
