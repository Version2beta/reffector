"""Fail-closed validation boundary for optional isolated packet investigations."""
from __future__ import annotations

import hashlib
import json
import re
import subprocess
from pathlib import Path

MAX_REQUEST_BYTES = 48_000
MAX_RESPONSE_BYTES = 16_000
MAX_EVENT_STREAM_BYTES = 256_000
MAX_RETRIEVED_SOURCE_BYTES = 16_000
PI_PROVIDER = "openai-codex"
PI_MODEL = "gpt-6-luna"
PI_TIMEOUT_SECONDS = 120
SCHEMA_VERSION = 2
_ID_PATTERN = {
    "candidate": re.compile(r"^candidate:[0-9a-f]{64}$"),
    "site": re.compile(r"^site:[0-9a-f]{64}$"),
    "link": re.compile(r"^link:[0-9a-f]{64}$"),
    "node": re.compile(r"^node:[0-9a-f]{64}$"),
    "source": re.compile(r"^source:[0-9a-f]{64}$"),
}
_RESPONSE_KEYS = {"candidate_id", "site_id", "hypothesis", "claims", "alternatives",
                   "limitations", "verification_plan"}
_CLAIM_KEYS = {"kind", "evidence_id", "site_id", "api", "role"}
_CLAIM_KINDS = {"operation", "relationship", "source"}
_CLAIM_ROLES = {"supporting", "contrary", "context"}


class InvestigationError(ValueError):
    pass


def _source_reference_id(reference):
    material = {key: reference[key] for key in ("path", "sha256", "start_line", "end_line")}
    encoded = json.dumps(material, sort_keys=True, separators=(",", ":"),
                         ensure_ascii=False).encode("utf-8")
    return "source:" + hashlib.sha256(encoded).hexdigest()


def _has_id(kind, value):
    pattern = _ID_PATTERN[kind]
    return isinstance(value, str) and pattern.fullmatch(value) is not None


def retrieve_source_excerpt(repository_root, reference):
    """Resolve one packet source reference without escaping the repository root.

    The reference must identify the complete source file by SHA-256 and a bounded
    one-based line range. Symlinks are rejected, including symlinked parents.
    """
    required = {"id", "path", "sha256", "start_line", "end_line"}
    if not isinstance(reference, dict) or not required <= set(reference):
        raise InvestigationError("source reference is incomplete")
    if not _has_id("source", reference["id"]) or reference["id"] != _source_reference_id(reference):
        raise InvestigationError("source reference ID does not match canonical path, digest, and range")
    if not isinstance(reference["sha256"], str) or not re.fullmatch(r"[0-9a-f]{64}", reference["sha256"]):
        raise InvestigationError("source reference digest is invalid")
    rel = reference["path"]
    if not isinstance(rel, str) or not rel or Path(rel).is_absolute():
        raise InvestigationError("source path must be repository-relative")
    parts = Path(rel).parts
    if any(part in ("..", "") for part in parts):
        raise InvestigationError("source path escapes repository")
    root = Path(repository_root).resolve(strict=True)
    path = root.joinpath(*parts)
    try:
        resolved = path.resolve(strict=True)
        resolved.relative_to(root)
    except (OSError, ValueError) as error:
        raise InvestigationError("source path is missing or outside repository") from error
    current = root
    for part in parts:
        current = current / part
        if current.is_symlink():
            raise InvestigationError("symlinked source paths are not allowed")
    if not resolved.is_file():
        raise InvestigationError("source path is not a regular file")
    content = resolved.read_bytes()
    digest = hashlib.sha256(content).hexdigest()
    if digest != reference["sha256"]:
        raise InvestigationError("source file digest does not match reference")
    start, end = reference["start_line"], reference["end_line"]
    if type(start) is not int or type(end) is not int or start < 1 or end < start:
        raise InvestigationError("source line range is invalid")
    lines = content.decode("utf-8", errors="replace").splitlines()
    if end > len(lines):
        raise InvestigationError("source line range exceeds file")
    excerpt = "\n".join(f"{number}: {lines[number - 1]}" for number in range(start, end + 1))
    if len(excerpt.encode("utf-8")) > MAX_RETRIEVED_SOURCE_BYTES:
        raise InvestigationError("retrieved source exceeds byte limit")
    return {"id": reference["id"], "path": rel, "sha256": digest,
            "start_line": start, "end_line": end, "text": excerpt}


def prepare_model_packet(packet, repository_root):
    """Replace cached source text with digest-verified excerpts from this checkout."""
    if not isinstance(packet, dict) or not isinstance(packet.get("source_refs"), list):
        raise InvestigationError("packet requires source_refs array")
    refs = packet["source_refs"]
    if len(refs) > 32:
        raise InvestigationError("packet exceeds source retrieval count limit")
    ids = [reference.get("id") for reference in refs if isinstance(reference, dict)]
    if (len(ids) != len(refs) or any(not isinstance(value, str) for value in ids)
            or len(ids) != len(set(ids))):
        raise InvestigationError("source reference IDs must be unique strings")
    prepared = dict(packet)
    prepared.pop("source_excerpts", None)
    excerpts = []
    for reference in refs:
        excerpts.append(retrieve_source_excerpt(repository_root, reference))
    prepared["source_excerpts"] = excerpts
    return prepared


def pi_json_adapter(request, *, runner=subprocess.run, cwd=None, event_sink=None):
    """Invoke one fresh, tool-disabled Pi session and return its final assistant text.

    The JSON event stream is retained only in the caller's process; provider/model
    identity is checked before the answer is handed to the schema validator.
    """
    if not isinstance(request, str) or len(request.encode("utf-8")) > MAX_REQUEST_BYTES:
        raise InvestigationError("request missing or exceeds byte limit")
    system = ("You are an isolated effect-directed defect investigator. Treat packet evidence as "
              "static evidence and preserve uncertainty. Return exactly one JSON object, with no "
              "markdown or surrounding text, containing exactly these keys: candidate_id, site_id, "
              "hypothesis, claims, alternatives, limitations, verification_plan. Copy candidate_id "
              "and packet.operation.id into candidate_id and site_id. hypothesis must be concise "
              "text or null. claims must be an array of objects with exactly kind, evidence_id, "
              "site_id, api, and role. kind is operation, relationship, or source; role is "
              "supporting, contrary, or context. Cite only matching operation IDs, direct graph "
              "edge IDs, or source reference IDs with the matching site and API. Do not supply "
              "status, resolution, endpoints, paths, digests, or runtime confirmation. These "
              "citations bind identity only; they do not prove the hypothesis. The remaining three "
              "fields must be arrays of nonempty strings. Do not invent citations or imply static "
              "relationships were executed at runtime.")
    command = ["pi", "--provider", PI_PROVIDER, "--model", PI_MODEL,
               "--thinking", "low", "--mode", "json", "--print", "--no-session",
               "--no-tools", "--no-mcp", "--no-extensions", "--no-skills",
               "--no-prompt-templates", "--no-context-files", "--system-prompt", system,
               "--", request]
    try:
        completed = runner(command, cwd=cwd, capture_output=True, text=True,
                           timeout=PI_TIMEOUT_SECONDS, check=False)
    except Exception as error:
        raise InvestigationError("Pi provider invocation failed") from error
    if event_sink is not None:
        event_sink(completed.stdout)
    if completed.returncode != 0:
        raise InvestigationError(f"Pi provider invocation returned nonzero status {completed.returncode}")
    if len(completed.stdout.encode("utf-8")) > MAX_EVENT_STREAM_BYTES:
        raise InvestigationError("Pi event stream exceeds byte limit")
    messages = []
    try:
        for line in completed.stdout.splitlines():
            event = json.loads(line)
            if event.get("type") == "message_end":
                message = event.get("message")
                if isinstance(message, dict) and message.get("role") == "assistant":
                    messages.append(message)
    except (ValueError, AttributeError, TypeError) as error:
        raise InvestigationError("Pi returned malformed JSON event stream") from error
    if not messages:
        raise InvestigationError("Pi returned no final assistant message")
    message = messages[-1]
    if message.get("provider") != PI_PROVIDER or message.get("model") != PI_MODEL:
        raise InvestigationError("Pi response provider/model metadata mismatch")
    content = message.get("content")
    if not isinstance(content, list):
        raise InvestigationError("Pi assistant content is missing")
    text = "".join(block.get("text", "") for block in content
                   if isinstance(block, dict) and block.get("type") == "text")
    if not text or len(text.encode("utf-8")) > MAX_RESPONSE_BYTES:
        raise InvestigationError("Pi assistant response missing or exceeds byte limit")
    return text


def _operation_apis(operation):
    apis = set()
    for value in (operation.get("api"),):
        if value is not None:
            if not isinstance(value, str) or not value:
                raise InvestigationError("operation API is invalid")
            apis.add(value)
    values = operation.get("apis", [])
    if not isinstance(values, list) or any(not isinstance(value, str) or not value for value in values):
        raise InvestigationError("operation APIs must be nonempty strings")
    apis.update(values)
    operations = operation.get("operations", [])
    if not isinstance(operations, list):
        raise InvestigationError("operation alternatives must be an array")
    for item in operations:
        if not isinstance(item, dict):
            raise InvestigationError("operation alternative must be an object")
        value = item.get("api")
        if value is not None:
            if not isinstance(value, str) or not value:
                raise InvestigationError("operation alternative API is invalid")
            apis.add(value)
    return apis


def _packet_site(packet):
    operation = packet.get("operation") if isinstance(packet, dict) else None
    if not isinstance(operation, dict) or not _has_id("site", operation.get("id")):
        raise InvestigationError("packet requires canonical operation site ID")
    if packet.get("operation_ref", operation["id"]) != operation["id"]:
        raise InvestigationError("packet operation reference does not match canonical site")
    apis = _operation_apis(operation)
    if not apis:
        raise InvestigationError("packet operation has no canonical API identity")
    return operation, apis


def _relationship_index(packet):
    graph = packet.get("graph")
    if not isinstance(graph, dict):
        return {}
    store_ref = graph.get("evidence_store_ref")
    if (isinstance(store_ref, dict) and store_ref.get("candidate_id") is not None
            and store_ref["candidate_id"] != packet.get("candidate_id")):
        raise InvestigationError("packet graph belongs to a different candidate")
    refs = graph.get("edge_refs", [])
    edges = graph.get("edges", [])
    nodes = graph.get("nodes", [])
    if (not isinstance(refs, list) or any(not _has_id("link", value) for value in refs)
            or len(refs) != len(set(refs)) or not isinstance(edges, list)
            or not isinstance(nodes, list)):
        raise InvestigationError("packet graph relationship references are malformed")
    if any(not isinstance(node, dict) for node in nodes):
        raise InvestigationError("packet graph node references are malformed")
    node_values = [node.get("id") for node in nodes]
    if (any(not _has_id("node", value) for value in node_values)
            or len(node_values) != len(set(node_values))):
        raise InvestigationError("packet graph node references are malformed")
    node_ids = set(node_values)
    edge_index = {}
    for edge in edges:
        if not isinstance(edge, dict):
            raise InvestigationError("packet graph edge must be an object")
        edge_id = edge.get("id")
        caller, callee = edge.get("caller"), edge.get("callee")
        if (not _has_id("link", edge_id) or edge_id not in refs or edge_id in edge_index
                or not _has_id("node", caller) or not _has_id("node", callee)
                or caller not in node_ids or callee not in node_ids
                or not isinstance(edge.get("kind"), str) or not edge["kind"]
                or not isinstance(edge.get("resolution"), str) or not edge["resolution"]
                or type(edge.get("candidate")) is not bool
                or type(edge.get("synthetic")) is not bool):
            raise InvestigationError("packet graph edge identity or endpoints are malformed")
        edge_index[edge_id] = edge
    return edge_index


def _source_index(packet):
    refs = packet.get("source_refs", [])
    excerpts = packet.get("source_excerpts", [])
    if not isinstance(refs, list) or not isinstance(excerpts, list):
        raise InvestigationError("packet source references and excerpts must be arrays")
    ref_index = {}
    required = {"id", "path", "sha256", "start_line", "end_line"}
    for ref in refs:
        if (not isinstance(ref, dict) or not required <= set(ref)
                or not _has_id("source", ref.get("id"))
                or not isinstance(ref.get("path"), str) or not ref["path"]
                or not isinstance(ref.get("sha256"), str)
                or not re.fullmatch(r"[0-9a-f]{64}", ref["sha256"])
                or type(ref.get("start_line")) is not int or type(ref.get("end_line")) is not int
                or ref["start_line"] < 1 or ref["end_line"] < ref["start_line"]):
            raise InvestigationError("packet source reference is malformed")
        if ref["id"] != _source_reference_id(ref) or ref["id"] in ref_index:
            raise InvestigationError("packet source reference ID is not canonical and unique")
        ref_index[ref["id"]] = ref
    excerpt_index = {}
    for excerpt in excerpts:
        if (not isinstance(excerpt, dict) or not isinstance(excerpt.get("id"), str)
                or excerpt["id"] in excerpt_index):
            raise InvestigationError("packet source excerpt is malformed or duplicated")
        excerpt_index[excerpt["id"]] = excerpt
    bound = {}
    for source_id, ref in ref_index.items():
        excerpt = excerpt_index.get(source_id)
        if (not isinstance(excerpt, dict) or not isinstance(excerpt.get("text"), str)
                or any(excerpt.get(key) != ref.get(key)
                       for key in ("path", "sha256", "start_line", "end_line"))):
            continue
        bound[source_id] = (ref, excerpt)
    return bound


def _bind_static_claims(claims, packet):
    if not isinstance(claims, list) or not isinstance(packet, dict):
        raise InvestigationError("claims and packet must be structured")
    if len(claims) > 64:
        raise InvestigationError("claim count exceeds limit")
    operation, apis = _packet_site(packet)
    sources = _source_index(packet)
    relationships = _relationship_index(packet)
    verified = []
    seen = set()
    for claim in claims:
        if not isinstance(claim, dict) or set(claim) != _CLAIM_KEYS:
            raise InvestigationError("claim does not match typed evidence schema")
        kind, evidence_id = claim["kind"], claim["evidence_id"]
        site_id, api, role = claim["site_id"], claim["api"], claim["role"]
        if (not isinstance(kind, str) or kind not in _CLAIM_KINDS
                or not isinstance(role, str) or role not in _CLAIM_ROLES):
            raise InvestigationError("claim kind or role is invalid")
        if (not isinstance(site_id, str) or not isinstance(api, str)
                or site_id != operation["id"] or api not in apis):
            raise InvestigationError("claim site or API does not match canonical operation")
        if not isinstance(evidence_id, str):
            raise InvestigationError("claim evidence ID must be text")
        key = (kind, evidence_id, site_id, api, role)
        if key in seen:
            raise InvestigationError("duplicate typed evidence claim")
        seen.add(key)
        if kind == "operation":
            if evidence_id != operation["id"]:
                raise InvestigationError("operation claim does not cite the canonical site")
            evidence = {"id": operation["id"], "api": api}
        elif kind == "relationship":
            edge = relationships.get(evidence_id)
            if edge is None:
                raise InvestigationError("relationship claim does not resolve to a direct canonical edge")
            evidence = {"id": evidence_id, "caller_id": edge["caller"], "callee_id": edge["callee"],
                        "kind": edge["kind"], "resolution": edge["resolution"],
                        "candidate": edge["candidate"], "synthetic": edge["synthetic"],
                        "execution_established": False}
        else:
            source = sources.get(evidence_id)
            if source is None:
                raise InvestigationError("source claim does not resolve to a canonical excerpt")
            ref, excerpt = source
            evidence = {"id": evidence_id, "path": ref["path"], "sha256": ref["sha256"],
                        "start_line": ref["start_line"], "end_line": ref["end_line"],
                        "excerpt_sha256": hashlib.sha256(excerpt["text"].encode("utf-8")).hexdigest(),
                        "text": excerpt["text"]}
        verified.append({"kind": kind, "evidence_id": evidence_id, "site_id": site_id,
                         "api": api, "role": role, "verification_outcome": "identity-bound",
                         "evidence": evidence})
    return verified


def verify_static_claims(claims, packet, *, repository_root=None):
    """Bind typed claims to canonical packet records, without proving their meaning.

    Source claims require a repository root so the cited file digest and exact range
    are re-read. Relationship claims bind the canonical edge ID, endpoints, and
    recorded resolution; they do not imply runtime execution.
    """
    source_claimed = (isinstance(claims, list) and
                      any(isinstance(claim, dict) and claim.get("kind") == "source"
                          for claim in claims))
    if source_claimed:
        if repository_root is None:
            raise InvestigationError("source claims require repository_root for digest verification")
        packet = prepare_model_packet(packet, repository_root)
    return _bind_static_claims(claims, packet)


def _hypothesis_status(hypothesis, verified_claims):
    if hypothesis is None:
        return "unresolved"
    has_supporting_citation = any(claim["role"] == "supporting" for claim in verified_claims)
    return "plausible-unverified" if has_supporting_citation else "unsupported"


def _qualify_result(result, packet, *, repository_root=None, sources_prepared=False):
    if not isinstance(result, dict) or set(result) != _RESPONSE_KEYS:
        raise InvestigationError("response does not match investigation schema")
    operation, _ = _packet_site(packet)
    if not _has_id("candidate", packet.get("candidate_id")):
        raise InvestigationError("packet requires canonical candidate ID")
    if (result["candidate_id"] != packet["candidate_id"]
            or result["site_id"] != operation["id"]):
        raise InvestigationError("response IDs do not match packet")
    if result["hypothesis"] is not None and (
            not isinstance(result["hypothesis"], str) or not result["hypothesis"].strip()):
        raise InvestigationError("hypothesis must be nonempty text or null")
    if not isinstance(result["claims"], list):
        raise InvestigationError("claims must be an array")
    for field in ("alternatives", "limitations", "verification_plan"):
        if not isinstance(result[field], list) or any(not isinstance(item, str) or not item.strip()
                                                       for item in result[field]):
            raise InvestigationError(f"{field} must be an array of nonempty strings")
    if sources_prepared:
        verified = _bind_static_claims(result["claims"], packet)
    else:
        verified = verify_static_claims(result["claims"], packet, repository_root=repository_root)
    qualified = dict(result)
    qualified.update({"schema_version": SCHEMA_VERSION, "verified_claims": verified,
                      "hypothesis_status": _hypothesis_status(result["hypothesis"], verified),
                      "runtime_status": "not-confirmed"})
    return qualified


def _read_persisted_investigation(artifact_dir, *, repository_root=None):
    try:
        root = Path(artifact_dir)
        manifest = json.loads((root / "manifest.json").read_text(encoding="utf-8"))
    except (OSError, ValueError, TypeError) as error:
        raise InvestigationError("persisted invocation manifest is missing or malformed") from error
    if (not isinstance(manifest, dict) or manifest.get("schema_version") != SCHEMA_VERSION
            or manifest.get("outcome") != "success"):
        raise InvestigationError("aggregation accepts only successful version-2 invocation artifacts")
    try:
        request_text = (root / "request.json").read_text(encoding="utf-8")
        result_bytes = (root / "result.json").read_bytes()
    except (OSError, ValueError, TypeError) as error:
        raise InvestigationError("persisted investigation artifacts are incomplete or malformed") from error
    if hashlib.sha256(result_bytes).hexdigest() != manifest.get("result_sha256"):
        raise InvestigationError("persisted result hash does not match manifest")
    try:
        request_json = request_text[:-1] if request_text.endswith("\n") else request_text
        request = json.loads(request_json)
        result = json.loads(result_bytes)
    except (ValueError, TypeError) as error:
        raise InvestigationError("persisted investigation artifacts are incomplete or malformed") from error
    if ((root / "error.json").exists() or not isinstance(request, dict)
            or set(request) != {"candidate_id", "packet"} or not isinstance(request["packet"], dict)):
        raise InvestigationError("persisted request/result artifacts disagree")
    canonical_request = json.dumps(request, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    if request_json != canonical_request:
        raise InvestigationError("persisted request is not canonical JSON")
    request_hash = hashlib.sha256(canonical_request.encode("utf-8")).hexdigest()
    if manifest.get("request_sha256") != request_hash:
        raise InvestigationError("persisted request hash does not match manifest")
    packet = request["packet"]
    operation, _ = _packet_site(packet)
    if (request["candidate_id"] != packet.get("candidate_id")
            or manifest.get("candidate_id") != request["candidate_id"]
            or manifest.get("site_id") != operation["id"]):
        raise InvestigationError("persisted manifest IDs do not match request")
    if not isinstance(result, dict) or not _RESPONSE_KEYS <= set(result):
        raise InvestigationError("persisted result lacks raw investigation fields")
    raw_result = {key: result[key] for key in _RESPONSE_KEYS}
    qualified = _qualify_result(raw_result, packet, repository_root=repository_root)
    if qualified != result:
        raise InvestigationError("persisted verification differs from canonical evidence")
    return packet, result


def aggregate_findings(artifact_dirs, *, repository_root=None):
    """Group exact duplicate, evidence-bound hypotheses from successful run artifacts.

    This is evidence-equivalence deduplication, not a common-resource or common-cause
    inference. It requires the same canonical site, exact hypothesis text, and the
    same complete set of typed claims resolved against each persisted request.
    """
    if not isinstance(artifact_dirs, list):
        raise InvestigationError("artifact directories must be an array")
    unique = set()
    buckets = {}
    for artifact_dir in artifact_dirs:
        _, result = _read_persisted_investigation(
            artifact_dir, repository_root=repository_root)
        candidate_id = result["candidate_id"]
        if candidate_id in unique:
            raise InvestigationError("candidate IDs in aggregation must be unique")
        unique.add(candidate_id)
        if result["hypothesis_status"] != "plausible-unverified" or not result["verified_claims"]:
            continue
        claim_signature = sorted(json.dumps(claim, sort_keys=True, separators=(",", ":"),
                                          ensure_ascii=False)
                                for claim in result["verified_claims"])
        signature = json.dumps({"site_id": result["site_id"], "hypothesis": result["hypothesis"],
                                "claims": claim_signature}, sort_keys=True,
                               separators=(",", ":"), ensure_ascii=False)
        buckets.setdefault(signature, []).append(result)
    groups = []
    for signature, members in sorted(buckets.items()):
        if len(members) < 2:
            continue
        members.sort(key=lambda item: item["candidate_id"])
        signature_value = json.loads(signature)
        ids = [item["candidate_id"] for item in members]
        first_claims = members[0]["verified_claims"]
        groups.append({"group_id": "group:" + hashlib.sha256(signature.encode("utf-8")).hexdigest(),
                       "finding_ids": ids,
                       "findings": [{"candidate_id": item["candidate_id"],
                                     "hypothesis": item["hypothesis"],
                                     "hypothesis_status": item["hypothesis_status"],
                                     "verified_claims": item["verified_claims"]}
                                    for item in members],
                       "provisional": True,
                       "basis": {"kind": "identical-verified-claim-set",
                                 "site_id": signature_value["site_id"],
                                 "evidence_ids": sorted({claim["evidence_id"] for claim in first_claims})}})
    return groups


def _prepare_for_model(packet, repository_root):
    if not isinstance(packet, dict) or not _has_id("candidate", packet.get("candidate_id")):
        raise InvestigationError("packet requires canonical candidate ID")
    _packet_site(packet)
    refs = packet.get("source_refs")
    if not isinstance(refs, list):
        raise InvestigationError("packet requires source_refs array")
    if refs:
        if repository_root is None:
            raise InvestigationError("source references require repository_root")
        return prepare_model_packet(packet, repository_root)
    prepared = dict(packet)
    prepared["source_refs"] = []
    prepared["source_excerpts"] = []
    return prepared


def _invoke_prepared(packet, adapter):
    if not callable(adapter):
        raise InvestigationError("adapter must be callable")
    request = json.dumps({"candidate_id": packet["candidate_id"], "packet": packet},
                         sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    if len(request.encode("utf-8")) > MAX_REQUEST_BYTES:
        raise InvestigationError("request exceeds byte limit")
    try:
        raw = adapter(request)
    except Exception as error:
        detail = str(error) or type(error).__name__
        raise InvestigationError(f"adapter invocation failed: {detail}") from error
    if not isinstance(raw, str) or len(raw.encode("utf-8")) > MAX_RESPONSE_BYTES:
        raise InvestigationError("response missing or exceeds byte limit")
    try:
        result = json.loads(raw)
    except (ValueError, TypeError) as error:
        raise InvestigationError("response is not JSON") from error
    return _qualify_result(result, packet, sources_prepared=True)


def invoke_isolated(packet, adapter, *, repository_root=None):
    """Send one bounded, source-prepared packet and bind typed claims to evidence.

    A source-bearing packet requires its checkout root. Free-form hypotheses remain
    unverified; typed claim validation establishes identity, not semantic entailment.
    """
    prepared = _prepare_for_model(packet, repository_root)
    return _invoke_prepared(prepared, adapter)


def run_and_persist(packet, adapter, artifact_dir, *, invocation_setup=None,
                    provider_adapter=False, repository_root=None):
    """Persist one immutable invocation under a new, caller-selected directory.

    Existing directories are never reused. The manifest is written last, so artifact
    readers can accept only complete runs. Source references are re-read under the
    supplied repository root before invocation; accepted claims bind to that prepared packet.
    """
    prepared = _prepare_for_model(packet, repository_root)
    request = json.dumps({"candidate_id": prepared["candidate_id"], "packet": prepared},
                         sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    if len(request.encode("utf-8")) > MAX_REQUEST_BYTES:
        raise InvestigationError("request exceeds byte limit")
    root = Path(artifact_dir)
    root.parent.mkdir(parents=True, exist_ok=True)
    try:
        root.mkdir(exist_ok=False)
    except FileExistsError as error:
        raise InvestigationError("artifact directory already exists; choose a new invocation directory") from error

    raw_events = []

    def capturing_adapter(value):
        if provider_adapter:
            return adapter(value, event_sink=raw_events.append)
        return adapter(value)

    result = None
    error_content = None
    try:
        result = _invoke_prepared(prepared, capturing_adapter)
    except Exception as error:
        error_content = json.dumps({"error": str(error)}, sort_keys=True) + "\n"
        (root / "error.json").write_text(error_content, encoding="utf-8")
    request_content = request + "\n"
    (root / "request.json").write_text(request_content, encoding="utf-8")
    if result is not None:
        result_content = json.dumps(result, sort_keys=True, indent=2) + "\n"
        (root / "result.json").write_text(result_content, encoding="utf-8")
    if raw_events:
        (root / "provider-events.jsonl").write_text(raw_events[-1], encoding="utf-8")
    manifest = {"schema_version": SCHEMA_VERSION, "candidate_id": prepared["candidate_id"],
                "site_id": prepared["operation"]["id"],
                "outcome": "success" if result is not None else "error",
                "request_sha256": hashlib.sha256(request.encode("utf-8")).hexdigest(),
                "provider": PI_PROVIDER if provider_adapter else None,
                "model": PI_MODEL if provider_adapter else None,
                "invocation_setup": invocation_setup or {"fresh_session": True}}
    if result is not None:
        manifest["result_sha256"] = hashlib.sha256(result_content.encode("utf-8")).hexdigest()
    else:
        manifest["error_sha256"] = hashlib.sha256(error_content.encode("utf-8")).hexdigest()
    (root / "manifest.json").write_text(json.dumps(manifest, sort_keys=True, indent=2) + "\n",
                                         encoding="utf-8")
    if result is None:
        raise InvestigationError("investigation failed; failure artifacts persisted")
    return result
