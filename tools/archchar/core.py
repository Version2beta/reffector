"""Offline, resumable architecture characterization using Reffector evidence."""
from __future__ import annotations

import hashlib
import json
import math
import os
import pathlib
import re
import shutil
import subprocess
import time
from collections import Counter, defaultdict
from typing import Any

from .reduction import build_evidence_index, partition_groups, reduce_request, stable_id
from .token_budget import TokenBudgetError, run_token_counter

SCHEMA_VERSION = 1
OFFLINE_GO_ENV = {
    "GOENV": "off",
    "GOTOOLCHAIN": "local",
    "GOPROXY": "off",
    "GOSUMDB": "off",
    "GOFLAGS": "-mod=readonly",
    "GOWORK": "off",
}
GO_PASSTHROUGH_ENV = {
    "PATH", "HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "SYSTEMROOT", "WINDIR", "PATHEXT",
    "GOROOT", "GOPATH", "GOMODCACHE", "GOCACHE", "GOOS", "GOARCH", "GOARM", "GOAMD64",
    "GOEXPERIMENT", "CGO_ENABLED", "CC", "CXX", "AR", "PKG_CONFIG",
}
CATEGORIES = {"established", "possible", "inferred", "unknown"}
DOMAINS = {"architecture", "relationship", "effect", "runtime", "concurrency"}
SENSITIVE_NAME = re.compile(r"(?i)(api[-_]?key|token|secret|password|authorization|credential|private[-_]?key|bearer)")
COMPONENT_PROMPT = """Interpret the supplied current source and deterministic program/evidence indexes; do not repeat mechanically derivable inventories. Return one JSON object with claims and optional need_sources (repo-relative files) and need_evidence (available evidence references). Each claim has id, statement, category (established|possible|inferred|unknown), domain (architecture|relationship|effect|runtime|concurrency), and evidence. Explain component responsibilities, exposed contracts, interfaces, invariants, state ownership, concurrency/lifecycle responsibilities, and cross-component boundaries. Source evidence is {kind: source, file, line, quote}; Reffector evidence is {kind: reffector, id, field?}. Source fragments can begin mid-file; cite original one-based source lines using line_start/line_end, not fragment-relative lines. Cite exact source locations and stable Reffector IDs. Read source for pure transformations, business rules, and control flow; no recognized effect does not prove no behavior. Keep candidate and possible evidence possible-only. Static call reachability does not establish runtime execution, completion, or success. Do not infer a complete call graph from effect-site reverse evidence. State coverage, omissions, and unresolved behavior. Request additional selected source or indexed evidence when needed; do not guess."""
AGGREGATE_PROMPT = """Synthesize supplied child claims into concise component/package/namespace characterizations; do not concatenate descriptions. Return claims with id, statement, category, domain, evidence, and child_claim_ids. Preserve responsibilities, interfaces, dependencies, cross-unit relationships, effects, concurrency, state/resource obligations, conflicts, and uncertainty. Do not strengthen a claim beyond cited children. Candidate or possible-only evidence must not become established. Static call reachability is not runtime execution. Do not invent source facts or imply the effect-site reverse graph is complete."""
VERIFY_PROMPT = """First rely on deterministic source, structure, and Reffector checks. Assess only semantic assertions that those checks cannot decide, using the cited source excerpts and indexed records. Return verdicts {claim_id, verdict: supported|unsupported|uncertain, reason}. Do not certify claims that contradict established static evidence. Candidate edges and possible_effects remain possible-only; static evidence cannot establish runtime execution. A source citation proves only the cited text, not a broader guarantee."""
BENCHMARK_PROMPT = """Characterize architecture from the supplied source excerpts and deterministic source/evidence indexes. Interpret responsibilities, contracts, interfaces, invariants, cross-component relationships, effects, concurrency/lifecycle obligations, and uncertainty; do not repeat deterministic inventories. Pure transformations and control flow may be important even without recognized effects. Return claims with id, statement, category (established|possible|inferred|unknown), domain (architecture|relationship|effect|runtime|concurrency), and evidence. Cite exact source lines and stable Reffector IDs. Keep candidate/possible evidence possible-only; static reachability does not prove runtime execution. State coverage, omissions, and unresolved behavior. This request may contain focused source excerpts rather than complete files; available_source_inventory and source_retrieval_catalog identify hash-bound material that can be requested by need_sources or need_source_refs. Treatment may additionally request stable canonical evidence IDs in need_evidence; source-only must not request Reffector evidence. Retrieval is bounded and each response is analyzed in a fresh context. Do not assume omitted source or graph edges are absent."""


class ArchcharError(Exception):
    """Raised for invalid inputs or unsafe output locations."""


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode("utf-8")


def write_json(path: pathlib.Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True, allow_nan=False) + "\n", encoding="utf-8")
    temporary.replace(path)


def read_json(path: pathlib.Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def is_within(path: pathlib.Path, root: pathlib.Path) -> bool:
    try:
        path.resolve().relative_to(root.resolve())
        return True
    except ValueError:
        return False


def relative_source_path(repo: pathlib.Path, raw_path: str | None) -> str | None:
    if not raw_path:
        return None
    path = pathlib.Path(raw_path)
    if not path.is_absolute():
        path = repo / path
    try:
        return path.resolve(strict=False).relative_to(repo.resolve()).as_posix()
    except ValueError:
        return None


def read_repo_file(repo: pathlib.Path, relative_path: str) -> bytes:
    if not isinstance(relative_path, str):
        raise ArchcharError("source path must be a string")
    pure = pathlib.PurePosixPath(relative_path.replace(chr(92), "/"))
    if pure.is_absolute() or ".." in pure.parts:
        raise ArchcharError("source path is not repository-relative")
    actual = (repo / pure.as_posix()).resolve(strict=True)
    if not is_within(actual, repo):
        raise ArchcharError("source path resolves outside the repository")
    return actual.read_bytes()


def safe_repo_output(repo: pathlib.Path, output: pathlib.Path) -> pathlib.Path:
    resolved = output.resolve()
    if is_within(resolved, repo):
        raise ArchcharError("output directory must be outside the analyzed repository")
    return resolved


def safe_command_display(argv: list[str]) -> list[str]:
    """Redact common secret-bearing arguments before saving command metadata."""
    result: list[str] = []
    redact_next = False
    for arg in argv:
        if redact_next:
            result.append("<redacted>")
            redact_next = False
            continue
        if SENSITIVE_NAME.search(arg):
            if "=" in arg:
                result.append(arg.split("=", 1)[0] + "=<redacted>")
            elif arg.startswith("-"):
                result.append(arg)
                redact_next = True
            else:
                result.append("<redacted>")
            continue
        result.append(arg)
    return result


def model_command_display(command_json: str | None) -> list[str] | None:
    if not command_json:
        return None
    try:
        argv = json.loads(command_json)
    except ValueError:
        return None
    if not isinstance(argv, list) or not argv or not all(isinstance(arg, str) for arg in argv):
        return None
    return safe_command_display(argv)


def redact_diagnostics(text: str) -> str:
    text = re.sub(r"(?i)(bearer\s+)[A-Za-z0-9._~+/-]+=*", r"\1<redacted>", text)
    text = re.sub(r"\bsk-[A-Za-z0-9_-]{16,}\b", "<redacted>", text)
    text = re.sub(r"(?i)((?:api[-_]?key|token|secret|password|authorization|credential|private[-_]?key)\s*[=:]\s*)[^\s,;]+", r"\1<redacted>", text)
    return text


def run_process(argv: list[str], cwd: pathlib.Path, env: dict[str, str], timeout: int) -> dict[str, Any]:
    start = time.monotonic()
    try:
        process = subprocess.run(argv, cwd=cwd, env=env, capture_output=True, timeout=timeout, check=False)
        return {
            "returncode": process.returncode,
            "stdout": process.stdout,
            "stderr": process.stderr,
            "elapsed_seconds": time.monotonic() - start,
            "timed_out": False,
        }
    except subprocess.TimeoutExpired as error:
        return {
            "returncode": -1,
            "stdout": error.stdout or b"",
            "stderr": error.stderr or b"",
            "elapsed_seconds": time.monotonic() - start,
            "timed_out": True,
        }
    except OSError as error:
        return {
            "returncode": -1,
            "stdout": b"",
            "stderr": str(error).encode("utf-8", errors="replace"),
            "elapsed_seconds": time.monotonic() - start,
            "timed_out": False,
        }


def git_revision(repo: pathlib.Path) -> dict[str, Any]:
    env = {key: os.environ[key] for key in ("PATH", "HOME", "USERPROFILE", "SYSTEMROOT") if key in os.environ}
    env.update({"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull, "GIT_OPTIONAL_LOCKS": "0", "GIT_TERMINAL_PROMPT": "0"})
    git = ["git", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false"]
    revision = subprocess.run([*git, "rev-parse", "HEAD"], cwd=repo, env=env, capture_output=True, text=True, check=False)
    status = subprocess.run([*git, "status", "--porcelain", "--untracked-files=normal"], cwd=repo, env=env, capture_output=True, text=True, check=False)
    return {
        "revision": revision.stdout.strip() if revision.returncode == 0 else None,
        "dirty": bool(status.stdout.strip()) if status.returncode == 0 else None,
        "git_available": revision.returncode == 0 and status.returncode == 0,
    }


def parse_json_stream(text: str) -> list[dict[str, Any]]:
    decoder = json.JSONDecoder()
    objects: list[dict[str, Any]] = []
    offset = 0
    while offset < len(text):
        while offset < len(text) and text[offset].isspace():
            offset += 1
        if offset == len(text):
            break
        value, end = decoder.raw_decode(text, offset)
        if not isinstance(value, dict):
            raise ValueError("go list emitted a non-object JSON value")
        objects.append(value)
        offset = end
    return objects


def parse_jsonl(text: str) -> tuple[list[dict[str, Any]], list[str]]:
    records: list[dict[str, Any]] = []
    errors: list[str] = []
    for number, line in enumerate(text.splitlines(), start=1):
        if not line.strip():
            continue
        try:
            record = json.loads(line)
            if not isinstance(record, dict):
                raise ValueError("record is not a JSON object")
            records.append(record)
        except ValueError as error:
            errors.append(f"line {number}: {error}")
    return records, errors


def truncation_present(value: Any) -> bool:
    if isinstance(value, dict):
        return any(truncation_present(child) for child in value.values())
    if isinstance(value, list):
        return any(truncation_present(child) for child in value)
    return bool(value)


def symbol_package(value: Any) -> str | None:
    if isinstance(value, dict):
        package = value.get("package")
        return package if isinstance(package, str) else None
    return None


def command_phase_metadata(argv: list[str], result: dict[str, Any], stdout_path: str, stderr_path: str) -> dict[str, Any]:
    return {
        "argv": safe_command_display(argv),
        "returncode": result["returncode"],
        "elapsed_seconds": result["elapsed_seconds"],
        "timed_out": result["timed_out"],
        "stdout_path": stdout_path,
        "stdout_bytes": len(result["stdout"]),
        "stderr_path": stderr_path,
        "stderr_bytes": len(result["stderr"]),
        "status": "complete" if result["returncode"] == 0 else "failed",
    }


def validate_effect_report(value: Any) -> list[str]:
    errors: list[str] = []
    if not isinstance(value, dict):
        return ["effects JSON must be an object"]
    if value.get("schema_version") != 1:
        errors.append("effects schema_version must be 1")
    for key in ("groups", "sites", "unresolved_sites", "reverse", "nodes", "edges", "coverage", "inventory"):
        if key not in value:
            errors.append(f"effects JSON is missing {key}")
    for key in ("groups", "sites", "unresolved_sites", "reverse", "nodes", "edges", "inventory"):
        if key in value and not isinstance(value[key], list):
            errors.append(f"effects field {key} must be an array")
    if not isinstance(value.get("coverage"), dict):
        errors.append("effects coverage must be an object")
    return errors


def parse_location_files(repo: pathlib.Path, record: dict[str, Any]) -> list[str]:
    locations: list[Any] = []
    if isinstance(record.get("locations"), list):
        locations.extend(record["locations"])
    if isinstance(record.get("location"), dict):
        locations.append(record["location"])
    result: list[str] = []
    for location in locations:
        if isinstance(location, dict):
            path = relative_source_path(repo, location.get("file"))
            if path and path not in result:
                result.append(path)
    return result


def effect_edge_record(repo: pathlib.Path, edge: dict[str, Any], node_packages: dict[str, str]) -> dict[str, Any]:
    source = edge.get("source") if isinstance(edge.get("source"), dict) else {}
    location = source.get("location") if isinstance(source.get("location"), dict) else {}
    source_id = source.get("id")
    caller = source.get("caller", {})
    callee = source.get("callee", {})
    caller_id = edge.get("caller")
    callee_id = edge.get("callee")
    caller_package = symbol_package(caller) or node_packages.get(caller_id)
    callee_package = symbol_package(callee) or node_packages.get(callee_id)
    return {
        "id": edge.get("id"),
        "source_edge_id": source_id,
        "caller_node": caller_id,
        "callee_node": callee_id,
        "caller": caller,
        "callee": callee,
        "caller_package": caller_package,
        "callee_package": callee_package,
        "location": location,
        "file": relative_source_path(repo, location.get("file")),
        "kind": source.get("kind"),
        "resolution": source.get("resolution", edge.get("resolution")),
        "candidate": bool(edge.get("candidate", False)),
        "synthetic": bool(edge.get("synthetic", False)),
        "scope": "effect-site reverse evidence; not a complete call graph",
    }


def make_environment(output: pathlib.Path) -> tuple[dict[str, str], dict[str, str]]:
    env = {key: os.environ[key] for key in GO_PASSTHROUGH_ENV if key in os.environ}
    env.update(OFFLINE_GO_ENV)
    temp_dir = output / "tmp"
    temp_dir.mkdir(parents=True, exist_ok=True)
    env["TMPDIR"] = str(temp_dir)
    env["TEMP"] = str(temp_dir)
    env["TMP"] = str(temp_dir)
    safe = dict(OFFLINE_GO_ENV)
    safe.update({"TMPDIR": str(temp_dir), "TEMP": str(temp_dir), "TMP": str(temp_dir)})
    return env, safe


def token_policy_from_args(args: Any, required: bool = False) -> dict[str, int] | None:
    values = {
        "context_window_tokens": getattr(args, "context_window_tokens", None),
        "output_reserve_tokens": getattr(args, "output_reserve_tokens", None),
        "safety_margin_tokens": getattr(args, "safety_margin_tokens", None),
    }
    provided = [value is not None for value in values.values()]
    if not any(provided):
        if required:
            raise ArchcharError("token context, output reserve, and safety margin must be configured before model generation")
        return None
    if not all(provided) or any(not isinstance(value, int) or isinstance(value, bool) or value <= 0 for value in values.values()):
        raise ArchcharError("token budget limits must all be positive integers")
    return values


def extract_go_structure(
    repo: pathlib.Path,
    package_by_id: dict[str, dict[str, Any]],
    source_manifest: dict[str, dict[str, Any]],
    env: dict[str, str],
) -> dict[str, Any]:
    files = []
    for package_id, package in sorted(package_by_id.items()):
        for path in package["files"]:
            data = read_repo_file(repo, path)
            if digest(data) != source_manifest.get(path, {}).get("sha256"):
                raise ArchcharError(f"source changed before structural indexing: {path}")
            try:
                text = data.decode("utf-8")
            except UnicodeError as error:
                raise ArchcharError(f"selected Go source is not valid UTF-8: {path}") from error
            files.append({"package": package_id, "path": path, "text": text})
    if not files:
        raise ArchcharError("no selected Go source files are available for structural indexing")
    executable = shutil.which("go", path=env.get("PATH"))
    if not executable:
        raise ArchcharError("Go toolchain is unavailable for structural indexing")
    helper = pathlib.Path(__file__).resolve().parent / "go-structure"
    try:
        process = subprocess.run(
            [executable, "run", "."], cwd=helper, env=env,
            input=canonical_json({"files": files}).decode("utf-8"),
            text=True, capture_output=True, timeout=300, check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise ArchcharError(f"Go structural indexer did not complete: {type(error).__name__}") from error
    if process.returncode != 0:
        raise ArchcharError("Go structural indexer failed: " + redact_diagnostics(process.stderr[:1000]))
    try:
        result = json.loads(process.stdout)
    except ValueError as error:
        raise ArchcharError("Go structural indexer returned invalid JSON") from error
    if not isinstance(result, dict):
        raise ArchcharError("Go structural indexer returned a non-object JSON value")
    expected = {item["path"] for item in files}
    actual = {item.get("path") for item in result.get("files", []) if isinstance(item, dict)}
    if result.get("status") != "complete" or actual != expected or result.get("errors"):
        raise ArchcharError("Go structural indexer did not completely cover the selected source files")
    return result


def build_program_model(
    repo: pathlib.Path,
    package_by_id: dict[str, dict[str, Any]],
    structure: dict[str, Any],
    package_imports: list[dict[str, str]],
    edges: list[dict[str, Any]],
    revision: dict[str, Any],
    source_manifest_sha256: str,
) -> dict[str, Any]:
    files = structure.get("files", [])
    declarations = structure.get("declarations", [])
    functions_by_package: dict[str, list[dict[str, Any]]] = defaultdict(list)
    declarations_by_package: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for declaration in declarations:
        package_id = declaration.get("package")
        if package_id:
            declarations_by_package[package_id].append(declaration)
            if declaration.get("kind") == "function":
                functions_by_package[package_id].append(declaration)
    packages = []
    for package_id, package in sorted(package_by_id.items()):
        local_files = [item for item in files if item.get("package") == package_id]
        local_declarations = declarations_by_package.get(package_id, [])
        packages.append({
            "id": package_id,
            "name": local_files[0].get("package_name") if local_files else None,
            "files": list(package.get("files", [])),
            "imports": list(package.get("imports", [])),
            "exported_entry_points": [
                {"name": item.get("name"), "symbol": item.get("symbol"), "file": item.get("file"), "line": item.get("start_line"), "signature": item.get("signature")}
                for item in local_declarations if item.get("kind") == "function" and item.get("exported")
            ],
            "analysis_coverage": {
                "source_files": len(local_files),
                "declarations": len(local_declarations),
                "functions_and_methods": len(functions_by_package.get(package_id, [])),
                "types": sum(item.get("kind") == "type" for item in local_declarations),
                "interfaces": sum(item.get("kind") == "type" and item.get("type_kind") == "interface" for item in local_declarations),
                "exported_declarations": sum(bool(item.get("exported")) for item in local_declarations),
            },
        })
    relationship_rows = []
    for link in package_imports:
        relationship_rows.append({**link, "kind": "package_import", "resolution": "established", "candidate": False})
    for edge in edges:
        if edge.get("caller_package") == edge.get("callee_package"):
            continue
        relationship_rows.append({
            "id": edge.get("id"), "caller_package": edge.get("caller_package"), "callee_package": edge.get("callee_package"),
            "caller": edge.get("caller"), "callee": edge.get("callee"), "file": edge.get("file"),
            "location": edge.get("location"), "kind": edge.get("kind"), "resolution": edge.get("resolution"),
            "candidate": edge.get("candidate", False), "relationship_scope": "effect-site reverse evidence; not a complete call graph",
        })
    relationship_rows.sort(key=lambda item: (str(item.get("caller_package")), str(item.get("callee_package")), str(item.get("id", item.get("kind")))))
    return {
        "schema_version": SCHEMA_VERSION,
        "revision": revision,
        "source_manifest_sha256": source_manifest_sha256,
        "files": files,
        "declarations": declarations,
        "packages": packages,
        "cross_component_relationships": relationship_rows,
        "relationship_scope": "Go package imports are exact; Reffector relationships are effect-site reverse evidence, not a complete call graph.",
    }


def partition_source_fragments(
    repo: pathlib.Path,
    declarations: list[dict[str, Any]],
    file_paths: list[str],
    program_model: dict[str, Any],
    source_manifest: dict[str, dict[str, Any]],
) -> list[dict[str, Any]]:
    file_index = {item.get("path"): item for item in program_model.get("files", []) if isinstance(item, dict)}
    declarations_by_file: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for declaration in declarations:
        if isinstance(declaration.get("file"), str):
            declarations_by_file[declaration["file"]].append(declaration)
    fragments: list[dict[str, Any]] = []
    for path in sorted(set(file_paths)):
        data = read_repo_file(repo, path)
        expected_hash = source_manifest.get(path, {}).get("sha256")
        if digest(data) != expected_hash:
            raise ArchcharError(f"source changed while constructing partition: {path}")
        lines = data.decode("utf-8").splitlines(keepends=True)
        file_record = file_index.get(path, {})
        selected = declarations_by_file.get(path, [])
        ranges: set[tuple[int, int, str]] = set()
        if not selected:
            ranges.add((1, max(1, len(lines)), "complete_file"))
        else:
            preamble_end = min(int(file_record.get("preamble_end_line", 0)), len(lines))
            if preamble_end > 0:
                ranges.add((1, preamble_end, "package_preamble"))
            for declaration in selected:
                start = int(declaration.get("start_line", 0))
                end = int(declaration.get("end_line", 0))
                if start > 0 and end >= start and start <= len(lines):
                    ranges.add((start, min(end, len(lines)), "declaration"))
        for start, end, kind in sorted(ranges):
            if start < 1 or end < start:
                continue
            fragments.append({
                "path": path,
                "sha256": expected_hash,
                "line_start": start,
                "line_end": end,
                "fragment_kind": kind,
                "text": "".join(lines[start - 1:end]),
            })
    return fragments


def filter_partition_evidence(evidence: dict[str, Any], declarations: list[dict[str, Any]]) -> dict[str, Any]:
    selected_functions = {str(item.get("symbol")) for item in declarations if item.get("kind") == "function"}
    selected_functions.update(str(item.get("name")) for item in declarations if item.get("kind") == "function")
    locations = {
        str(item.get("file")): (int(item.get("start_line", 0)), int(item.get("end_line", 0)))
        for item in declarations if isinstance(item.get("file"), str)
    }

    def local(file: Any, line_number: Any) -> bool:
        if not isinstance(file, str):
            return False
        bounds = locations.get(file)
        return bool(bounds and isinstance(line_number, int) and bounds[0] <= line_number <= bounds[1])

    sites = []
    for site in evidence.get("operation_sites", []):
        function = str(site.get("function", ""))
        source = site.get("source_ref", {})
        if function in selected_functions or local(source.get("file"), source.get("line")):
            sites.append(site)
    audit_sites = []
    for site in evidence.get("uncertainty_sites", []):
        function = str(site.get("function", ""))
        source = site.get("source_ref", {})
        if function in selected_functions or local(source.get("file"), source.get("line")):
            audit_sites.append(site)
    site_ids = {str(item.get("id")) for item in sites + audit_sites if item.get("id") is not None}
    edges = []
    for edge in evidence.get("call_relationships", []):
        caller = edge.get("caller", {}) if isinstance(edge.get("caller"), dict) else {}
        callee = edge.get("callee", {}) if isinstance(edge.get("callee"), dict) else {}
        if caller.get("function") in selected_functions or callee.get("function") in selected_functions or local(edge.get("file"), (edge.get("location") or {}).get("line")):
            edges.append(edge)
    edge_refs = {item.get("evidence_ref") for item in edges}
    node_ids = {item.get("caller_node") for item in edges} | {item.get("callee_node") for item in edges}
    reverse = [item for item in evidence.get("reverse_impact_summaries", []) if str(item.get("site")) in site_ids]
    reverse_refs = {item.get("evidence_ref") for item in reverse}
    nodes = [item for item in evidence.get("nodes", []) if item.get("id") in node_ids]
    effect_counts: Counter[str] = Counter()
    for site in sites:
        for effect in site.get("effects", []):
            effect_counts[str(effect)] += 1
        for effect in site.get("possible_effects", []):
            effect_counts["possible:" + str(effect)] += 1
    result = dict(evidence)
    result.update({
        "scope": "declaration-coherent partition; package coverage and unselected evidence remain addressable by reference",
        "operation_sites": sites,
        "uncertainty_sites": audit_sites,
        "call_relationships": edges,
        "candidate_relationship_refs": [item.get("evidence_ref") for item in edges if item.get("candidate") or item.get("resolution") != "established"],
        "nodes": nodes,
        "reverse_impact_summaries": reverse,
        "effect_site_counts": dict(sorted(effect_counts.items())),
        "coverage": {
            **evidence.get("coverage", {}), "partition_operation_sites": len(sites),
            "partition_uncertainty_sites": len(audit_sites), "partition_call_relationships": len(edges),
            "partition_reverse_records": len(reverse),
        },
        "deferred_record_detail_refs": sorted(set(evidence.get("available_evidence_refs", [])) - {item.get("evidence_ref") for item in sites + audit_sites} - edge_refs - reverse_refs),
    })
    return result


def split_partition_group(group: dict[str, Any], program_model: dict[str, Any]) -> list[dict[str, Any]]:
    declarations = group.get("declarations", [])
    if len(declarations) <= 1:
        return []
    all_declarations = program_model.get("declarations", [])
    types = {item.get("name"): item for item in all_declarations if item.get("package") == group.get("package") and item.get("kind") == "type"}
    result = []
    for declaration in declarations:
        members = [declaration]
        receiver = declaration.get("receiver_type")
        if receiver in types:
            members.insert(0, types[receiver])
        member_ids = {item.get("id") for item in members}
        members = [item for item in members if item.get("id") in member_ids]
        result.append({
            "id": "single:" + str(declaration.get("id")), "package": group.get("package"),
            "declarations": members, "source_files": sorted({item.get("file") for item in members if item.get("file")}),
        })
    return result


def prepare(args: Any) -> pathlib.Path:
    repo = pathlib.Path(args.repo).resolve(strict=True)
    if not repo.is_dir():
        raise ArchcharError("repository path must be a directory")
    output = safe_repo_output(repo, pathlib.Path(args.out))
    patterns = list(getattr(args, "pattern", None) or ["./..."])
    if any(not pattern or pattern.startswith("-") for pattern in patterns):
        raise ArchcharError("Go package patterns must be nonempty positional patterns")
    if min(args.max_sites, args.max_edges, args.max_depth, args.max_context_bytes) <= 0:
        raise ArchcharError("all site, edge, depth, and context limits must be positive")
    if output.exists() and any(output.iterdir()):
        raise ArchcharError("output directory is not empty; use a new run directory to preserve prior artifacts")
    output.mkdir(parents=True, exist_ok=True)
    env, safe_env = make_environment(output)
    token_counter_command = getattr(args, "token_counter_command_json", None)
    token_policy = token_policy_from_args(args)
    if token_counter_command and token_policy is None:
        raise ArchcharError("token counter requires context, output reserve, and safety margin limits")
    metadata: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "status": "partial",
        "repo": str(repo),
        "revision": git_revision(repo),
        "patterns": patterns,
        "go_environment": safe_env,
        "source_configuration": {
            "max_sites": args.max_sites,
            "max_edges": args.max_edges,
            "max_depth": args.max_depth,
            "max_context_bytes": args.max_context_bytes,
            "max_retrieval_rounds": args.max_retrieval_rounds,
            "max_retrieval_bytes": args.max_retrieval_bytes,
            "context_window_tokens": token_policy.get("context_window_tokens") if token_policy else None,
            "output_reserve_tokens": token_policy.get("output_reserve_tokens") if token_policy else None,
            "safety_margin_tokens": token_policy.get("safety_margin_tokens") if token_policy else None,
        },
        "token_budget_policy": token_policy,
        "phases": {},
        "errors": [],
    }
    reffector_arg = args.reffector or "reffector"
    reffector_path = shutil.which(reffector_arg) or (str(pathlib.Path(reffector_arg).resolve()) if pathlib.Path(reffector_arg).exists() else None)
    if not reffector_path:
        if not getattr(args, "effects", None):
            metadata["errors"].append("reffector executable was not found")
        reffector_path = reffector_arg
    else:
        try:
            metadata["reffector"] = {"path": reffector_path, "sha256": digest(pathlib.Path(reffector_path).read_bytes())}
        except OSError:
            metadata["reffector"] = {"path": reffector_path, "sha256": None}

    effect_raw_path = output / "raw-effects.json"
    audit_raw_path = output / "raw-audit.jsonl"
    errors = metadata["errors"]
    if getattr(args, "effects", None):
        effect_bytes = pathlib.Path(args.effects).read_bytes()
        effect_raw_path.write_bytes(effect_bytes)
        metadata["phases"]["effects"] = {"status": "imported", "source": str(pathlib.Path(args.effects).resolve()), "sha256": digest(effect_bytes)}
    else:
        argv = [reffector_path, "-all", "-json", "-max-sites", str(args.max_sites), "-max-edges", str(args.max_edges), "-max-depth", str(args.max_depth), *patterns]
        result = run_process(argv, repo, env, timeout=600)
        effect_raw_path.write_bytes(result["stdout"])
        (output / "raw-effects.stderr").write_bytes(result["stderr"])
        metadata["phases"]["effects"] = command_phase_metadata(argv, result, "raw-effects.json", "raw-effects.stderr")
        if result["returncode"] != 0:
            errors.append("Reffector effects command failed")

    if getattr(args, "audit", None):
        audit_bytes = pathlib.Path(args.audit).read_bytes()
        audit_raw_path.write_bytes(audit_bytes)
        metadata["phases"]["audit"] = {"status": "imported", "source": str(pathlib.Path(args.audit).resolve()), "sha256": digest(audit_bytes)}
    elif not getattr(args, "effects", None):
        # Audit mode rejects graph-limit flags; only the documented audit flags are passed.
        argv = [reffector_path, "-audit-unresolved", "-audit-include-known", "-all", "-stats", *patterns]
        result = run_process(argv, repo, env, timeout=600)
        audit_raw_path.write_bytes(result["stdout"])
        (output / "raw-audit.stderr").write_bytes(result["stderr"])
        metadata["phases"]["audit"] = command_phase_metadata(argv, result, "raw-audit.jsonl", "raw-audit.stderr")
        if result["returncode"] != 0:
            errors.append("Reffector audit command failed")
    else:
        audit_raw_path.write_bytes(b"")
        metadata["phases"]["audit"] = {"status": "missing", "reason": "an audit export was not supplied"}
        errors.append("audit export is required when importing effects JSON")

    try:
        effect_text = effect_raw_path.read_text(encoding="utf-8")
        effects = json.loads(effect_text)
    except (OSError, UnicodeError, ValueError) as error:
        effects = {}
        errors.append(f"effects JSON could not be parsed: {error}")
    errors.extend(validate_effect_report(effects))
    if not isinstance(effects, dict):
        effects = {}
    effect_coverage = effects.get("coverage")
    if not isinstance(effect_coverage, dict):
        effect_coverage = {}
    effect_arrays = {}
    for field in ("groups", "sites", "unresolved_sites", "reverse", "nodes", "edges", "inventory"):
        value = effects.get(field)
        effect_arrays[field] = value if isinstance(value, list) else []
    metadata["effects_coverage"] = effect_coverage
    try:
        audit_text = audit_raw_path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as error:
        audit_text = ""
        errors.append(f"audit JSONL could not be read: {error}")
    audit_records, audit_errors = parse_jsonl(audit_text)
    errors.extend([f"audit JSONL {message}" for message in audit_errors])
    for number, record in enumerate(audit_records, start=1):
        if record.get("record_type") not in ("site", "function") or record.get("schema_version") != 1:
            errors.append(f"audit record {number} has unsupported type or schema")
            break

    go_version = run_process(["go", "version"], repo, env, timeout=30)
    go_env = run_process(["go", "env", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED"], repo, env, timeout=30)
    metadata["go_toolchain"] = {
        "version": go_version["stdout"].decode("utf-8", errors="replace").strip() if go_version["returncode"] == 0 else None,
        "selected_environment": go_env["stdout"].decode("utf-8", errors="replace").splitlines() if go_env["returncode"] == 0 else None,
    }
    list_argv = ["go", "list", "-json", *patterns]
    list_result = run_process(list_argv, repo, env, timeout=600)
    (output / "raw-go-list.json").write_bytes(list_result["stdout"])
    (output / "raw-go-list.stderr").write_bytes(list_result["stderr"])
    metadata["phases"]["go_list"] = command_phase_metadata(list_argv, list_result, "raw-go-list.json", "raw-go-list.stderr")
    if list_result["returncode"] != 0:
        errors.append("go list failed under the recorded offline environment")
    try:
        package_records = parse_json_stream(list_result["stdout"].decode("utf-8"))
    except (UnicodeError, ValueError) as error:
        package_records = []
        errors.append(f"go list JSON could not be parsed: {error}")
    if not package_records:
        errors.append("go list selected no packages")

    package_by_id: dict[str, dict[str, Any]] = {}
    source_files: dict[str, dict[str, Any]] = {}
    exclusions: list[dict[str, Any]] = []
    for package in package_records:
        package_id = package.get("ImportPath")
        if not isinstance(package_id, str) or not package_id:
            errors.append("go list package has no ImportPath")
            continue
        files: list[str] = []
        file_kinds: dict[str, str] = {}
        for field, kind in (("GoFiles", "go"), ("CgoFiles", "cgo"), ("TestGoFiles", "test"), ("XTestGoFiles", "external_test")):
            for name in package.get(field, []):
                absolute = (pathlib.Path(package.get("Dir", repo)) / name).resolve(strict=False)
                relative = relative_source_path(repo, str(absolute))
                if relative is None:
                    exclusions.append({"package": package_id, "file": str(absolute), "reason": "outside repository root"})
                    continue
                if relative not in files:
                    files.append(relative)
                    file_kinds[relative] = kind
        for field in ("IgnoredGoFiles", "InvalidGoFiles", "CFiles", "CXXFiles", "SFiles", "HFiles", "EmbedFiles"):
            for name in package.get(field, []):
                exclusions.append({"package": package_id, "file": name, "reason": field})
        files.sort()
        package_by_id[package_id] = {
            "id": package_id,
            "dir": relative_source_path(repo, package.get("Dir")) or package.get("Dir"),
            "files": files,
            "imports": sorted(set(package.get("Imports", []))),
            "file_kinds": file_kinds,
        }
        for relative in files:
            path = repo / relative
            try:
                data = path.read_bytes()
            except OSError as error:
                source_files[relative] = {"package": package_id, "error": str(error), "bytes": None, "sha256": None, "kind": file_kinds[relative]}
                errors.append(f"selected source file could not be read: {relative}")
                continue
            source_files[relative] = {
                "package": package_id,
                "bytes": len(data),
                "sha256": digest(data),
                "kind": file_kinds[relative],
            }
    selected_files = set(source_files)
    reported_packages = effect_coverage.get("packages")
    selected_packages = sorted(package_by_id)
    if not isinstance(reported_packages, list) or not all(isinstance(item, str) for item in reported_packages):
        errors.append("effects export does not identify selected package coverage")
        metadata["evidence_scope"] = {"status": "unconfirmed", "selected_packages": selected_packages, "effects_packages": None}
    else:
        effects_packages = sorted(set(reported_packages))
        if effects_packages != selected_packages:
            errors.append("effects export package coverage does not match go list source scope")
            metadata["evidence_scope"] = {"status": "mismatch", "selected_packages": selected_packages, "effects_packages": effects_packages}
        else:
            metadata["evidence_scope"] = {"status": "matched", "selected_packages": selected_packages, "effects_packages": selected_packages}

    package_imports: list[dict[str, str]] = []
    for package_id, package in sorted(package_by_id.items()):
        for imported in package["imports"]:
            if imported in package_by_id:
                package_imports.append({"caller_package": package_id, "callee_package": imported, "evidence": "go list Imports"})

    node_records = {
        node.get("id"): node
        for node in effect_arrays["nodes"]
        if isinstance(node, dict) and isinstance(node.get("id"), str)
    }
    node_packages = {
        node_id: node.get("package")
        for node_id, node in node_records.items()
        if isinstance(node.get("package"), str)
    }
    edges = [effect_edge_record(repo, edge, node_packages) for edge in effect_arrays["edges"] if isinstance(edge, dict)]
    effect_sites: list[dict[str, Any]] = []
    unmatched: list[dict[str, Any]] = []
    sites_by_file: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for site in effect_arrays["sites"]:
        if not isinstance(site, dict):
            continue
        relative = relative_source_path(repo, (site.get("location") or {}).get("file"))
        record = dict(site)
        record["repo_file"] = relative
        effect_sites.append(record)
        if relative in selected_files:
            sites_by_file[relative].append(record)
        else:
            unmatched.append({"kind": "effect_site", "id": site.get("id"), "file": relative, "reason": "outside selected source files" if relative else "outside repository or missing location"})
    edges_by_file: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for edge in edges:
        if edge["file"] in selected_files:
            edges_by_file[edge["file"]].append(edge)
        else:
            unmatched.append({"kind": "effect_edge", "id": edge.get("id"), "file": edge.get("file"), "reason": "outside selected source files" if edge.get("file") else "synthetic or missing source location"})

    audit_by_file: dict[str, list[dict[str, Any]]] = defaultdict(list)
    audit_index: list[dict[str, Any]] = []
    for raw_line, record in enumerate(audit_records, start=1):
        locations = parse_location_files(repo, record)
        mapped = [path for path in locations if path in selected_files]
        audit_index.append({"record": record, "repo_files": mapped, "raw_line": raw_line})
        for path in mapped:
            audit_by_file[path].append(record)
        if not mapped:
            symbol = f"{record.get('package', '')}.{record.get('function', '')}".strip(".")
            unmatched.append({"kind": "audit_" + str(record.get("record_type")), "id": record.get("id"), "symbol": symbol, "files": locations, "reason": "outside selected source files" if locations else "missing source location"})

    reverse_by_site: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for reverse in effect_arrays["reverse"]:
        if not isinstance(reverse, dict):
            continue
        site_id = reverse.get("site_id") or reverse.get("site")
        if isinstance(site_id, dict):
            site_id = site_id.get("id")
        if isinstance(site_id, str):
            reverse_by_site[site_id].append(reverse)
    groups_by_site: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for group in effect_arrays["groups"]:
        if not isinstance(group, dict):
            continue
        refs = set()
        for field in ("sites", "possible_sites"):
            values = group.get(field, [])
            if isinstance(values, list):
                refs.update(str(value) for value in values)
        for site_id in refs:
            groups_by_site[site_id].append(group)

    # Package dependencies plus Reffector's effect-site reverse relationships are
    # kept separate; neither is represented as a complete source call graph.
    relationship_inventory = {
        "scope": "package imports and effect-site reverse evidence only; not a complete call graph",
        "package_imports": package_imports,
        "effect_site_reverse_edges": edges,
        "unmatched_references": unmatched,
    }
    write_json(output / "relationships.json", relationship_inventory)
    write_json(output / "source-manifest.json", {"files": source_files, "selected_packages": sorted(package_by_id)})
    inventory = {
        "schema_version": SCHEMA_VERSION,
        "repo": str(repo),
        "revision": metadata["revision"],
        "patterns": patterns,
        "packages": [package_by_id[key] for key in sorted(package_by_id)],
        "source_files": source_files,
        "package_imports": package_imports,
        "coverage": effect_coverage,
        "truncated": effects.get("truncated", {}),
        "effects_schema_version": effects.get("schema_version"),
        "audit_record_counts": {
            "site": sum(record.get("record_type") == "site" for record in audit_records),
            "function": sum(record.get("record_type") == "function" for record in audit_records),
        },
        "unmatched_references": unmatched,
        "exclusions": exclusions,
        "call_graph_scope": relationship_inventory["scope"],
    }
    write_json(output / "inventory.json", inventory)

    if truncation_present(inventory["truncated"]):
        errors.append("Reffector effects report is truncated")
    if effect_coverage.get("omitted", 0):
        errors.append("Reffector omitted selected source functions")
    if any(item["reason"] == "outside selected source files" for item in unmatched):
        # Dependency/source references are retained explicitly, not treated as absent.
        metadata["scope_notes"] = ["Some Reffector evidence refers outside the selected repository source files; see relationships.json."]
    metadata["source_manifest_sha256"] = digest(canonical_json(source_files))
    program_model: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION, "revision": metadata["revision"],
        "source_manifest_sha256": metadata["source_manifest_sha256"], "files": [], "declarations": [],
        "packages": [], "cross_component_relationships": [],
        "relationship_scope": "structural index unavailable",
    }
    try:
        structure = extract_go_structure(repo, package_by_id, source_files, env)
        write_json(output / "program-structure.json", structure)
        program_model = build_program_model(
            repo, package_by_id, structure, package_imports, edges,
            metadata["revision"], metadata["source_manifest_sha256"],
        )
        write_json(output / "program-model.json", program_model)
        metadata["phases"]["structure"] = {
            "status": "complete", "source_files": len(structure["files"]),
            "declarations": len(structure["declarations"]), "artifact": "program-structure.json",
        }
    except (ArchcharError, OSError, ValueError) as error:
        errors.append("Go structural index failed: " + redact_diagnostics(str(error)))
        metadata["phases"]["structure"] = {"status": "failed", "error": redact_diagnostics(str(error))}
        write_json(output / "program-model.json", program_model)
    evidence_index = build_evidence_index(
        effects, audit_index, edges, node_records, lambda raw: relative_source_path(repo, raw),
        metadata["revision"], metadata["source_manifest_sha256"],
    )
    write_json(output / "evidence-index.json", evidence_index)
    metadata["phases"]["evidence_index"] = {"status": "complete", "records": evidence_index["record_count"], "artifact": "evidence-index.json"}
    metadata["program_model_sha256"] = digest(canonical_json(program_model))
    metadata["evidence_index_sha256"] = digest(canonical_json(evidence_index))
    metadata["input_sha256"] = {
        "effects": digest(effect_raw_path.read_bytes()),
        "audit": digest(audit_raw_path.read_bytes()),
        "go_list": digest((output / "raw-go-list.json").read_bytes()),
    }
    metadata["errors"] = errors
    metadata["status"] = "complete" if not errors else "partial"
    write_json(output / "metadata.json", metadata)

    # Build package-sized contexts first. If token-count preflight finds an
    # oversized request, partition by structural declarations and relationships,
    # never by arbitrary byte slices or dropped evidence.
    unit_count = 0
    budget_records: list[dict[str, Any]] = []
    final_packet_statuses: list[dict[str, str]] = []
    structural_ok = metadata["phases"].get("structure", {}).get("status") == "complete"

    def configured_request(package_id: str, package: dict[str, Any], files: list[str], unit_kind: str, include_current: bool = False) -> dict[str, Any]:
        request = build_request(
            repo, package_id, package, files, source_files, sites_by_file,
            audit_by_file, edges_by_file, reverse_by_site, groups_by_site,
            package_imports, edges, package_by_id, node_records, metadata,
        )
        request = reduce_request(request, program_model, evidence_index, set(package_by_id))
        if unit_kind == "package":
            request.get("program", {}).pop("symbol_directory", None)
        request["unit_kind"] = unit_kind
        request["unit_id"] = f"{package_id}::package" if unit_kind == "package" else f"{package_id}::{unit_kind}:{digest(canonical_json(files))[:16]}"
        request["source_manifest_sha256"] = metadata["source_manifest_sha256"]
        request["limits"] = {
            "max_context_bytes": args.max_context_bytes,
            "max_retrieval_rounds": args.max_retrieval_rounds,
            "max_retrieval_bytes": args.max_retrieval_bytes,
            "context_window_tokens": token_policy.get("context_window_tokens") if token_policy else None,
            "output_reserve_tokens": token_policy.get("output_reserve_tokens") if token_policy else None,
            "safety_margin_tokens": token_policy.get("safety_margin_tokens") if token_policy else None,
        }
        allowed_packages = set(request.get("component", {}).get("related_packages", [])) | {package_id}
        request["allowed_source_packages"] = sorted(allowed_packages)
        request["available_source_inventory"] = related_source_inventory(
            package_id, files, package_by_id, source_files, allowed_packages,
            include_current=include_current,
        )
        request["available_same_package_sources"] = request["available_source_inventory"]
        return request

    def evaluate_request(request: dict[str, Any]) -> tuple[int, dict[str, Any] | None]:
        size = request_size(request)
        if size > args.max_context_bytes:
            budget_records.append({"unit_id": request.get("unit_id"), "status": "byte_limit_exceeded", "request_bytes": size})
            return size, None
        if not token_counter_command or not token_policy:
            return size, None
        try:
            budget = run_token_counter(token_counter_command, request, output, token_policy)
            budget["request_sha256"] = digest(canonical_json(request))
        except TokenBudgetError as error:
            budget = {"status": "unavailable", "error": redact_diagnostics(str(error))}
        budget_records.append({"unit_id": request.get("unit_id"), "request_bytes": size, **budget})
        return size, budget

    def make_partition_request(package_id: str, package: dict[str, Any], group: dict[str, Any], ordinal: int) -> dict[str, Any]:
        paths = group.get("source_files", [])
        request = configured_request(package_id, package, paths, "symbol_partition", include_current=True)
        declarations = group.get("declarations", [])
        request["unit_id"] = f"{package_id}::symbol:{group.get('id', ordinal)}"
        request["source"] = partition_source_fragments(repo, declarations, paths, program_model, source_files)
        request["program"]["declarations"] = declarations
        request["program"]["partition"] = {
            "id": group.get("id"), "declaration_count": len(declarations),
            "symbol_ids": [item.get("id") for item in declarations],
            "scope": "source bodies are declaration-scoped; package symbol directory and package/import relationships remain visible",
        }
        request["evidence"] = filter_partition_evidence(request["evidence"], declarations)
        request["component"]["source_files"] = paths
        return request

    def final_status(request: dict[str, Any], size: int, budget: dict[str, Any] | None) -> tuple[str, str | None]:
        if size > args.max_context_bytes:
            return "over_budget", "exact serialized request exceeds the configured byte limit; no source or evidence was truncated"
        if not structural_ok:
            return "structural_index_incomplete", "Go AST index did not cover every selected source file; model calls are blocked"
        if token_counter_command:
            if budget is None:
                return "budget_unavailable", "token-count preflight was unavailable; generation is blocked"
            if budget.get("status") == "unavailable":
                return "budget_unavailable", budget.get("error", "token-count preflight was unavailable")
            if budget.get("status") != "within_budget":
                return "over_budget", "token input plus output and safety reserves exceed runtime context; no generation call"
            return "pending", None
        return "pending_unbudgeted", "token-count preflight is required before model generation"

    for package_id, package in sorted(package_by_id.items()):
        package_request = configured_request(package_id, package, package["files"], "package")
        package_size, package_budget = evaluate_request(package_request)
        candidate_units: list[tuple[str, list[str], dict[str, Any], dict[str, Any] | None]] = []
        if package_size <= args.max_context_bytes and (package_budget is None or package_budget.get("status") == "within_budget"):
            candidate_units.append(("package", package["files"], package_request, package_budget))
        elif package_budget and package_budget.get("status") == "unavailable":
            candidate_units.append(("package", package["files"], package_request, package_budget))
        else:
            groups = partition_groups(program_model, package_id, package["files"], edges)
            covered = {path for group in groups for path in group["source_files"]}
            for path in package["files"]:
                if path not in covered:
                    groups.append({"id": stable_id("file-only", [package_id, path]), "package": package_id, "declarations": [], "source_files": [path]})
            for ordinal, group in enumerate(groups, start=1):
                request = make_partition_request(package_id, package, group, ordinal)
                size, budget = evaluate_request(request)
                if size <= args.max_context_bytes and (budget is None or budget.get("status") == "within_budget"):
                    candidate_units.append(("symbol_partition", group["source_files"], request, budget))
                    continue
                if budget and budget.get("status") == "unavailable":
                    candidate_units.append(("symbol_partition", group["source_files"], request, budget))
                    continue
                subdivisions = split_partition_group(group, program_model)
                if subdivisions:
                    for subordinal, subgroup in enumerate(subdivisions, start=1):
                        subrequest = make_partition_request(package_id, package, subgroup, subordinal)
                        subsize, subbudget = evaluate_request(subrequest)
                        candidate_units.append(("symbol_partition", subgroup["source_files"], subrequest, subbudget))
                else:
                    candidate_units.append(("symbol_partition", group["source_files"], request, budget))
        if not candidate_units and not package["files"]:
            candidate_units.append(("package", [], package_request, package_budget))
        for unit_kind, files, request, budget in candidate_units:
            request_bytes = request_size(request)
            status, reason = final_status(request, request_bytes, budget)
            unit_key = digest(request["unit_id"].encode("utf-8"))[:24]
            packet = {
                "schema_version": SCHEMA_VERSION, "unit_id": request["unit_id"], "unit_key": unit_key,
                "package": package_id, "unit_kind": unit_kind, "source_files": files,
                "request": request, "request_bytes": request_bytes,
                "max_context_bytes": args.max_context_bytes, "token_budget": budget,
                "status": status, "reason": reason,
            }
            write_json(output / "packets" / f"{unit_key}.json", packet)
            write_json(output / "components" / f"{unit_key}.json", {
                "unit_id": request["unit_id"], "package": package_id, "status": status,
                "request_bytes": request_bytes, "token_budget": budget,
            })
            final_packet_statuses.append({"unit_id": request["unit_id"], "status": status})
            unit_count += 1
    if token_counter_command:
        write_json(output / "token-budget-report.json", {
            "schema_version": SCHEMA_VERSION, "policy": token_policy,
            "status": "complete" if final_packet_statuses and all(item["status"] == "pending" for item in final_packet_statuses) else "partial",
            "candidate_count": len(budget_records), "candidates": budget_records,
            "final_packets": final_packet_statuses,
        })
    else:
        write_json(output / "token-budget-report.json", {
            "schema_version": SCHEMA_VERSION, "status": "not_run",
            "reason": "No token-count command was configured; pending_unbudgeted packets cannot be sent to a model.",
        })
    inventory["analysis_units"] = unit_count
    inventory["unit_coverage"] = {"source_files": len(source_files), "packages": len(package_by_id), "analysis_units": unit_count}
    write_json(output / "inventory.json", inventory)
    print(f"{metadata['status']} {output}")
    return output


def request_size(request: dict[str, Any]) -> int:
    return len(canonical_json(request))


def compact_audit_site(record: dict[str, Any], repo: pathlib.Path) -> dict[str, Any]:
    candidates = record.get("vta_candidates", [])
    nodes = record.get("ssa_nodes", [])
    candidates = candidates if isinstance(candidates, list) else []
    nodes = nodes if isinstance(nodes, list) else []
    location = record.get("location", {})
    return {
        "record_type": "site",
        "schema_version": record.get("schema_version"),
        "id": record.get("id"),
        "package": record.get("package"),
        "function": record.get("function"),
        "kind": record.get("kind"),
        "file": relative_source_path(repo, location.get("file")),
        "line": location.get("line"),
        "column": location.get("column"),
        "effects": record.get("effects", []),
        "possible_effects": record.get("possible_effects", []),
        "uncertain": record.get("uncertain", False),
        "dispatch_kinds": record.get("dispatch_kinds", []),
        "origin": record.get("origin"),
        "selected": record.get("selected"),
        "recognition": record.get("recognition"),
        "primary_reason": record.get("primary_reason"),
        "uncertainty_reasons": record.get("uncertainty_reasons", []),
        "unknown_api": record.get("unknown_api", False),
        "vta_candidate_count": record.get("vta_candidate_count", len(candidates)),
        "vta_candidates": [
            {key: candidate.get(key) for key in ("id", "package", "function", "variant", "api", "effect_propagation_supported") if key in candidate}
            for candidate in candidates[:32] if isinstance(candidate, dict)
        ],
        "vta_candidates_truncated": len(candidates) > 32,
        "ssa_node_count": len(nodes),
        "ssa_node_ids": [node.get("id") for node in nodes[:32] if isinstance(node, dict) and isinstance(node.get("id"), str)],
        "ssa_node_ids_truncated": len(nodes) > 32,
    }


def compact_reverse_summary(reverse: dict[str, Any]) -> dict[str, Any]:
    origins = reverse.get("origins", [])
    edges = reverse.get("edges", [])
    nodes = reverse.get("nodes", [])
    origins = origins if isinstance(origins, list) else []
    edges = edges if isinstance(edges, list) else []
    nodes = nodes if isinstance(nodes, list) else []
    sample_limit = 4
    edge_ids = [edge if isinstance(edge, str) else edge.get("id") for edge in edges]
    node_records = [
        {"id": node.get("id"), "depth": node.get("depth")}
        for node in nodes[:sample_limit] if isinstance(node, dict) and isinstance(node.get("id"), str)
    ]
    return {
        "site": reverse.get("site") or reverse.get("site_id"),
        "origins": origins[:sample_limit],
        "origin_count": len(origins),
        "origins_truncated": len(origins) > sample_limit,
        "edges": edge_ids[:sample_limit],
        "edge_count": len(edges),
        "edge_ids_truncated": len(edges) > sample_limit,
        "nodes": node_records,
        "node_count": len(nodes),
        "node_ids_truncated": len(nodes) > sample_limit,
        "cycle": reverse.get("cycle", False),
        "revisit": reverse.get("revisit", False),
        "analysis_truncated": reverse.get("truncated", {}),
    }


def build_request(
    repo: pathlib.Path,
    package_id: str,
    package: dict[str, Any],
    file_paths: list[str],
    source_manifest: dict[str, dict[str, Any]],
    sites_by_file: dict[str, list[dict[str, Any]]],
    audit_by_file: dict[str, list[dict[str, Any]]],
    edges_by_file: dict[str, list[dict[str, Any]]],
    reverse_by_site: dict[str, list[dict[str, Any]]],
    groups_by_site: dict[str, list[dict[str, Any]]],
    package_imports: list[dict[str, str]],
    all_edges: list[dict[str, Any]],
    packages: dict[str, dict[str, Any]],
    node_records: dict[str, dict[str, Any]],
    metadata: dict[str, Any],
) -> dict[str, Any]:
    source: list[dict[str, Any]] = []
    local_sites: list[dict[str, Any]] = []
    local_site_audit: dict[str, dict[str, Any]] = {}
    local_audit: dict[str, dict[str, Any]] = {}
    local_edges: dict[str, dict[str, Any]] = {}
    for path in file_paths:
        data = read_repo_file(repo, path)
        current_hash = digest(data)
        if current_hash != source_manifest.get(path, {}).get("sha256"):
            raise ArchcharError(f"source changed while preparing analysis: {path}")
        source.append({"path": path, "sha256": current_hash, "text": data.decode("utf-8", errors="replace")})
        local_sites.extend(dict(site) for site in sites_by_file.get(path, []))
        for record in audit_by_file.get(path, []):
            if record.get("record_type") == "site" and isinstance(record.get("id"), str):
                local_site_audit[record["id"]] = record
            else:
                key = str(record.get("id") or f"{record.get('record_type')}:{record.get('package')}:{record.get('function')}")
                local_audit[key] = record
        for edge in edges_by_file.get(path, []):
            key = str(edge.get("id") or edge.get("source_edge_id") or digest(canonical_json(edge)))
            local_edges[key] = edge
    site_ids = {str(site.get("id")) for site in local_sites if site.get("id")}
    for site in local_sites:
        audit_record = local_site_audit.get(str(site.get("id")))
        if audit_record is not None:
            site["audit"] = compact_audit_site(audit_record, repo)
    for site_id, audit_record in local_site_audit.items():
        if site_id not in site_ids:
            local_audit[f"site:{site_id}"] = compact_audit_site(audit_record, repo)
    local_groups: dict[str, dict[str, Any]] = {}
    local_reverse: dict[str, dict[str, Any]] = {}
    for site_id in site_ids:
        for group in groups_by_site.get(site_id, []):
            key = digest(canonical_json(group))
            local_groups[key] = group
        for reverse in reverse_by_site.get(site_id, []):
            key = digest(canonical_json(reverse))
            local_reverse[key] = reverse
    node_ids: set[str] = set()
    cross_package_edges: list[dict[str, Any]] = []
    for edge in local_edges.values():
        if edge.get("caller_node"):
            node_ids.add(edge["caller_node"])
        if edge.get("callee_node"):
            node_ids.add(edge["callee_node"])
        if edge.get("caller_package") != edge.get("callee_package"):
            cross_package_edges.append(edge)
    # Import relationships are exact package-level evidence; call edges are only
    # those reverse edges whose source location belongs to the supplied files.
    import_links = [
        link for link in package_imports
        if link["caller_package"] == package_id or link["callee_package"] == package_id
    ]
    adjacent_packages = related_packages(package_id, package_imports, all_edges)
    return {
        "protocol": "reffector-archchar/v1",
        "phase": "component",
        "system": COMPONENT_PROMPT,
        "unit_id": f"{package_id}::{file_paths[0] if len(file_paths) == 1 else 'package'}",
        "unit_kind": "package" if len(file_paths) > 1 else "file_partition",
        "revision": metadata["revision"],
        "component": {
            "package": package_id,
            "imports": package["imports"],
            "direct_selected_imports": [link for link in import_links if link["caller_package"] == package_id],
            "source_files": file_paths,
            "related_packages": adjacent_packages,
            "boundary_note": "A Go package is a deterministic starting unit, not an established architectural component. File partitions are context units, not inferred architecture.",
        },
        "source": source,
        "evidence": {
            "effects_schema_version": SCHEMA_VERSION,
            "coverage": {
                "packages_selected": metadata.get("effects_coverage", {}).get("packages_selected"),
                "source_functions": metadata.get("effects_coverage", {}).get("source_functions"),
            },
            "sites": unique_records(local_sites, "id"),
            "audit_records": list(local_audit.values()),
            "audit_site_projection": "Matching audit site fields are merged into the effect site. VTA candidates and SSA node IDs are limited to 32 per site with counts/truncation flags; full records remain in raw-audit.jsonl.",
            "effect_site_reverse_edges": list(local_edges.values()),
            "cross_package_edge_ids": [edge.get("id") for edge in cross_package_edges],
            "cross_package_edge_count": len(cross_package_edges),
            "nodes": [node_records[node_id] for node_id in sorted(node_ids) if node_id in node_records],
            "groups": list(local_groups.values()),
            "reverse_summaries": [compact_reverse_summary(record) for record in local_reverse.values()],
            "reverse_summary_projection": "Reverse summaries preserve site/origin IDs, analysis truncation, and counts; edge/node identifiers are sampled to 4 with explicit counts and truncation flags. Full evidence remains in raw-effects.json and relationships.json.",
            "package_import_relationships": import_links,
            "graph_scope": "effect-site reverse evidence plus package imports; not a complete call graph",
        },
        "questions": [
            "Describe this unit's responsibilities and important relationships, citing exact source and Reffector evidence.",
            "Separate established static evidence, candidates, inference, and unresolved boundaries; do not claim runtime execution from static reachability.",
        ],
        "allowed_source_package": package_id,
    }


def unique_records(records: list[dict[str, Any]], key: str) -> list[dict[str, Any]]:
    seen: set[str] = set()
    result: list[dict[str, Any]] = []
    for record in records:
        value = str(record.get(key) or canonical_json(record))
        if value not in seen:
            seen.add(value)
            result.append(record)
    return result


def related_packages(package_id: str, imports: list[dict[str, str]], edges: list[dict[str, Any]]) -> list[str]:
    related = {package_id}
    for link in imports:
        if package_id in (link["caller_package"], link["callee_package"]):
            related.add(link["caller_package"])
            related.add(link["callee_package"])
    for edge in edges:
        if package_id in (edge.get("caller_package"), edge.get("callee_package")):
            for endpoint in (edge.get("caller_package"), edge.get("callee_package")):
                if endpoint:
                    related.add(endpoint)
    return sorted(related)


def related_source_inventory(
    package_id: str,
    file_paths: list[str],
    packages: dict[str, dict[str, Any]],
    source_manifest: dict[str, dict[str, Any]],
    allowed_packages: set[str] | None = None,
    include_current: bool = False,
) -> list[dict[str, Any]]:
    # Source retrieval is limited to the selected package and its indexed
    # package/import or effect-site neighbors. Authority is explicit per path.
    allowed = allowed_packages or {package_id}
    current_files = set(file_paths)
    result = []
    for related_package in sorted(allowed):
        for path in packages.get(related_package, {}).get("files", []):
            if path in current_files and not include_current:
                continue
            source = source_manifest.get(path, {})
            result.append({"file": path, "package": related_package, "bytes": source.get("bytes"), "sha256": source.get("sha256")})
    return result


def sanitize_model_value(value: Any) -> Any:
    if isinstance(value, float) and not math.isfinite(value):
        return None
    if isinstance(value, str):
        return redact_diagnostics(value)
    if isinstance(value, list):
        return [sanitize_model_value(item) for item in value]
    if isinstance(value, dict):
        return {
            key: "<redacted>" if SENSITIVE_NAME.search(str(key)) else sanitize_model_value(item)
            for key, item in value.items()
        }
    return value


def invoke_model(
    command_json: str | list[str], request: dict[str, Any], cwd: pathlib.Path,
    timeout: int = 300, response_kind: str = "claims",
    token_counter_command_json: str | list[str] | None = None,
    token_policy: dict[str, int] | None = None,
    preflight_budget: dict[str, Any] | None = None,
) -> dict[str, Any]:
    start = time.monotonic()
    try:
        argv = json.loads(command_json) if isinstance(command_json, str) else command_json
        if not isinstance(argv, list) or not argv or not all(isinstance(arg, str) for arg in argv):
            raise ArchcharError("model command must be a nonempty JSON argv array")
        request_hash = digest(canonical_json(request))
        if preflight_budget is not None:
            if preflight_budget.get("request_sha256") != request_hash:
                raise TokenBudgetError("preflight token budget does not match this request")
            if preflight_budget.get("status") != "within_budget":
                raise TokenBudgetError("preflight token budget is not within the declared runtime limit")
            if not isinstance(token_policy, dict) or any(
                preflight_budget.get(key) != expected
                for key, expected in (("output_reserved_tokens", token_policy.get("output_reserve_tokens")),
                                      ("safety_margin_tokens", token_policy.get("safety_margin_tokens")),
                                      ("declared_context_window_tokens", token_policy.get("context_window_tokens")))
            ):
                raise TokenBudgetError("preflight token budget does not match the declared reserve policy")
            token_budget = preflight_budget
        else:
            token_budget = run_token_counter(token_counter_command_json, request, cwd, token_policy)
            token_budget["request_sha256"] = request_hash
        if token_budget["status"] != "within_budget":
            return {
                "status": "failed", "error": "request exceeds token context budget; generation was not started",
                "request": request, "request_sha256": digest(canonical_json(request)),
                "raw_stdout": "", "raw_stderr": "", "result": None, "schema_valid": False,
                "latency_seconds": time.monotonic() - start, "usage": None, "cost_usd": None,
                "token_budget": token_budget, "command": safe_command_display(argv),
                "token_counter_command": safe_command_display(token_budget.get("counter_command", [])),
            }
        process = subprocess.run(
            argv,
            cwd=cwd,
            input=canonical_json(request).decode("utf-8"),
            text=True,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
        try:
            response = json.loads(process.stdout)
        except ValueError:
            response = None
        if response_kind == "claims":
            valid = isinstance(response, dict) and isinstance(response.get("claims"), list)
            if valid:
                valid = all(valid_claim_shape(claim, aggregate=request.get("phase") == "aggregate") for claim in response["claims"])
            if valid and "need_sources" in response:
                valid = isinstance(response["need_sources"], list) and all(isinstance(path, str) for path in response["need_sources"])
            if valid and "need_evidence" in response:
                valid = isinstance(response["need_evidence"], list) and all(isinstance(ref, str) for ref in response["need_evidence"])
            if valid and "need_source_refs" in response:
                valid = isinstance(response["need_source_refs"], list) and all(isinstance(ref, str) for ref in response["need_source_refs"])
        else:
            valid = isinstance(response, dict) and isinstance(response.get("verdicts"), list)
            if valid:
                valid = all(
                    isinstance(verdict, dict)
                    and isinstance(verdict.get("claim_id"), str)
                    and verdict.get("verdict") in {"supported", "unsupported", "uncertain"}
                    and isinstance(verdict.get("reason"), str)
                    for verdict in response["verdicts"]
                )
        if valid:
            reported_model = response.get("model") if isinstance(response, dict) else None
            valid = isinstance(reported_model, str) and reported_model == token_budget.get("model")
        status = "complete" if process.returncode == 0 and valid else "failed"
        safe_response = sanitize_model_value(response)
        raw_usage = response.get("usage") if isinstance(response, dict) else None
        usage = None
        if isinstance(raw_usage, dict):
            usage = {
                field: value
                for field, value in raw_usage.items()
                if field in {"input_tokens", "output_tokens", "total_tokens", "cache_read_input_tokens", "cache_creation_input_tokens"}
                and isinstance(value, int)
                and not isinstance(value, bool)
                and value >= 0
            } or None
        cost = response.get("cost_usd") if isinstance(response, dict) else None
        if cost is None and isinstance(response, dict):
            cost = response.get("cost")
        if not isinstance(cost, (int, float)) or isinstance(cost, bool) or not math.isfinite(cost) or cost < 0:
            cost = None
        return {
            "status": status,
            "returncode": process.returncode,
            "request": request,
            "request_sha256": digest(canonical_json(request)),
            "raw_stdout": redact_diagnostics(process.stdout),
            "raw_stderr": redact_diagnostics(process.stderr),
            "result": safe_response,
            "schema_valid": bool(valid),
            "latency_seconds": time.monotonic() - start,
            "usage": usage,
            "cost_usd": float(cost) if cost is not None else None,
            "model": safe_response.get("model") if isinstance(safe_response, dict) else None,
            "command": safe_command_display(argv),
            "token_budget": token_budget,
            "token_counter_command": safe_command_display(token_budget.get("counter_command", [])),
        }
    except (OSError, subprocess.TimeoutExpired, ArchcharError, TokenBudgetError, ValueError) as error:
        return {
            "status": "failed",
            "error": redact_diagnostics(str(error)),
            "request": request,
            "request_sha256": digest(canonical_json(request)),
            "raw_stdout": "",
            "raw_stderr": "",
            "schema_valid": False,
            "latency_seconds": time.monotonic() - start,
            "usage": None,
            "cost_usd": None,
            "token_budget": locals().get("token_budget"),
            "token_counter_command": safe_command_display(token_counter_command_json) if isinstance(token_counter_command_json, list) else None,
        }


def invoke_saved_model_call(
    run: pathlib.Path,
    phase_directory: str,
    key: str,
    command_json: str,
    request: dict[str, Any],
    response_kind: str = "claims",
    token_counter_command_json: str | list[str] | None = None,
    token_policy: dict[str, int] | None = None,
    preflight_budget: dict[str, Any] | None = None,
) -> tuple[dict[str, Any], pathlib.Path]:
    directory = run / phase_directory
    base = directory / f"{key}.json"
    candidates = [base, *sorted(directory.glob(f"{key}.attempt-*.json"))] if directory.exists() else [base]
    request_hash = digest(canonical_json(request))
    try:
        command_argv = json.loads(command_json)
        command_display = safe_command_display(command_argv) if isinstance(command_argv, list) else None
    except ValueError:
        command_display = None
    counter_argv = token_counter_command_json
    if isinstance(counter_argv, str):
        try:
            counter_argv = json.loads(counter_argv)
        except ValueError:
            counter_argv = None
    counter_display = safe_command_display(counter_argv) if isinstance(counter_argv, list) else None
    for path in candidates:
        if not path.exists():
            continue
        try:
            prior = read_json(path)
        except (OSError, ValueError):
            continue
        if (
            prior.get("request_sha256") == request_hash
            and prior.get("command") == command_display
            and prior.get("token_counter_command") == counter_display
            and (prior.get("token_budget") or {}).get("context_window_tokens") == (token_policy or {}).get("context_window_tokens")
            and (prior.get("token_budget") or {}).get("output_reserved_tokens") == (token_policy or {}).get("output_reserve_tokens")
            and (prior.get("token_budget") or {}).get("safety_margin_tokens") == (token_policy or {}).get("safety_margin_tokens")
            and prior.get("status") == "complete"
        ):
            return prior, path
    if not base.exists():
        path = base
    else:
        attempt = 1
        while True:
            path = directory / f"{key}.attempt-{attempt:03d}.json"
            if not path.exists():
                break
            attempt += 1
    response = invoke_model(
        command_json, request, run, response_kind=response_kind,
        token_counter_command_json=token_counter_command_json, token_policy=token_policy,
        preflight_budget=preflight_budget,
    )
    write_json(path, response)
    return response, path


def valid_claim_shape(claim: Any, aggregate: bool = False) -> bool:
    if not isinstance(claim, dict):
        return False
    required = {"id", "statement", "category", "domain", "evidence"}
    if aggregate:
        required.add("child_claim_ids")
    if not required.issubset(claim):
        return False
    if not isinstance(claim["id"], str) or not isinstance(claim["statement"], str):
        return False
    if claim["category"] not in CATEGORIES or claim["domain"] not in DOMAINS:
        return False
    if not isinstance(claim["evidence"], list):
        return False
    if aggregate and not isinstance(claim["child_claim_ids"], list):
        return False
    return True


def packet_files(run: pathlib.Path) -> list[pathlib.Path]:
    return sorted((run / "packets").glob("*.json"))


def component_artifact_path(run: pathlib.Path, unit_key: str) -> pathlib.Path:
    return run / "components" / f"{unit_key}.json"


def save_call(run: pathlib.Path, unit_key: str, attempt: int, response: dict[str, Any]) -> pathlib.Path:
    directory = run / "model-calls" / unit_key
    path = directory / f"attempt-{attempt:03d}.json"
    while path.exists():
        attempt += 1
        path = directory / f"attempt-{attempt:03d}.json"
    write_json(path, response)
    return path


def investigate(
    run_path: str, model_command_json: str, max_rounds: int | None = None,
    max_retrieval_bytes: int | None = None,
    token_counter_command_json: str | list[str] | None = None,
    token_policy: dict[str, int] | None = None,
) -> dict[str, Any]:
    run = pathlib.Path(run_path).resolve()
    metadata = read_json(run / "metadata.json")
    manifest = read_json(run / "source-manifest.json")["files"]
    source_limit = metadata["source_configuration"]["max_context_bytes"]
    rounds_limit = max_rounds if max_rounds is not None else metadata["source_configuration"]["max_retrieval_rounds"]
    retrieval_limit = max_retrieval_bytes if max_retrieval_bytes is not None else metadata["source_configuration"]["max_retrieval_bytes"]
    token_policy = token_policy or metadata.get("token_budget_policy")
    statuses: list[dict[str, Any]] = []
    repo = pathlib.Path(metadata["repo"])
    if metadata.get("phases", {}).get("structure", {}).get("status") != "complete":
        summary = {"status": "partial", "unit_count": len(packet_files(run)), "reason": "complete Go structural indexing is required before model generation", "units": []}
        write_json(run / "investigation-status.json", summary)
        return summary
    changed_sources: list[str] = []
    for source_path, info in sorted(manifest.items()):
        try:
            if digest(read_repo_file(repo, source_path)) != info["sha256"]:
                changed_sources.append(source_path)
        except (ArchcharError, OSError):
            changed_sources.append(source_path)
    if changed_sources:
        summary = {"status": "partial", "unit_count": len(packet_files(run)), "source_scope_changed": changed_sources, "units": [{"status": "stale", "source": path} for path in changed_sources]}
        write_json(run / "investigation-status.json", summary)
        return summary
    evidence_rows = {row.get("ref"): row for row in read_json(run / "evidence-index.json").get("records", []) if isinstance(row, dict)}
    for packet_path in packet_files(run):
        packet = read_json(packet_path)
        key = packet["unit_key"]
        destination = component_artifact_path(run, key)
        if destination.exists():
            prior = read_json(destination)
            if prior.get("status") == "complete":
                statuses.append({"unit_id": packet["unit_id"], "status": "complete", "resumed": True})
                continue
        if packet.get("status") != "pending":
            statuses.append({"unit_id": packet["unit_id"], "status": packet.get("status"), "request_bytes": packet["request_bytes"], "reason": packet.get("reason")})
            continue
        base_request = packet["request"]
        current_source = list(base_request.get("source", []))
        allowed = {item["file"]: item for item in base_request.get("available_source_inventory", base_request.get("available_same_package_sources", []))}
        allowed_packages = set(base_request.get("allowed_source_packages", [base_request.get("allowed_source_package")]))
        retrieved: list[dict[str, Any]] = []
        retrieved_evidence: list[dict[str, Any]] = []
        call_summaries: list[dict[str, Any]] = []
        all_claims: list[dict[str, Any]] = []
        pending_requests: list[str] = []
        total_extra_bytes = 0
        request = base_request
        status = "partial"
        for attempt in range(rounds_limit + 1):
            size = request_size(request)
            if size > source_limit:
                pending_requests.append(f"follow-up request is {size} bytes, above {source_limit}")
                break
            stale = []
            for source in request.get("source", []) + request.get("additional_sources", []):
                try:
                    if digest(read_repo_file(repo, source["path"])) != manifest[source["path"]]["sha256"]:
                        stale.append(source["path"])
                except (ArchcharError, OSError, KeyError):
                    stale.append(source["path"])
            if stale:
                pending_requests.append("source changed or disappeared since prepare: " + ", ".join(sorted(set(stale))))
                break
            response = invoke_model(
                model_command_json, request, run, token_counter_command_json=token_counter_command_json,
                token_policy=token_policy,
            )
            call_path = save_call(run, key, attempt, response)
            call_summaries.append({
                "artifact": str(call_path.relative_to(run)), "status": response["status"],
                "latency_seconds": response["latency_seconds"], "usage": response["usage"],
                "cost_usd": response["cost_usd"], "model": response["model"],
                "token_budget": response.get("token_budget"),
            })
            if response["status"] != "complete":
                pending_requests.append(response.get("error", "model response failed schema validation or subprocess execution"))
                break
            content = response["result"]
            all_claims.extend(content["claims"])
            requested_sources = content.get("need_sources", [])
            requested_evidence = content.get("need_evidence", [])
            if not requested_sources and not requested_evidence:
                status = "complete"
                break
            if attempt >= rounds_limit:
                pending_requests.extend("source: " + str(path) for path in requested_sources)
                pending_requests.extend("evidence: " + str(ref) for ref in requested_evidence)
                break
            additions: list[dict[str, Any]] = []
            present_source_paths = {
                item["path"] for item in current_source + retrieved
                if not item.get("fragment_kind")
            }
            for requested_path in requested_sources:
                if not isinstance(requested_path, str):
                    pending_requests.append("rejected non-string source request")
                    continue
                normalized_request = requested_path.replace(chr(92), "/")
                pure = pathlib.PurePosixPath(normalized_request)
                normalized = pure.as_posix()
                if pure.is_absolute() or ".." in pure.parts or normalized not in allowed:
                    pending_requests.append(f"rejected out-of-scope source request: {requested_path}")
                    continue
                if normalized in present_source_paths:
                    pending_requests.append(f"complete source already present: {normalized}")
                    continue
                info = allowed[normalized]
                if info.get("package") not in allowed_packages:
                    pending_requests.append(f"rejected unrelated package source: {normalized}")
                    continue
                try:
                    data = read_repo_file(repo, normalized)
                except (ArchcharError, OSError):
                    pending_requests.append(f"source is unavailable or outside repository: {normalized}")
                    continue
                if digest(data) != manifest.get(normalized, {}).get("sha256"):
                    pending_requests.append(f"source changed since preparation: {normalized}")
                    continue
                addition_bytes = len(data)
                if total_extra_bytes + addition_bytes > retrieval_limit:
                    pending_requests.append(f"retrieval byte budget exceeded by {normalized}")
                    continue
                total_extra_bytes += addition_bytes
                additions.append({"path": normalized, "package": info["package"], "sha256": digest(data), "text": data.decode("utf-8", errors="replace")})
                present_source_paths.add(normalized)
            evidence_additions: list[dict[str, Any]] = []
            available_refs = set(base_request.get("evidence", {}).get("available_evidence_refs", []))
            present_evidence_refs = {item.get("ref") for item in retrieved_evidence}
            for requested_ref in requested_evidence:
                if not isinstance(requested_ref, str) or requested_ref not in available_refs:
                    pending_requests.append(f"rejected out-of-scope evidence request: {requested_ref}")
                    continue
                if requested_ref in present_evidence_refs:
                    pending_requests.append(f"evidence already retrieved: {requested_ref}")
                    continue
                row = evidence_rows.get(requested_ref)
                if not isinstance(row, dict):
                    pending_requests.append(f"evidence reference is unavailable: {requested_ref}")
                    continue
                record = {"ref": requested_ref, "record_type": row.get("record_type"), "record": row.get("data")}
                addition_bytes = len(canonical_json(record))
                if total_extra_bytes + addition_bytes > retrieval_limit:
                    pending_requests.append(f"retrieval byte budget exceeded by evidence {requested_ref}")
                    continue
                total_extra_bytes += addition_bytes
                evidence_additions.append(record)
                present_evidence_refs.add(requested_ref)
            if not additions and not evidence_additions:
                break
            retrieved.extend(additions)
            retrieved_evidence.extend(evidence_additions)
            request = {
                **base_request, "phase": "component_followup",
                "additional_sources": retrieved, "additional_evidence": retrieved_evidence,
                "prior_claims": all_claims, "retrieval_round": attempt + 1,
                "retrieval_bytes": total_extra_bytes,
                "limits": {"max_context_bytes": source_limit, "max_retrieval_bytes": retrieval_limit, "max_retrieval_rounds": rounds_limit},
            }
        result = {
            "schema_version": SCHEMA_VERSION, "unit_id": packet["unit_id"], "package": packet["package"],
            "unit_kind": packet["unit_kind"], "status": status if not pending_requests else "partial",
            "claims": all_claims, "calls": call_summaries,
            "retrieved_sources": [{"path": item["path"], "package": item["package"], "bytes": len(item["text"].encode("utf-8")), "sha256": item["sha256"]} for item in retrieved],
            "retrieved_evidence_refs": [item["ref"] for item in retrieved_evidence],
            "unresolved_requests": pending_requests, "request_bytes": packet["request_bytes"],
            "max_context_bytes": source_limit,
        }
        write_json(destination, result)
        statuses.append({"unit_id": packet["unit_id"], "status": result["status"], "calls": len(call_summaries), "request_bytes": packet["request_bytes"]})
    complete = metadata.get("status") == "complete" and bool(statuses) and all(item["status"] == "complete" for item in statuses)
    summary = {"status": "complete" if complete else "partial", "preparation_status": metadata.get("status"), "units": statuses, "unit_count": len(statuses)}
    write_json(run / "investigation-status.json", summary)
    return summary


def claim_conflicts(claims: list[dict[str, Any]]) -> list[dict[str, Any]]:
    by_id: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for claim in claims:
        if isinstance(claim, dict) and isinstance(claim.get("id"), str):
            by_id[claim["id"]].append(claim)
    return [
        {"claim_id": claim_id, "claims": values}
        for claim_id, values in sorted(by_id.items())
        if len({(value.get("statement"), value.get("category"), value.get("domain")) for value in values}) > 1
    ]


def aggregate(
    run_path: str, model_command_json: str | None = None, max_context_bytes: int | None = None,
    token_counter_command_json: str | list[str] | None = None,
    token_policy: dict[str, int] | None = None,
) -> dict[str, Any]:
    run = pathlib.Path(run_path).resolve()
    metadata = read_json(run / "metadata.json")
    inventory = read_json(run / "inventory.json")
    limit = max_context_bytes or metadata["source_configuration"]["max_context_bytes"]
    token_policy = token_policy or metadata.get("token_budget_policy")
    packet_by_id: dict[str, dict[str, Any]] = {}
    for path in packet_files(run):
        packet = read_json(path)
        packet_by_id[packet["unit_id"]] = packet
    result_by_id: dict[str, dict[str, Any]] = {}
    for path in sorted((run / "components").glob("*.json")):
        component = read_json(path)
        if component.get("unit_id"):
            result_by_id[component["unit_id"]] = component

    package_nodes: dict[str, dict[str, Any]] = {}
    for package in inventory["packages"]:
        package_id = package["id"]
        expected = sorted(unit_id for unit_id, packet in packet_by_id.items() if packet.get("package") == package_id)
        children = []
        for unit_id in expected:
            unit = result_by_id.get(unit_id, {"status": "pending", "claims": []})
            children.append({
                "id": unit_id,
                "claims": unit.get("claims", []),
                "status": unit.get("status", "pending"),
                "source_files": packet_by_id[unit_id].get("source_files", []),
            })
        claims = [claim for child in children for claim in child["claims"] if isinstance(claim, dict)]
        missing = [child["id"] for child in children if child["status"] != "complete"]
        node = {
            "id": "package:" + package_id,
            "kind": "package",
            "package": package_id,
            "children": expected,
            "child_summaries": children,
            "claims": claims,
            "model_claims": [],
            "conflicts": claim_conflicts(claims),
            "missing_children": missing,
            "status": "complete" if children and not missing else "partial",
            "aggregation": "structural_only",
        }
        if model_command_json and node["status"] == "complete":
            node.update(aggregate_model_call(run, package_id, "package", node, children, inventory, model_command_json, limit, token_counter_command_json, token_policy))
        package_nodes[package_id] = node

    group_tree, roots = namespace_hierarchy(sorted(package_nodes))
    group_nodes: dict[str, dict[str, Any]] = {}
    for group_id in sorted(group_tree, key=lambda value: (-value.count("/"), value)):
        child_ids = group_tree[group_id]
        children: list[dict[str, Any]] = []
        for child_id in child_ids:
            if child_id.startswith("package:"):
                children.append(package_nodes[child_id[len("package:"):]])
            elif child_id in group_nodes:
                children.append(group_nodes[child_id])
        child_claims = [
            claim
            for child in children
            for claim in (child.get("model_claims") or child.get("claims", []))
            if isinstance(claim, dict)
        ]
        group = {
            "id": "namespace:" + group_id,
            "kind": "namespace_group",
            "namespace": group_id,
            "children": child_ids,
            "claims": child_claims,
            "model_claims": [],
            "conflicts": claim_conflicts(child_claims),
            "status": "complete" if children and all(child.get("status") == "complete" for child in children) else "partial",
            "aggregation": "structural_only",
        }
        if model_command_json and group["status"] == "complete":
            group.update(aggregate_model_call(run, group_id, "namespace_group", group, children, inventory, model_command_json, limit, token_counter_command_json, token_policy))
        group_nodes[group_id] = group

    component_claims = [
        claim
        for path in sorted((run / "components").glob("*.json"))
        for claim in read_json(path).get("claims", [])
        if isinstance(claim, dict)
    ]
    package_aggregate_claims = [claim for node in package_nodes.values() for claim in node.get("model_claims", [])]
    namespace_aggregate_claims = [claim for node in group_nodes.values() for claim in node.get("model_claims", [])]
    all_claims = component_claims + package_aggregate_claims + namespace_aggregate_claims
    all_conflicts = [conflict for node in package_nodes.values() for conflict in node["conflicts"]]
    all_conflicts.extend(conflict for node in group_nodes.values() for conflict in node["conflicts"])
    package_complete = bool(package_nodes) and all(node["status"] == "complete" for node in package_nodes.values())
    groups_complete = all(group_nodes[root].get("status") == "complete" for root in roots) if roots else True
    truncated = truncation_present(inventory.get("truncated")) or bool(inventory.get("coverage", {}).get("omitted", 0))
    incomplete = (
        not package_complete
        or not groups_complete
        or metadata.get("status") != "complete"
        or truncated
        or bool(all_conflicts)
    )
    semantic = bool(model_command_json) and package_complete and groups_complete and all(
        node.get("aggregation") == "model" for node in package_nodes.values()
    ) and all(group_nodes[root].get("aggregation") == "model" for root in roots)
    hierarchy = {
        "packages": package_nodes,
        "namespace_groups": group_nodes,
        "roots": roots,
        "namespace_candidates_are_heuristic": True,
    }
    result = {
        "schema_version": SCHEMA_VERSION,
        "status": "partial" if incomplete else ("complete" if semantic else "structural_only"),
        "semantic_aggregation": "model" if semantic else "not_run",
        "hierarchy": hierarchy,
        "claims": all_claims,
        "conflicts": all_conflicts,
        "uncertainties": [claim for claim in all_claims if claim.get("category") in ("possible", "inferred", "unknown")],
        "relationships": read_json(run / "relationships.json"),
        "configuration": {
            "model_command": model_command_display(model_command_json),
            "max_context_bytes": limit,
            "token_budget_policy": token_policy,
            "token_counter_command": model_command_display(json.dumps(token_counter_command_json) if isinstance(token_counter_command_json, list) else token_counter_command_json),
        },
    }
    write_json(run / "aggregate.json", result)
    return result


def namespace_hierarchy(package_ids: list[str]) -> tuple[dict[str, list[str]], list[str]]:
    """Build a deterministic import-path trie; namespace nodes are heuristic groupings."""
    package_set = set(package_ids)
    groups: dict[str, list[str]] = {}
    memo: dict[str, str | None] = {}

    def build(prefix: str) -> str | None:
        if prefix in memo:
            return memo[prefix]
        children: list[str] = []
        if prefix in package_set:
            children.append("package:" + prefix)
        next_segments = sorted({
            package_id[len(prefix) + 1:].split("/", 1)[0]
            for package_id in package_ids
            if package_id.startswith(prefix + "/")
        })
        for segment in next_segments:
            child_result = build(prefix + "/" + segment)
            if child_result:
                children.append(child_result)
        if len(children) >= 2:
            groups[prefix] = children
            memo[prefix] = prefix
        elif children:
            memo[prefix] = children[0]
        else:
            memo[prefix] = None
        return memo[prefix]

    for package_id in package_ids:
        parts = package_id.split("/")
        for index in range(1, len(parts)):
            build("/".join(parts[:index]))
    roots = sorted(
        group_id for group_id in groups
        if not any(group_id.startswith(parent + "/") for parent in groups if parent != group_id)
    )
    return groups, roots


def relevant_aggregate_relationships(run: pathlib.Path, aggregate_id: str, kind: str) -> dict[str, Any]:
    relationships = read_json(run / "relationships.json")
    inventory = read_json(run / "inventory.json")
    if kind == "package":
        members = {aggregate_id}
    else:
        members = {
            package["id"] for package in inventory.get("packages", [])
            if package["id"] == aggregate_id or package["id"].startswith(aggregate_id + "/")
        }
    imports = [
        link for link in relationships.get("package_imports", [])
        if link.get("caller_package") in members or link.get("callee_package") in members
    ]
    edges = [
        edge for edge in relationships.get("effect_site_reverse_edges", [])
        if edge.get("caller_package") in members or edge.get("callee_package") in members
    ]
    return {
        "scope": relationships.get("scope"),
        "package_imports": imports,
        "effect_site_reverse_edges": edges,
    }


def aggregate_model_call(
    run: pathlib.Path,
    aggregate_id: str,
    kind: str,
    node: dict[str, Any],
    children: list[dict[str, Any]],
    inventory: dict[str, Any],
    model_command_json: str,
    max_context_bytes: int,
    token_counter_command_json: str | list[str] | None = None,
    token_policy: dict[str, int] | None = None,
) -> dict[str, Any]:
    request = {
        "protocol": "reffector-archchar/v1",
        "phase": "aggregate",
        "system": AGGREGATE_PROMPT,
        "aggregate_id": aggregate_id,
        "kind": kind,
        "children": [
            {"id": child.get("id"), "claims": child.get("model_claims") or child.get("claims", []), "conflicts": child.get("conflicts", []), "status": child.get("status")}
            for child in children
        ],
        "relationships": relevant_aggregate_relationships(run, aggregate_id, kind),
        "evidence_scope": inventory.get("call_graph_scope"),
        "questions": ["What responsibilities and interactions are supported across these children? Preserve every contradiction and uncertainty."],
        "max_context_bytes": max_context_bytes,
    }
    size = request_size(request)
    if size > max_context_bytes:
        return {"aggregation": "over_budget", "status": "partial", "request_bytes": size, "model_claims": [], "reason": "aggregate request exceeds context-byte limit"}
    key = digest(f"aggregate:{kind}:{aggregate_id}".encode())[:24]
    response, call_path = invoke_saved_model_call(
        run, "model-calls/aggregate", key, model_command_json, request,
        token_counter_command_json=token_counter_command_json, token_policy=token_policy,
    )
    model_claims = response.get("result", {}).get("claims", []) if response.get("status") == "complete" else []
    return {
        "aggregation": "model" if response.get("status") == "complete" else "failed",
        "status": "complete" if response.get("status") == "complete" else "partial",
        "model_claims": model_claims,
        "model_call": {"artifact": str(call_path.relative_to(run)), "status": response.get("status"), "latency_seconds": response.get("latency_seconds"), "usage": response.get("usage"), "cost_usd": response.get("cost_usd"), "model": response.get("model"), "token_budget": response.get("token_budget")},
        "request_bytes": size,
    }


def index_evidence(run: pathlib.Path) -> dict[str, dict[str, Any]]:
    effects = read_json(run / "raw-effects.json")
    audit_records, _ = parse_jsonl((run / "raw-audit.jsonl").read_text(encoding="utf-8"))
    index: dict[str, dict[str, Any]] = {}
    for field in ("sites", "edges", "nodes", "groups"):
        for record in effects.get(field, []):
            if not isinstance(record, dict):
                continue
            if isinstance(record.get("id"), str):
                index[record["id"]] = {"record_type": field, "record": record}
            if field == "edges" and isinstance(record.get("source"), dict):
                source_id = record["source"].get("id")
                if isinstance(source_id, str):
                    index[source_id] = {"record_type": field, "record": record}
    for record in audit_records:
        if record.get("record_type") == "site" and isinstance(record.get("id"), str):
            index.setdefault(record["id"], {"record_type": "audit_site", "record": record})
    canonical_path = run / "evidence-index.json"
    if canonical_path.exists():
        for row in read_json(canonical_path).get("records", []):
            if not isinstance(row, dict) or not isinstance(row.get("ref"), str) or not isinstance(row.get("data"), dict):
                continue
            kind = {"effect_site": "sites", "audit_site": "audit_site", "call_edge": "edges", "node": "nodes", "effect_group": "groups", "reverse_index": "reverse", "function_audit": "function"}.get(row.get("record_type"), row.get("record_type"))
            entry = {"record_type": kind, "record": row["data"], "canonical_ref": row["ref"]}
            index[row["ref"]] = entry
            for alias in row.get("aliases", []):
                if isinstance(alias, str):
                    index.setdefault(alias, entry)
    return index


def validate_source_ref(repo: pathlib.Path, manifest: dict[str, Any], ref: dict[str, Any]) -> str | None:
    path = ref.get("file")
    if not isinstance(path, str) or pathlib.PurePosixPath(path).is_absolute() or ".." in pathlib.PurePosixPath(path).parts:
        return "invalid or non-repository source path"
    entry = manifest.get(path)
    if not isinstance(entry, dict):
        return f"source path is not in selected manifest: {path}"
    try:
        source = read_repo_file(repo, path)
        if digest(source) != entry.get("sha256"):
            return f"source changed since preparation: {path}"
        lines = source.decode("utf-8", errors="replace").splitlines()
    except (ArchcharError, OSError):
        return f"source file is unavailable or outside repository: {path}"
    line_number = ref.get("line")
    if not isinstance(line_number, int) or line_number < 1 or line_number > len(lines):
        return f"invalid source line: {path}:{line_number}"
    quote = ref.get("quote")
    if not isinstance(quote, str) or quote != lines[line_number - 1]:
        return f"source quote does not match exact line: {path}:{line_number}"
    return None


def claim_evidence_issues(claim: dict[str, Any], evidence_index: dict[str, dict[str, Any]], repo: pathlib.Path, source_manifest: dict[str, Any]) -> list[str]:
    issues: list[str] = []
    if not valid_claim_shape(claim):
        return ["claim has invalid fields or category"]
    if claim["category"] != "unknown" and not claim["evidence"]:
        issues.append("non-unknown claim has no evidence")
    referenced: list[dict[str, Any]] = []
    for ref in claim["evidence"]:
        if not isinstance(ref, dict):
            issues.append("evidence reference is not an object")
            continue
        kind = ref.get("kind")
        if kind == "source":
            issue = validate_source_ref(repo, source_manifest, ref)
            if issue:
                issues.append(issue)
        elif kind == "reffector":
            evidence_id = ref.get("id")
            entry = evidence_index.get(evidence_id) if isinstance(evidence_id, str) else None
            if not entry:
                issues.append(f"unknown Reffector evidence ID: {evidence_id}")
            else:
                referenced.append(entry)
                field = ref.get("field")
                if field not in (None, "effects", "possible_effects", "resolution", "uncertainty_reasons", "candidate_ids", "candidate_records", "source_ref", "provenance"):
                    issues.append(f"unsupported Reffector evidence field: {field}")
        else:
            issues.append(f"unsupported evidence kind: {kind}")
    statement = claim.get("statement", "").lower()
    domain = claim.get("domain")
    if domain == "runtime" or claim.get("runtime") is True:
        issues.append("static source/Reffector evidence cannot establish runtime execution")
    if domain == "effect" and claim.get("category") == "established":
        established_effect_refs = []
        candidate_edge_refs = []
        for item in referenced:
            record_type = item["record_type"]
            record = item["record"]
            if record_type in ("sites", "audit_site"):
                established = {str(effect).lower() for effect in record.get("effects", [])}
                possible = {str(effect).lower() for effect in record.get("possible_effects", [])}
                operations = record.get("operations", [])
                for operation in operations:
                    if isinstance(operation, dict):
                        established.update(str(effect).lower() for effect in operation.get("effects", []))
                        possible.update(str(effect).lower() for effect in operation.get("possible_effects", []))
                if any(effect in statement for effect in established - possible) and not any(effect in statement for effect in possible):
                    established_effect_refs.append(item)
            elif record_type == "edges":
                source = record.get("source", {})
                resolution = source.get("resolution", record.get("resolution"))
                if resolution != "established" or record.get("candidate"):
                    candidate_edge_refs.append(item)
        if candidate_edge_refs or not established_effect_refs:
            issues.append("established effect claim is supported only by candidates/possible fields or no effect-site evidence")
    if domain == "relationship" and claim.get("category") == "established":
        established_edges = [
            item for item in referenced
            if item["record_type"] == "edges"
            and item["record"].get("source", {}).get("resolution", item["record"].get("resolution")) == "established"
            and not item["record"].get("candidate")
        ]
        candidate_edges = [
            item for item in referenced
            if item["record_type"] == "edges"
            and (item["record"].get("source", {}).get("resolution", item["record"].get("resolution")) != "established" or item["record"].get("candidate"))
        ]
        if candidate_edges and not established_edges:
            issues.append("established relationship claim cites only candidate edges")
    if re.search(r"\b(always|guaranteed|will execute|executes at runtime|has completed)\b", statement) and domain != "runtime":
        issues.append("absolute/runtime wording requires runtime evidence")
    return issues


def verify_prepared_context(
    run: pathlib.Path,
    metadata: dict[str, Any],
    inventory: dict[str, Any],
    manifest: dict[str, Any],
    packets: dict[str, dict[str, Any]],
) -> list[dict[str, Any]]:
    issues: list[dict[str, Any]] = []
    source_manifest_hash = digest(canonical_json(manifest))
    if source_manifest_hash != metadata.get("source_manifest_sha256"):
        issues.append({"kind": "source_manifest_integrity", "detail": "source manifest hash differs from preparation metadata"})
    program_path = run / "program-model.json"
    evidence_path = run / "evidence-index.json"
    try:
        program = read_json(program_path)
        if digest(canonical_json(program)) != metadata.get("program_model_sha256"):
            issues.append({"kind": "program_model_integrity", "detail": "program model hash differs from preparation metadata"})
        if program.get("source_manifest_sha256") != source_manifest_hash or program.get("revision") != metadata.get("revision"):
            issues.append({"kind": "program_model_scope", "detail": "program model source or revision differs from this run"})
    except (OSError, ValueError):
        program = {}
        issues.append({"kind": "program_model_missing", "detail": "program-model.json is missing or invalid"})
    try:
        evidence = read_json(evidence_path)
        if digest(canonical_json(evidence)) != metadata.get("evidence_index_sha256"):
            issues.append({"kind": "evidence_index_integrity", "detail": "evidence index hash differs from preparation metadata"})
        if evidence.get("source_manifest_sha256") != source_manifest_hash or evidence.get("revision") != metadata.get("revision"):
            issues.append({"kind": "evidence_index_scope", "detail": "evidence index source or revision differs from this run"})
        if evidence.get("record_count") != len(evidence.get("records", [])):
            issues.append({"kind": "evidence_index_coverage", "detail": "evidence index record count does not match its records"})
    except (OSError, ValueError):
        issues.append({"kind": "evidence_index_missing", "detail": "evidence-index.json is missing or invalid"})
        evidence = {}

    expected_files = set(manifest)
    packet_files_covered: set[str] = set()
    expected_declarations = {item.get("id") for item in program.get("declarations", []) if isinstance(item, dict) and isinstance(item.get("id"), str)}
    packet_declarations: set[str] = set()
    repo = pathlib.Path(metadata.get("repo", "."))
    for unit_id, packet in packets.items():
        declared = packet.get("source_files", [])
        packet_files_covered.update(path for path in declared if isinstance(path, str))
        request = packet.get("request", {})
        packet_declarations.update(
            item.get("id") for item in request.get("program", {}).get("declarations", [])
            if isinstance(item, dict) and isinstance(item.get("id"), str)
        )
        source_paths = set()
        for source in request.get("source", []):
            path = source.get("path")
            if not isinstance(path, str) or path not in manifest:
                issues.append({"kind": "packet_source_scope", "unit": unit_id, "detail": f"packet source is outside the selected manifest: {path}"})
                continue
            source_paths.add(path)
            try:
                data = read_repo_file(repo, path)
            except (ArchcharError, OSError):
                issues.append({"kind": "packet_source_missing", "unit": unit_id, "detail": path})
                continue
            if digest(data) != manifest[path].get("sha256") or source.get("sha256") != manifest[path].get("sha256"):
                issues.append({"kind": "packet_source_hash", "unit": unit_id, "detail": path})
                continue
            text = source.get("text")
            if not isinstance(text, str):
                issues.append({"kind": "packet_source_fragment", "unit": unit_id, "detail": f"source text is missing for {path}"})
                continue
            line_start, line_end = source.get("line_start"), source.get("line_end")
            if line_start is None and line_end is None:
                if text != data.decode("utf-8", errors="replace"):
                    issues.append({"kind": "packet_source_fragment", "unit": unit_id, "detail": f"full source differs from manifest for {path}"})
            elif not isinstance(line_start, int) or isinstance(line_start, bool) or not isinstance(line_end, int) or isinstance(line_end, bool) or line_start < 1 or line_end < line_start:
                issues.append({"kind": "packet_source_fragment", "unit": unit_id, "detail": f"invalid source line range for {path}"})
            else:
                lines = data.decode("utf-8", errors="replace").splitlines(keepends=True)
                if line_end > len(lines) or text != "".join(lines[line_start - 1:line_end]):
                    issues.append({"kind": "packet_source_fragment", "unit": unit_id, "detail": f"source fragment does not match {path}:{line_start}-{line_end}"})
        if source_paths != set(declared):
            issues.append({"kind": "packet_source_coverage", "unit": unit_id, "detail": {"declared": sorted(set(declared)), "supplied": sorted(source_paths)}})
        budget = packet.get("token_budget")
        if packet.get("status") == "pending":
            if not isinstance(budget, dict) or budget.get("status") != "within_budget":
                issues.append({"kind": "packet_token_budget", "unit": unit_id, "detail": "pending generation packet lacks a successful token preflight"})
            elif budget.get("total_reserved_tokens") != budget.get("input_tokens", -1) + budget.get("output_reserved_tokens", -1) + budget.get("safety_margin_tokens", -1) or budget.get("total_reserved_tokens", 0) > budget.get("context_window_tokens", 0):
                issues.append({"kind": "packet_token_budget", "unit": unit_id, "detail": "token reserve arithmetic is invalid or exceeds context"})
    if packet_files_covered != expected_files:
        issues.append({"kind": "unit_file_coverage", "detail": {"missing": sorted(expected_files - packet_files_covered), "extra": sorted(packet_files_covered - expected_files)}})
    if packet_declarations != expected_declarations:
        issues.append({"kind": "unit_declaration_coverage", "detail": {"missing": sorted(expected_declarations - packet_declarations), "extra": sorted(packet_declarations - expected_declarations)}})
    if inventory.get("unit_coverage", {}).get("source_files") != len(expected_files):
        issues.append({"kind": "inventory_source_coverage", "detail": "inventory unit coverage count differs from selected source manifest"})
    for path, info in sorted(manifest.items()):
        try:
            if digest(read_repo_file(repo, path)) != info.get("sha256"):
                issues.append({"kind": "source_scope_changed", "detail": path})
        except (ArchcharError, OSError):
            issues.append({"kind": "source_scope_changed", "detail": path})
    return issues


def verify(
    run_path: str, model_command_json: str | None = None, max_context_bytes: int | None = None,
    token_counter_command_json: str | list[str] | None = None,
    token_policy: dict[str, int] | None = None,
) -> dict[str, Any]:
    run = pathlib.Path(run_path).resolve()
    metadata = read_json(run / "metadata.json")
    inventory = read_json(run / "inventory.json")
    manifest = read_json(run / "source-manifest.json")["files"]
    evidence_index = index_evidence(run)
    token_policy = token_policy or metadata.get("token_budget_policy")
    repo = pathlib.Path(metadata["repo"])
    issues: list[dict[str, Any]] = []
    if metadata.get("status") != "complete":
        issues.append({"kind": "input_coverage", "detail": "preparation phase is not complete"})
    if truncation_present(inventory.get("truncated")):
        issues.append({"kind": "truncated", "detail": inventory.get("truncated")})
    if inventory.get("coverage", {}).get("omitted", 0):
        issues.append({"kind": "omitted_functions", "detail": inventory["coverage"].get("omitted")})
    packets = {read_json(path)["unit_id"]: read_json(path) for path in packet_files(run)}
    results = {read_json(path).get("unit_id"): read_json(path) for path in sorted((run / "components").glob("*.json"))}
    issues.extend(verify_prepared_context(run, metadata, inventory, manifest, packets))
    all_claims: list[tuple[str, dict[str, Any], str]] = []
    for unit_id, packet in packets.items():
        result = results.get(unit_id)
        if result is None or result.get("status") != "complete":
            issues.append({"kind": "incomplete_unit", "unit": unit_id, "detail": result.get("status") if result else "missing"})
        if packet["request_bytes"] != request_size(packet.get("request", {})):
            issues.append({"kind": "packet_request_integrity", "unit": unit_id, "detail": "saved request byte count differs from serialized request"})
        if packet.get("request", {}).get("unit_id") != unit_id:
            issues.append({"kind": "packet_identity", "unit": unit_id, "detail": "packet and request unit IDs differ"})
        if packet["request_bytes"] > packet["max_context_bytes"]:
            issues.append({"kind": "context_limit", "unit": unit_id, "detail": packet["request_bytes"]})
        if packet["status"] == "over_budget":
            issues.append({"kind": "over_budget", "unit": unit_id, "detail": packet.get("reason")})
        for claim in (result or {}).get("claims", []):
            all_claims.append((unit_id, claim, "component"))
    aggregate_path = run / "aggregate.json"
    aggregate_result = read_json(aggregate_path) if aggregate_path.exists() else None
    if not aggregate_result:
        issues.append({"kind": "missing_aggregation", "detail": "aggregate phase has not run"})
    elif aggregate_result.get("status") not in ("complete", "structural_only"):
        issues.append({"kind": "partial_aggregation", "detail": aggregate_result.get("status")})
    if aggregate_result:
        for package_id, package in aggregate_result.get("hierarchy", {}).get("packages", {}).items():
            known: dict[str, list[dict[str, Any]]] = defaultdict(list)
            for unit_id, child_claim, _ in all_claims:
                if unit_id.startswith(package_id + "::") and isinstance(child_claim.get("id"), str):
                    known[child_claim["id"]].append(child_claim)
            for claim in package.get("model_claims", []):
                all_claims.append(("package:" + package_id, claim, "aggregate"))
                children = claim.get("child_claim_ids", [])
                if not children:
                    issues.append({"kind": "aggregation_ancestry", "claim": claim.get("id"), "detail": "aggregate claim has no child_claim_ids"})
                child_records = []
                for child in children:
                    matches = known.get(str(child), [])
                    if not matches:
                        issues.append({"kind": "aggregation_ancestry", "claim": claim.get("id"), "detail": f"unknown package child claim {child}"})
                    child_records.extend(matches)
                categories = [record.get("category") for record in child_records]
                if claim.get("category") == "established" and (not categories or any(category != "established" for category in categories)):
                    issues.append({"kind": "confidence_upgrade", "claim": claim.get("id"), "detail": "aggregate confidence exceeds at least one cited child"})
        for namespace, group in aggregate_result.get("hierarchy", {}).get("namespace_groups", {}).items():
            child_claim_map: dict[str, list[dict[str, Any]]] = defaultdict(list)
            for child in group.get("children", []):
                if child.startswith("package:"):
                    child_node = aggregate_result["hierarchy"]["packages"].get(child[len("package:"):], {})
                else:
                    child_node = aggregate_result["hierarchy"]["namespace_groups"].get(child, {})
                child_claims = child_node.get("model_claims") or child_node.get("claims", [])
                for child_claim in child_claims:
                    if isinstance(child_claim.get("id"), str):
                        child_claim_map[child_claim["id"]].append(child_claim)
            for claim in group.get("model_claims", []):
                all_claims.append(("namespace:" + namespace, claim, "aggregate"))
                children = claim.get("child_claim_ids", [])
                if not children:
                    issues.append({"kind": "aggregation_ancestry", "claim": claim.get("id"), "detail": "aggregate claim has no child_claim_ids"})
                child_records = []
                for child in children:
                    matches = child_claim_map.get(str(child), [])
                    if not matches:
                        issues.append({"kind": "aggregation_ancestry", "claim": claim.get("id"), "detail": f"unknown group child claim {child}"})
                    child_records.extend(matches)
                categories = [record.get("category") for record in child_records]
                if claim.get("category") == "established" and (not categories or any(category != "established" for category in categories)):
                    issues.append({"kind": "confidence_upgrade", "claim": claim.get("id"), "detail": "namespace aggregate confidence exceeds at least one cited child"})

    call_paths = list((run / "model-calls").rglob("*.json")) if (run / "model-calls").exists() else []
    call_paths.extend((run / "benchmark-calls").glob("*.json") if (run / "benchmark-calls").exists() else [])
    for call_path in call_paths:
        try:
            call = read_json(call_path)
        except (OSError, ValueError):
            issues.append({"kind": "model_call_artifact", "detail": str(call_path.relative_to(run))})
            continue
        if call.get("status") != "complete":
            continue
        budget = call.get("token_budget")
        if not isinstance(budget, dict) or budget.get("status") != "within_budget":
            issues.append({"kind": "model_call_token_budget", "detail": str(call_path.relative_to(run))})
            continue
        if budget.get("request_sha256") != call.get("request_sha256"):
            issues.append({"kind": "model_call_token_count_binding", "detail": str(call_path.relative_to(run))})
        if call.get("model") != budget.get("model"):
            issues.append({"kind": "model_call_model_mismatch", "detail": str(call_path.relative_to(run))})
        total = budget.get("input_tokens", -1) + budget.get("output_reserved_tokens", -1) + budget.get("safety_margin_tokens", -1)
        if total != budget.get("total_reserved_tokens") or total > budget.get("context_window_tokens", 0):
            issues.append({"kind": "model_call_token_arithmetic", "detail": str(call_path.relative_to(run))})

    claim_results: list[dict[str, Any]] = []
    for owner, claim, phase in all_claims:
        errors = claim_evidence_issues(claim, evidence_index, repo, manifest) if isinstance(claim, dict) else ["claim is not an object"]
        for error in errors:
            issues.append({"kind": "claim_evidence", "owner": owner, "claim": claim.get("id") if isinstance(claim, dict) else None, "detail": error})
        claim_results.append({"owner": owner, "claim_id": claim.get("id") if isinstance(claim, dict) else None, "valid": not errors, "issues": errors, "phase": phase})

    model_checks: list[dict[str, Any]] = []
    if model_command_json:
        limit = max_context_bytes or metadata["source_configuration"]["max_context_bytes"]
        deterministic_rejections = {item.get("claim_id") for item in claim_results if not item.get("valid")}
        deterministic_rejections.update(issue.get("claim") for issue in issues if issue.get("kind") in {"aggregation_ancestry", "confidence_upgrade"})
        for owner, claim, _phase in all_claims:
            if not isinstance(claim, dict):
                continue
            if claim.get("id") in deterministic_rejections:
                model_checks.append({"claim_id": claim.get("id"), "status": "skipped_deterministic_rejection"})
                continue
            excerpts = []
            records = []
            for ref in claim.get("evidence", []):
                if ref.get("kind") == "source":
                    try:
                        lines = read_repo_file(repo, ref.get("file", "")).decode("utf-8", errors="replace").splitlines()
                        line = int(ref.get("line", 0))
                        excerpts.append({"file": ref["file"], "line": line, "text": lines[line - 1] if 1 <= line <= len(lines) else ""})
                    except (ArchcharError, OSError, ValueError, KeyError):
                        pass
                elif ref.get("kind") == "reffector" and ref.get("id") in evidence_index:
                    records.append({"id": ref["id"], **evidence_index[ref["id"]]})
            request = {"protocol": "reffector-archchar/v1", "phase": "verify", "system": VERIFY_PROMPT, "claim": claim, "source_excerpts": excerpts, "reffector_records": records}
            if request_size(request) > limit:
                model_checks.append({"claim_id": claim.get("id"), "status": "over_budget"})
                issues.append({"kind": "model_verification", "claim": claim.get("id"), "detail": "verifier request exceeds context-byte limit"})
                continue
            key = digest(f"verify:{owner}:{claim.get('id')}".encode())[:24]
            response, call_path = invoke_saved_model_call(
                run, "model-calls/verify", key, model_command_json, request, response_kind="verdicts",
                token_counter_command_json=token_counter_command_json, token_policy=token_policy,
            )
            model_checks.append({"claim_id": claim.get("id"), "artifact": str(call_path.relative_to(run)), "status": response["status"], "result": response.get("result"), "latency_seconds": response.get("latency_seconds"), "usage": response.get("usage"), "cost_usd": response.get("cost_usd"), "token_budget": response.get("token_budget")})
            if response["status"] != "complete":
                issues.append({"kind": "model_verification", "claim": claim.get("id"), "detail": "independent verifier call failed"})
            else:
                verdicts = [verdict for verdict in response["result"]["verdicts"] if verdict["claim_id"] == claim.get("id")]
                if len(verdicts) != 1 or verdicts[0]["verdict"] != "supported":
                    issues.append({"kind": "model_verification", "claim": claim.get("id"), "detail": "independent verifier did not support the claim"})

    report = {
        "schema_version": SCHEMA_VERSION,
        "status": "complete" if not issues else "partial",
        "level": "deterministic citation/confidence checks" + (" plus isolated model verdicts" if model_command_json else "; semantic correctness is not established"),
        "issues": issues,
        "claims": claim_results,
        "model_checks": model_checks,
        "coverage": {"source_files": len(manifest), "analysis_units": len(packets), "claims": len(all_claims), "unmatched_references": len(inventory.get("unmatched_references", []))},
        "effects_truncated": inventory.get("truncated"),
    }
    write_json(run / "verification.json", report)
    return report


def metrics(records: list[dict[str, Any]]) -> dict[str, Any]:
    costs = [record.get("cost_usd") for record in records]
    latencies = [record.get("latency_seconds") for record in records]
    usages = [record.get("usage") for record in records]
    def total(field: str) -> int | None:
        values = [usage.get(field) if isinstance(usage, dict) else None for usage in usages]
        if not records or any(not isinstance(value, int) or isinstance(value, bool) or value < 0 for value in values):
            return None
        return sum(values)
    return {
        "calls": len(records),
        "cost_usd": sum(costs) if records and all(isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value) and value >= 0 for value in costs) else None,
        "known_cost_usd_sum": sum(value for value in costs if isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value) and value >= 0),
        "cost_known_calls": sum(isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value) and value >= 0 for value in costs),
        "input_tokens": total("input_tokens"),
        "output_tokens": total("output_tokens"),
        "usage_known_calls": sum(
            isinstance(usage, dict)
            and all(isinstance(usage.get(field), int) and not isinstance(usage.get(field), bool) and usage[field] >= 0 for field in ("input_tokens", "output_tokens"))
            for usage in usages
        ),
        "latency_seconds": sum(value for value in latencies if isinstance(value, (int, float)) and math.isfinite(value) and value >= 0)
        if records and all(isinstance(value, (int, float)) and math.isfinite(value) and value >= 0 for value in latencies)
        else None,
    }


def model_call_records(run: pathlib.Path, include_phases: set[str] | None = None) -> list[dict[str, Any]]:
    directory = run / "model-calls"
    if not directory.exists():
        return []
    result = []
    for path in sorted(directory.rglob("*.json")):
        try:
            record = read_json(path)
        except (OSError, ValueError):
            continue
        phase = record.get("request", {}).get("phase")
        if include_phases is None or phase in include_phases:
            result.append(record)
    return result


def load_manifest(path: str | pathlib.Path) -> dict[str, Any]:
    manifest = read_json(pathlib.Path(path))
    if not isinstance(manifest, dict) or manifest.get("schema_version") != 1:
        raise ArchcharError("benchmark manifest schema_version must be 1")
    questions = manifest.get("questions")
    if not isinstance(questions, list) or not questions or any(
        not isinstance(question, dict)
        or not isinstance(question.get("id"), str)
        or not question["id"]
        or not isinstance(question.get("prompt"), str)
        or not question["prompt"]
        for question in questions
    ):
        raise ArchcharError("benchmark manifest must contain questions with nonempty id and prompt")
    rubric = manifest.get("rubric")
    if not isinstance(rubric, dict) or not isinstance(rubric.get("required_facts"), list) or not rubric["required_facts"]:
        raise ArchcharError("benchmark manifest must contain nonempty rubric.required_facts")
    fact_ids: set[str] = set()
    for fact in rubric["required_facts"]:
        if not isinstance(fact, dict) or not isinstance(fact.get("id"), str) or not fact["id"]:
            raise ArchcharError("each rubric fact requires a nonempty id")
        if fact["id"] in fact_ids:
            raise ArchcharError(f"duplicate rubric fact id: {fact['id']}")
        fact_ids.add(fact["id"])
        for field in ("all_terms", "any_terms"):
            terms = fact.get(field, [])
            if not isinstance(terms, list) or not all(isinstance(term, str) and term for term in terms):
                raise ArchcharError(f"rubric fact {fact['id']} field {field} must contain nonempty strings")
        if not fact.get("all_terms") and not fact.get("any_terms"):
            raise ArchcharError(f"rubric fact {fact.get('id')} needs all_terms or any_terms")
        if "source_refs" in fact:
            raise ArchcharError(f"rubric fact {fact['id']} uses source_refs; use citation_options")
        if "citation_options" in fact:
            options = fact["citation_options"]
            if not isinstance(options, list) or not options:
                raise ArchcharError(f"rubric fact {fact['id']} citation_options must be a nonempty list")
            for option in options:
                if not isinstance(option, dict):
                    raise ArchcharError(f"rubric fact {fact['id']} has an invalid citation option")
                if option.get("kind") == "source":
                    path = option.get("file")
                    line = option.get("line")
                    pure_path = pathlib.PurePosixPath(path) if isinstance(path, str) else None
                    if pure_path is None or pure_path.is_absolute() or ".." in pure_path.parts or not isinstance(line, int) or isinstance(line, bool) or line < 1:
                        raise ArchcharError(f"rubric fact {fact['id']} has an invalid source citation option")
                elif option.get("kind") == "reffector":
                    record_types = option.get("record_types")
                    allowed_types = {"sites", "audit_site", "edges", "nodes", "groups"}
                    if not isinstance(record_types, list) or not record_types or not all(isinstance(item, str) and item in allowed_types for item in record_types):
                        raise ArchcharError(f"rubric fact {fact['id']} has invalid Reffector citation record types")
                else:
                    raise ArchcharError(f"rubric fact {fact['id']} citation kind must be source or reffector")
    if "patterns" in manifest and (not isinstance(manifest["patterns"], list) or not all(isinstance(item, str) and item for item in manifest["patterns"])):
        raise ArchcharError("manifest patterns must be a list of nonempty strings")
    if "expected_packages" in manifest and (not isinstance(manifest["expected_packages"], list) or not all(isinstance(item, str) and item for item in manifest["expected_packages"])):
        raise ArchcharError("expected_packages must be a list of nonempty strings")
    if "revision" in manifest and (not isinstance(manifest["revision"], str) or not re.fullmatch(r"[a-fA-F0-9]{40,64}", manifest["revision"])):
        raise ArchcharError("manifest revision must be a full hexadecimal Git object ID")
    for field in ("expected_source_files", "expected_source_bytes", "expected_source_lines"):
        if field in manifest and (not isinstance(manifest[field], int) or isinstance(manifest[field], bool) or manifest[field] < 0):
            raise ArchcharError(f"{field} must be a nonnegative integer")
    return manifest


def claim_matches_citation_option(
    claim: dict[str, Any],
    options: list[dict[str, Any]],
    evidence_index: dict[str, dict[str, Any]],
    repo: pathlib.Path,
    source_manifest: dict[str, Any],
) -> bool:
    for option in options:
        for ref in claim.get("evidence", []):
            if not isinstance(ref, dict):
                continue
            if option["kind"] == "source" and ref.get("kind") == "source":
                if ref.get("file") == option["file"] and ref.get("line") == option["line"] and validate_source_ref(repo, source_manifest, ref) is None:
                    return True
            elif option["kind"] == "reffector" and ref.get("kind") == "reffector":
                item = evidence_index.get(ref.get("id"))
                if item and item.get("record_type") in option["record_types"]:
                    return True
    return False


def claim_score(claims: list[dict[str, Any]], rubric: dict[str, Any], evidence_index: dict[str, dict[str, Any]], repo: pathlib.Path, source_manifest: dict[str, Any]) -> dict[str, Any]:
    fact_results = []
    texts = [(claim, str(claim.get("statement", "")).casefold()) for claim in claims if isinstance(claim, dict)]
    for fact in rubric["required_facts"]:
        all_terms = [str(term).casefold() for term in fact.get("all_terms", [])]
        any_terms = [str(term).casefold() for term in fact.get("any_terms", [])]
        options = fact.get("citation_options", [])
        matching = [
            claim.get("id")
            for claim, statement in texts
            if all(term in statement for term in all_terms)
            and (not any_terms or any(term in statement for term in any_terms))
            and not claim_evidence_issues(claim, evidence_index, repo, source_manifest)
            and (not options or claim_matches_citation_option(claim, options, evidence_index, repo, source_manifest))
        ]
        fact_results.append({"id": fact["id"], "hit": bool(matching), "matching_claim_ids": matching})
    valid_claims = 0
    promotion_issues: list[dict[str, Any]] = []
    for claim in claims:
        if not isinstance(claim, dict):
            continue
        errors = claim_evidence_issues(claim, evidence_index, repo, source_manifest)
        if not errors:
            valid_claims += 1
        for error in errors:
            if "candidate" in error or "possible" in error or "runtime" in error or "established" in error:
                promotion_issues.append({"claim_id": claim.get("id"), "issue": error})
    denominator = len(rubric["required_facts"])
    return {
        "required_facts": fact_results,
        "lexical_fact_recall": sum(fact["hit"] for fact in fact_results) / denominator if denominator else None,
        "valid_citation_ratio": valid_claims / len(claims) if claims else None,
        "valid_claims": valid_claims,
        "claim_count": len(claims),
        "confidence_or_runtime_issues": promotion_issues,
        "semantic_accuracy_scored": False,
    }


def compact_evidence_record(record_type: str, record: dict[str, Any], repo: pathlib.Path) -> dict[str, Any]:
    if record_type in ("sites", "audit_site"):
        location = record.get("location", {})
        candidates = record.get("vta_candidates", [])
        return {
            "record_type": record_type,
            "id": record.get("id"),
            "package": record.get("package"),
            "function": record.get("function"),
            "kind": record.get("kind"),
            "file": relative_source_path(repo, location.get("file")),
            "line": location.get("line"),
            "effects": record.get("effects", []),
            "possible_effects": record.get("possible_effects", []),
            "uncertain": record.get("uncertain", False),
            "candidate_count": len(candidates) if isinstance(candidates, list) else 0,
            "candidate_ids": [item.get("id") for item in candidates[:20] if isinstance(item, dict)],
            "candidate_ids_truncated": len(candidates) > 20 if isinstance(candidates, list) else False,
        }
    if record_type == "edges":
        return {
            "record_type": record_type,
            "id": record.get("id"),
            "caller_package": record.get("caller_package"),
            "callee_package": record.get("callee_package"),
            "file": relative_source_path(repo, record.get("file")),
            "line": (record.get("location") or {}).get("line"),
            "kind": record.get("kind"),
            "resolution": record.get("resolution"),
            "candidate": record.get("candidate", False),
            "synthetic": record.get("synthetic", False),
        }
    return {
        "record_type": record_type,
        "id": record.get("id"),
        "package": record.get("package"),
        "function": record.get("function"),
        "effects": record.get("effects", []),
        "possible_effects": record.get("possible_effects", []),
    }


def benchmark_evidence(run: pathlib.Path, inventory: dict[str, Any], selected_files: set[str]) -> dict[str, Any]:
    selected_packages = {package["id"] for package in inventory.get("packages", [])}
    index = read_json(run / "evidence-index.json")
    rows = index.get("records", [])
    raw_unresolved_count = len(read_json(run / "raw-effects.json").get("unresolved_sites", []))
    sites = [row for row in rows if row.get("record_type") in {"effect_site", "audit_site"} and row.get("file") in selected_files and row.get("package") in selected_packages]
    functions = [row for row in rows if row.get("record_type") == "function_audit" and row.get("package") in selected_packages and any(location.get("file") in selected_files for location in row.get("data", {}).get("source_locations", []) if isinstance(location, dict))]
    unresolved_rows = [row for row in rows if row.get("record_type") == "unresolved_site" and (row.get("file") in selected_files or (row.get("file") is None and row.get("package") is None))]
    edges = [row for row in rows if row.get("record_type") == "call_edge" and (row.get("file") in selected_files or (row.get("data", {}).get("caller_package") in selected_packages and row.get("data", {}).get("callee_package") in selected_packages))]
    imports = [
        link for link in read_json(run / "relationships.json").get("package_imports", [])
        if link.get("caller_package") in selected_packages and link.get("callee_package") in selected_packages
    ]
    audit_counts: Counter[str] = Counter()
    for row in rows:
        if row.get("record_type") in {"effect_site", "audit_site", "function_audit"} and row.get("package") in selected_packages and row.get("file") in selected_files:
            audit_counts[str(row.get("record_type"))] += 1

    referenced: set[str] = set()
    for path in sorted((run / "components").glob("*.json")):
        result = read_json(path)
        for claim in result.get("claims", []):
            for ref in claim.get("evidence", []):
                if isinstance(ref, dict) and ref.get("kind") == "reffector" and isinstance(ref.get("id"), str):
                    referenced.add(ref["id"])
    aggregate_path = run / "aggregate.json"
    if aggregate_path.exists():
        aggregate_result = read_json(aggregate_path)
        for claim in aggregate_result.get("claims", []):
            for ref in claim.get("evidence", []):
                if isinstance(ref, dict) and ref.get("kind") == "reffector" and isinstance(ref.get("id"), str):
                    referenced.add(ref["id"])
    by_ref = {row.get("ref"): row for row in rows if isinstance(row.get("ref"), str)}
    by_alias = {alias: row for row in rows for alias in row.get("aliases", []) if isinstance(alias, str)}
    referenced_records = []
    missing_ids = []
    for identifier in sorted(referenced):
        row = by_ref.get(identifier) or by_alias.get(identifier)
        if row is None:
            missing_ids.append(identifier)
            continue
        data = row.get("data", {})
        referenced_records.append({
            "id": identifier, "record_type": row.get("record_type"),
            "package": row.get("package"), "file": row.get("file"),
            "source_ref": row.get("source_ref"),
            "data": data,
            "relationship_details_included": row.get("record_type") == "call_edge",
        })
    site_effects: Counter[str] = Counter()
    possible_effects: Counter[str] = Counter()
    for row in sites:
        data = row.get("data", {})
        for effect in data.get("effects", []):
            site_effects[str(effect)] += 1
        for effect in data.get("possible_effects", []):
            possible_effects[str(effect)] += 1
    candidate_edges = [row for row in edges if row.get("data", {}).get("candidate") or row.get("data", {}).get("resolution") != "established"]
    reverse = [row for row in rows if row.get("record_type") == "reverse_index" and row.get("data", {}).get("site") in {item.get("data", {}).get("id") for item in sites}]
    effect_site_ids = {row.get("data", {}).get("id") for row in sites if row.get("record_type") == "effect_site"}
    site_evidence_details_deferred = sum(
        row.get("record_type") != "audit_site" or row.get("data", {}).get("id") not in effect_site_ids
        for row in sites
    )
    uncertainty_site_details_deferred = sum(
        row.get("record_type") == "audit_site" and row.get("data", {}).get("id") not in effect_site_ids
        for row in sites
    )
    return {
        "scope": "claim-referenced source-scoped evidence summaries; canonical evidence-index.json retains every full record and stable reference",
        "graph_scope": "effect-site reverse relationships are not a complete call graph and do not establish runtime execution",
        "effect_site_count": sum(row.get("record_type") == "effect_site" for row in sites),
        "audit_site_count": sum(row.get("record_type") == "audit_site" for row in sites),
        "function_audit_count": len(functions),
        "established_site_effect_counts": dict(sorted(site_effects.items())),
        "possible_site_effect_counts": dict(sorted(possible_effects.items())),
        "audit_record_counts": dict(sorted(audit_counts.items())),
        "site_evidence_details_deferred": site_evidence_details_deferred,
        "uncertainty_site_details_deferred": uncertainty_site_details_deferred,
        "unresolved_site_export_count": inventory.get("coverage", {}).get("unresolved_sites_identified"),
        "unresolved_site_export_rows": raw_unresolved_count,
        "unresolved_site_records_in_scope": [
            {"ref": row.get("ref"), "id": row.get("data", {}).get("id"),
             "package": row.get("package"), "file": row.get("file"),
             "source_ref": row.get("source_ref"), "raw_ref": row.get("data", {}).get("raw_ref")}
            for row in unresolved_rows
        ],
        "edge_count": len(edges),
        "candidate_relationship_count": len(candidate_edges),
        "cross_package_relationship_count": sum(row.get("data", {}).get("caller_package") != row.get("data", {}).get("callee_package") for row in edges),
        "candidate_relationship_refs": [row.get("ref") for row in candidate_edges],
        "cross_package_relationship_refs": [
            row.get("ref") for row in edges if row.get("data", {}).get("caller_package") != row.get("data", {}).get("callee_package")
        ],
        "reverse_impact_summary_count": len(reverse),
        "reverse_impact_truncated_count": sum(bool(row.get("data", {}).get("truncated")) for row in reverse),
        "package_imports": imports,
        "referenced_records": referenced_records,
        "referenced_record_count": len(referenced_records),
        "omitted_referenced_record_count": 0,
        "missing_referenced_ids": missing_ids,
        "evidence_index_record_count": index.get("record_count"),
        "deferred_evidence_ref_count": max(0, len(rows) - len(referenced_records)),
        "coverage": inventory.get("coverage", {}),
        "truncated": inventory.get("truncated", {}),
    }


def compact_one_shot_evidence(index: dict[str, Any], inventory: dict[str, Any], selected_files: set[str], selected_refs: set[str] | None = None) -> dict[str, Any]:
    selected_packages = {package["id"] for package in inventory.get("packages", [])}
    grouped: dict[tuple[str | None, str | None], dict[str, list[tuple[str, Any, dict[str, Any]]]]] = defaultdict(lambda: defaultdict(list))
    selected_rows = []
    for row in index.get("records", []):
        if selected_refs is not None and row.get("ref") not in selected_refs:
            continue
        kind = row.get("record_type")
        data = row.get("data", {})
        if kind == "call_edge":
            relevant = row.get("file") in selected_files or data.get("caller_package") in selected_packages or data.get("callee_package") in selected_packages
        else:
            relevant = row.get("file") in selected_files
        if relevant and kind in {"effect_site", "audit_site", "call_edge", "unresolved_site"}:
            selected_rows.append(row)

    fields: dict[str, set[str]] = defaultdict(set)
    constants: dict[str, dict[str, Any]] = {}
    for kind in sorted({row["record_type"] for row in selected_rows}):
        rows = [row for row in selected_rows if row["record_type"] == kind]
        shared: dict[str, Any] = {}
        for key in set.intersection(*(set(row.get("data", {})) for row in rows)) if rows else set():
            if key == "provenance" and all(row["data"].get(key) == rows[0]["data"].get(key) for row in rows):
                shared[key] = rows[0]["data"][key]
        constants[kind] = shared
        for row in rows:
            data = row.get("data", {})
            fields[kind].update(key for key in data if key not in {"id", "package"} and key not in shared)

    for row in selected_rows:
        kind = row["record_type"]
        data = row.get("data", {})
        field_order = sorted(fields[kind])
        values = []
        for key_name in field_order:
            value = data.get(key_name)
            if key_name in {"source_ref", "location"} and isinstance(value, dict) and set(value) == {"file", "line", "column"}:
                source_path = value.get("file")
                if source_path == row.get("file"):
                    value = [value.get("line"), value.get("column")]
                else:
                    value = [source_path, value.get("line"), value.get("column")]
            values.append(value)
        package_marker = "=" if "package" in data and data.get("package") == row.get("package") else data.get("package")
        ref_prefix = {"effect_site": "effect-site:", "audit_site": "audit-site:", "call_edge": "call-edge:", "unresolved_site": "unresolved-site:"}.get(kind, "")
        ref_suffix = str(row.get("ref", "")).removeprefix(ref_prefix) if ref_prefix else None
        id_marker = None if "id" not in data else ("=" if data.get("id") == ref_suffix else data.get("id"))
        key = (row.get("file"), row.get("package"))
        presence_mask = "".join("1" if field in data else "0" for field in field_order)
        grouped[key][kind].append((row.get("ref"), package_marker, id_marker, presence_mask, dict(zip(field_order, values))))

    nested_fields: set[str] = set()
    repeated_strings: Counter[str] = Counter()
    def collect_nested_keys(value: Any) -> None:
        if isinstance(value, dict):
            nested_fields.update(str(key) for key in value)
            for child in value.values():
                collect_nested_keys(child)
        elif isinstance(value, list):
            for child in value:
                collect_nested_keys(child)
        elif isinstance(value, str):
            repeated_strings[value] += 1
    for type_groups in grouped.values():
        for rows in type_groups.values():
            for _, _, _, _, record in rows:
                collect_nested_keys(record)
    nested_field_order = sorted(nested_fields)
    nested_key_index = {key: index for index, key in enumerate(nested_field_order)}
    string_pool = sorted(value for value, count in repeated_strings.items() if count >= 3 and len(value) >= 12)
    string_index = {value: index for index, value in enumerate(string_pool)}

    def encode_nested(value: Any) -> Any:
        if isinstance(value, str) and value in string_index:
            return ["@s", string_index[value]]
        if isinstance(value, dict):
            pairs = []
            for key, child in value.items():
                pairs.extend((nested_key_index[str(key)], encode_nested(child)))
            return ["@o", *pairs]
        if isinstance(value, list):
            encoded = [encode_nested(child) for child in value]
            return ["@a", *encoded] if encoded and isinstance(encoded[0], str) and encoded[0] in {"@o", "@a", "@s"} else encoded
        return value

    by_file = []
    for (file_path, package), type_groups in sorted(grouped.items(), key=lambda item: (item[0][0] or "", item[0][1] or "")):
        by_file.append({"file": file_path, "package": package, **{
            kind: [[ref, package_marker, id_marker, presence_mask,
                   [encode_nested(record.get(field)) for field in sorted(fields[kind])]]
                   for ref, package_marker, id_marker, presence_mask, record in rows]
            for kind, rows in sorted(type_groups.items())
        }})
    return {
        "schema": "reffector-archchar/one-shot-evidence-v1",
        "scope": "selected effect/audit sites and selected/boundary call edges; canonical evidence-index.json remains unchanged",
        "field_order": {kind: sorted(values) for kind, values in sorted(fields.items())},
        "tuple_field_order": ["ref", "data_package_marker", "data_id_marker(= means ref suffix, null means absent, otherwise literal id)", "field_presence_bitset", "values_by_field_order"],
        "group_field_order": ["file", "package", "typed_records"],
        "record_id": "data.id is encoded as a per-record marker: ref suffix, absent, or literal id",
        "package": "data.package is restored from the marker: = means group package; null means absent; other string is preserved",
        "location_encoding": {"source_ref": "[line,column] when file equals group file, else [file,line,column]", "location": "[line,column] when file equals group file, else [file,line,column]"},
        "constants": constants,
        "records_by_file": by_file,
        "nested_field_order": nested_field_order,
        "string_pool": string_pool,
        "nested_object_encoding": "dictionary=[@o,field_index,value,...]; repeated strings=[@s,string_index]; ordinary arrays recurse and arrays beginning with @o/@a/@s are escaped as [@a,...]",
        "record_counts": dict(sorted(Counter(row["record_type"] for row in selected_rows).items())),
        "coverage": inventory.get("coverage", {}),
        "truncated": inventory.get("truncated", {}),
    }


def benchmark_treatment_context(run: pathlib.Path, inventory: dict[str, Any], selected_files: set[str], one_shot: bool = False, questions: list[dict[str, Any]] | None = None, selection_budget_bytes: int = 75000) -> dict[str, Any]:
    if one_shot:
        from .selection import retrieve_evidence_records, select_context
        index = read_json(run / "evidence-index.json")
        if not questions:
            return {"reffector": compact_one_shot_evidence(index, inventory, selected_files)}
        source_index = read_json(run / "program-model.json")
        target = {"id": "rclone-vfs-cache-subsystem", "packages": [package["id"] for package in inventory.get("packages", [])], "files": sorted(selected_files)}
        question_selections = [select_context(question, target, index, source_index, selection_budget_bytes) for question in questions]
        selected_refs = sorted({item["ref"] for result in question_selections for item in result["selected"]})
        revision = read_json(run / "metadata.json").get("revision")
        retrieval = retrieve_evidence_records(index, selected_refs, revision, selection_budget_bytes * max(1, len(questions)))
        return {"reffector": compact_one_shot_evidence(index, inventory, selected_files, set(selected_refs)),
                "selection": {"unit": target, "budget_bytes_per_question": selection_budget_bytes,
                              "retrieval": {key: retrieval[key] for key in ("revision", "budget_bytes", "retrieved_bytes", "retrieved_refs", "log")},
                              "per_question": [{"question_id": result.get("question_id"),
                                                "available_count": result["available_count"],
                                                "selected_count": result["selected_count"],
                                                "deferred_count": result["deferred_count"],
                                                "not_inspected_count": result["not_inspected_count"],
                                                "selected_bytes": result["selected_bytes"],
                                                "coverage": result["coverage"],
                                                "uncertainty": result["uncertainty"]}
                                               for result in question_selections],
                              "semantic_sufficiency": "not_established"},
                "selection_audit": question_selections}
    components = []
    for path in ([] if one_shot else sorted((run / "components").glob("*.json"))):
        component = read_json(path)
        components.append({
            "unit_id": component.get("unit_id"), "package": component.get("package"),
            "status": component.get("status"), "claims": component.get("claims", []),
            "retrieved_sources": component.get("retrieved_sources", []),
            "retrieved_evidence_refs": component.get("retrieved_evidence_refs", []),
            "unresolved_requests": component.get("unresolved_requests", []),
        })
    structure_path = run / "program-model.json"
    program_model = read_json(structure_path) if structure_path.exists() else {}
    selected_packages = {package["id"] for package in inventory.get("packages", [])}
    package_rows = [item for item in program_model.get("packages", []) if item.get("id") in selected_packages]
    file_rows = [item for item in program_model.get("files", []) if item.get("path") in selected_files]
    declarations = [item for item in program_model.get("declarations", []) if item.get("file") in selected_files]
    relationships = [
        item for item in program_model.get("cross_component_relationships", [])
        if item.get("caller_package") in selected_packages or item.get("callee_package") in selected_packages
    ]
    program = {
        "packages": [
            {"id": item.get("id"), "name": item.get("name"), "files": item.get("files", []),
             "imports": item.get("imports", []), "analysis_coverage": item.get("analysis_coverage", {}),
             "exported_entry_point_count": len(item.get("exported_entry_points", []))}
            for item in package_rows
        ],
        "files": [
            {"path": item.get("path"), "package": item.get("package"),
             "package_name": item.get("package_name"), "line_count": item.get("line_count"),
             "declaration_count": len(item.get("declaration_ids", [])), "imports": item.get("imports", [])}
            for item in file_rows
        ],
        "declaration_counts": {
            "total": len(declarations),
            "functions_and_methods": sum(item.get("kind") in {"function", "method"} for item in declarations),
            "types": sum(item.get("kind") == "type" for item in declarations),
        },
        "cross_component_relationship_count": len(relationships),
        "relationship_scope": program_model.get("relationship_scope"),
    }
    aggregate_path = run / "aggregate.json"
    aggregate = None
    if aggregate_path.exists():
        full = read_json(aggregate_path)
        aggregate = {
            "status": full.get("status"),
            "semantic_aggregation": full.get("semantic_aggregation"),
            "packages": {
                package: {"status": node.get("status"), "aggregation": node.get("aggregation"), "claims": node.get("model_claims", []), "conflicts": node.get("conflicts", [])}
                for package, node in full.get("hierarchy", {}).get("packages", {}).items()
            },
            "namespace_groups": {
                namespace: {"status": node.get("status"), "aggregation": node.get("aggregation"), "claims": node.get("model_claims", []), "conflicts": node.get("conflicts", [])}
                for namespace, node in full.get("hierarchy", {}).get("namespace_groups", {}).items()
            },
            "conflicts": full.get("conflicts", []),
        }
    verify_path = run / "verification.json"
    verification = None
    if verify_path.exists():
        full_verification = read_json(verify_path)
        issue_counts = Counter(str(item.get("kind", "unknown")) for item in full_verification.get("issues", []) if isinstance(item, dict))
        verification = {
            "status": full_verification.get("status"),
            "coverage": full_verification.get("coverage", {}),
            "issue_count": len(full_verification.get("issues", [])),
            "issue_counts_by_kind": dict(sorted(issue_counts.items())),
            "semantic_review_scope": full_verification.get("level"),
        }
    evidence = benchmark_evidence(run, inventory, selected_files)
    return {
        "program": program,
        "components": components,
        "aggregate": aggregate,
        "reffector": evidence,
        "verification": verification,
    }


def benchmark(
    run_path: str, manifest_path: str, model_command_json: str | None = None,
    max_context_bytes: int | None = None, reasoning_level: str = "medium",
    max_output_tokens: int = 4000,
    token_counter_command_json: str | list[str] | None = None,
    token_policy: dict[str, int] | None = None,
    methodology: str = "multi_stage_v1",
) -> dict[str, Any]:
    run = pathlib.Path(run_path).resolve()
    metadata = read_json(run / "metadata.json")
    inventory = read_json(run / "inventory.json")
    manifest = load_manifest(manifest_path)
    source_manifest = read_json(run / "source-manifest.json")["files"]
    limit = max_context_bytes or metadata["source_configuration"]["max_context_bytes"]
    token_policy = token_policy or metadata.get("token_budget_policy")
    repo = pathlib.Path(metadata["repo"])
    sources: list[dict[str, Any]] = []
    missing_files: list[str] = []
    source_lines = 0
    source_bytes = 0
    for path, info in sorted(source_manifest.items()):
        try:
            data = read_repo_file(repo, path)
            if digest(data) != info.get("sha256"):
                missing_files.append(path + " (changed)")
                continue
            text = data.decode("utf-8", errors="replace")
            source_lines += len(text.splitlines())
            source_bytes += len(data)
            sources.append({"path": path, "sha256": info.get("sha256"), "text": text})
        except (ArchcharError, OSError):
            missing_files.append(path + " (missing or outside repository)")
    source_hash = digest(canonical_json([{"path": source["path"], "sha256": source_manifest[source["path"]]["sha256"]} for source in sources]))
    question_data = manifest["questions"]
    question_hash = digest(canonical_json(question_data))
    patterns_match = not manifest.get("patterns") or manifest["patterns"] == metadata.get("patterns")
    revision_match = not manifest.get("revision") or manifest["revision"] == metadata.get("revision", {}).get("revision")
    expected_packages = manifest.get("expected_packages")
    actual_packages = sorted(package["id"] for package in inventory.get("packages", []))
    packages_match = not expected_packages or sorted(expected_packages) == actual_packages
    scope_mismatches = []
    if metadata.get("status") != "complete":
        scope_mismatches.append("prepared analysis is partial")
    if truncation_present(inventory.get("truncated")) or inventory.get("coverage", {}).get("omitted", 0):
        scope_mismatches.append("prepared Reffector evidence is truncated or omitted")
    if metadata.get("evidence_scope", {}).get("status") != "matched":
        scope_mismatches.append("Reffector package scope does not match selected source packages")
    if not patterns_match:
        scope_mismatches.append("prepared Go package patterns differ from benchmark manifest")
    if not revision_match:
        scope_mismatches.append("repository revision differs from benchmark manifest")
    if not packages_match:
        scope_mismatches.append("selected packages differ from benchmark manifest")
    for field, actual in (("expected_source_files", len(sources)), ("expected_source_bytes", source_bytes), ("expected_source_lines", source_lines)):
        expected = manifest.get(field)
        if expected is not None and expected != actual:
            scope_mismatches.append(f"{field} expected {expected}, observed {actual}")
    # Source inventory and hashes stay complete externally; model-facing excerpts
    # are selected from AST declarations without consulting Reffector evidence.
    source_structure = read_json(run / "program-structure.json") if (run / "program-structure.json").exists() else {}
    source_model = read_json(run / "program-model.json") if (run / "program-model.json").exists() else source_structure
    selected_packages = {package["id"] for package in inventory.get("packages", [])}
    selected_file_rows = [item for item in source_structure.get("files", []) if item.get("path") in source_manifest]
    selected_declarations = [item for item in source_model.get("declarations", []) if item.get("file") in source_manifest]
    decls_by_file: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for declaration in selected_declarations:
        decls_by_file[str(declaration.get("file"))].append(declaration)
    # Shared source preparation remains evidence-independent. The complete pinned
    # source is available by hash, while only selected declaration excerpts are inline.
    source_preparation = {
        "packages": [
            {"id": item.get("id"), "name": item.get("name"), "files": item.get("files", []), "imports": item.get("imports", [])}
            for item in source_structure.get("packages", []) if item.get("id") in selected_packages
        ],
        "files": [
            {"path": item.get("path"), "package": item.get("package"), "line_count": item.get("line_count"),
             "imports": item.get("imports", []), "declaration_count": len(decls_by_file.get(item.get("path"), [])),
             "declaration_refs": sorted(str(decl.get("id")) for decl in decls_by_file.get(item.get("path"), []) if decl.get("id") is not None),
             "source_sha256": source_manifest.get(item.get("path"), {}).get("sha256")}
            for item in selected_file_rows
        ],
        "declaration_count": len(selected_declarations),
        "source_relationship_directory": {
            "imports": sorted({(item.get("package"), imported) for item in selected_file_rows
                               for imported in item.get("imports", []) if isinstance(imported, str)}),
            "count": sum(len(item.get("imports", [])) for item in selected_file_rows),
            "scope": "source-derived Go imports only; Reffector call/effect relationships are excluded",
        },
        "source_coverage": {"files": len(sources), "bytes": source_bytes, "lines": source_lines,
                            "registered_files": len(source_manifest), "deduplicated_paths": True,
                            "source_refs_are_hash_bound": True},
        "available_source_inventory": sorted(source_manifest),
        "canonical_index": "program-structure.json",
    }
    full_sources = list(sources)
    from .source_selection import select_source_context
    target_unit = {"id": "rclone-vfs-cache-subsystem", "packages": sorted(selected_packages), "files": sorted(source_manifest)}
    source_selections = [select_source_context(question, target_unit, source_model, source_manifest, repo,
                                               budget_bytes=30000) for question in question_data]
    excerpt_by_ref = {}
    for selection in source_selections:
        for excerpt in selection["selected"]:
            entry = dict(excerpt)
            entry["path"] = entry["file"]
            entry.pop("reason", None)
            entry.pop("relevance_hits", None)
            entry.pop("has_body", None)
            entry.pop("type_kind", None)
            entry.pop("interface_methods", None)
            entry["question_ids"] = [selection["question_id"]]
            if excerpt["ref"] in excerpt_by_ref:
                excerpt_by_ref[excerpt["ref"]]["question_ids"].append(selection["question_id"])
            else:
                excerpt_by_ref[excerpt["ref"]] = entry
    selected_source_excerpts = sorted(excerpt_by_ref.values(), key=lambda row: (row["file"], row["start_line"], row["ref"]))
    selected_source_paths = sorted({row["file"] for row in selected_source_excerpts})
    selected_declaration_refs = {row["ref"] for row in selected_source_excerpts}
    selected_declarations = [row for row in selected_declarations if row.get("id") in selected_declaration_refs]
    selected_by_file: dict[str, list[str]] = defaultdict(list)
    for declaration in selected_declarations:
        selected_by_file[str(declaration.get("file"))].append(str(declaration.get("id")))
    for file_row in source_preparation["files"]:
        file_row["available_declaration_count"] = file_row.pop("declaration_count")
        file_row["declaration_count"] = len(selected_by_file.get(str(file_row.get("path")), []))
        file_row["declaration_refs"] = sorted(selected_by_file.get(str(file_row.get("path")), []))
    source_preparation["selected_declaration_count"] = len(selected_declarations)
    source_preparation["available_declaration_count"] = len(source_model.get("declarations", []))
    focused_source_files = []
    for path in selected_source_paths:
        excerpts = [row for row in selected_source_excerpts if row["file"] == path]
        focused_source_files.append({
            "path": path, "sha256": source_manifest[path]["sha256"],
            "text": "\n\n".join(row["text"] for row in excerpts),
            "excerpts": [{key: row[key] for key in ("ref", "start_line", "end_line", "question_ids", "text")} for row in excerpts],
        })
    use_focused_context = methodology == "one_shot_paired_v1"
    sources = focused_source_files if use_focused_context else full_sources
    source_retrieval_catalog = [
        {key: declaration.get(key) for key in ("id", "package", "file", "kind", "symbol", "name", "start_line", "end_line", "exported", "type_kind")}
        for declaration in selected_declarations if isinstance(declaration.get("id"), str)
    ]
    source_selection_context = []
    for selection in source_selections:
        source_selection_context.append({
            "question_id": selection["question_id"], "unit": selection["unit"],
            "available_declaration_count": selection["available_declaration_count"],
            "candidate_declaration_count": selection["candidate_declaration_count"],
            "selected": [{field: excerpt.get(field) for field in ("ref", "file", "start_line", "end_line", "question_ids")}
                         for excerpt in selection["selected"]],
            "deferred_count": len(selection["deferred"]),
            "not_selected_count": len(selection["not_selected_refs"]),
            "budget_bytes": selection["budget_bytes"], "selected_bytes": selection["selected_bytes"],
            "traversal": selection["traversal"], "source_hashes": selection["source_hashes"],
            "retrieval_log": selection["retrieval_log"], "source_coverage": selection["source_coverage"],
            "uncertainty": selection["uncertainty"],
        })
    common = {
        "protocol": "reffector-archchar/focused-paired-v2" if use_focused_context else "reffector-archchar/v1",
        "phase": "benchmark", "system": BENCHMARK_PROMPT,
        "questions": question_data, "source": sources,
        "source_paths": selected_source_paths if use_focused_context else sorted(source_manifest),
        "source_preparation": source_preparation,
        "source_revision": metadata["revision"], "reasoning_level": reasoning_level,
        "max_output_tokens": max_output_tokens,
    }
    if use_focused_context:
        common.update({"included_source_paths": selected_source_paths,
                       "available_source_inventory": source_preparation["files"],
                       "source_retrieval_catalog": source_retrieval_catalog,
                       "source_selection": source_selection_context})
    selected_files = set(source_manifest)
    if methodology not in {"multi_stage_v1", "one_shot_paired_v1"}:
        raise ArchcharError("unknown benchmark methodology")
    treatment_context = benchmark_treatment_context(run, inventory, selected_files, methodology == "one_shot_paired_v1", question_data)
    selection_audit = treatment_context.pop("selection_audit", None)
    phase_mismatches = []
    investigation_path = run / "investigation-status.json"
    if methodology == "multi_stage_v1" and (not investigation_path.exists() or read_json(investigation_path).get("status") != "complete"):
        phase_mismatches.append("component investigations are missing or incomplete")
    aggregate_path = run / "aggregate.json"
    if methodology == "multi_stage_v1" and (not aggregate_path.exists() or read_json(aggregate_path).get("status") not in ("complete", "structural_only")):
        phase_mismatches.append("hierarchical aggregation is missing or incomplete")
    verification_path = run / "verification.json"
    if methodology == "multi_stage_v1" and (not verification_path.exists() or read_json(verification_path).get("status") != "complete"):
        phase_mismatches.append("evidence verification is missing or incomplete")
    labels_and_contexts = (("source_only", None), ("reffector_treatment", treatment_context))
    prepared_requests = {}
    for label, additional in labels_and_contexts:
        request = dict(common)
        request["condition_data"] = additional
        request["mode"] = ("paired_source_context" if methodology == "one_shot_paired_v1" else
                            ("ordinary_source_analysis" if additional is None else "component_analysis_with_reffector"))
        request["methodology"] = methodology
        prepared_requests[label] = request
    partitioned_arms: dict[str, dict[str, Any]] = {}
    pair_call_budgets: dict[str, dict[str, Any]] = {}
    if methodology == "one_shot_paired_v1":
        whole_budgets: dict[str, dict[str, Any] | None] = {label: None for label in prepared_requests}
        whole_over = any(request_size(request) > limit for request in prepared_requests.values())
        if not whole_over and token_counter_command_json and token_policy:
            for label, request in prepared_requests.items():
                try:
                    whole_budgets[label] = run_token_counter(token_counter_command_json, request, run, token_policy)
                    whole_budgets[label]["request_sha256"] = digest(canonical_json(request))
                except TokenBudgetError as error:
                    whole_budgets[label] = {"status": "unavailable", "error": redact_diagnostics(str(error))}
            whole_over = any(value is None or value.get("status") != "within_budget" for value in whole_budgets.values())
        if whole_over:
            partition_rows: dict[str, list[dict[str, Any]]] = {}
            all_fit = not missing_files and not scope_mismatches
            for label, request in prepared_requests.items():
                rows = []
                for ordinal, source in enumerate(sources, start=1):
                    part = dict(request)
                    part["source"] = [source]
                    part["source_paths"] = [source["path"]]
                    part["source_partition"] = {"ordinal": ordinal, "total": len(sources), "file": source["path"], "complete_file": True}
                    part["source_preparation"] = dict(source_preparation)
                    part["source_preparation"]["files"] = list(source_preparation["files"])
                    part["source_preparation"]["partition_coverage"] = {
                        "file": source["path"], "files": 1,
                        "bytes": len(source["text"].encode("utf-8")),
                        "source_sha256": source_manifest[source["path"]]["sha256"],
                        "declaration_refs": next((item["declaration_refs"] for item in source_preparation["files"] if item.get("path") == source["path"]), []),
                    }
                    rows.append({"request": part, "file": source["path"], "ordinal": ordinal,
                                 "request_bytes": request_size(part), "token_budget": None})
                partition_rows[label] = rows
            # Count every partition of both arms before any model process starts.
            for label, rows in partition_rows.items():
                for row in rows:
                    if row["request_bytes"] > limit:
                        all_fit = False
                        row["status"] = "over_budget_bytes"
                        continue
                    if token_counter_command_json and token_policy:
                        try:
                            row["token_budget"] = run_token_counter(token_counter_command_json, row["request"], run, token_policy)
                            row["token_budget"]["request_sha256"] = digest(canonical_json(row["request"]))
                            if row["token_budget"].get("status") != "within_budget":
                                all_fit = False
                        except TokenBudgetError as error:
                            row["token_budget"] = {"status": "unavailable", "error": redact_diagnostics(str(error))}
                            all_fit = False
                    else:
                        all_fit = False
                        row["status"] = "pending_unbudgeted"
            if model_command_json and (not token_counter_command_json or not token_policy):
                all_fit = False
            for label, rows in partition_rows.items():
                if not all_fit or not model_command_json:
                    partitioned_arms[label] = {
                        "status": "partition_plan_pending" if all_fit else "preflight_blocked",
                        "request": prepared_requests[label], "request_bytes": request_size(prepared_requests[label]),
                        "whole_request_token_budget": whole_budgets.get(label), "source_partitions": rows,
                        "partition_count": len(rows), "planned_generation_calls": len(rows) if all_fit else 0,
                        "aggregation_required": True,
                        "reason": "count-only partition plan; generation is not complete" if all_fit else "at least one paired partition failed preflight; no model process started",
                    }
            if all_fit and model_command_json:
                # Deterministic sequential execution: arm order, then manifest source order.
                partition_execution_failed = False
                for label, rows in partition_rows.items():
                    if partition_execution_failed:
                        partitioned_arms[label] = {"status": "skipped_after_peer_failure", "request": prepared_requests[label],
                                                   "whole_request_token_budget": whole_budgets.get(label),
                                                   "partition_count": len(rows), "planned_generation_calls": len(rows),
                                                   "generation_calls_started": 0, "reason": "paired peer partition execution failed"}
                        continue
                    combined = []
                    artifacts = []
                    failed = False
                    calls_started = 0
                    for row in rows:
                        key = f"{label}-partition-{row['ordinal']:04d}"
                        calls_started += 1
                        response, call_path = invoke_saved_model_call(
                            run, "benchmark-calls", key, model_command_json, row["request"],
                            token_counter_command_json=token_counter_command_json, token_policy=token_policy,
                            preflight_budget=row["token_budget"],
                        )
                        artifacts.append(str(call_path.relative_to(run)))
                        if response.get("status") != "complete":
                            failed = True
                            partition_execution_failed = True
                            break
                        claims = response.get("result", {}).get("claims", [])
                        for claim in claims:
                            merged = dict(claim)
                            original_id = str(merged.get("id", "claim"))
                            merged["id"] = f"partition-{row['ordinal']:04d}:{original_id}"
                            merged["partition_provenance"] = {"file": row["file"], "ordinal": row["ordinal"], "source_sha256": source_manifest[row["file"]]["sha256"]}
                            combined.append(merged)
                    partitioned_arms[label] = {
                        "status": "failed" if failed else "complete", "request": prepared_requests[label],
                        "request_bytes": request_size(prepared_requests[label]),
                        "whole_request_token_budget": whole_budgets.get(label), "artifacts": artifacts,
                        "source_partitions": rows, "result": {"claims": combined}, "partition_count": len(rows),
                        "planned_generation_calls": len(rows), "generation_calls_started": calls_started,
                        "failed_generation_calls": int(failed),
                        "aggregation": "deterministic_claim_list_concatenation_with_partition_provenance",
                    }
    pair_preflight_error = None
    if methodology == "one_shot_paired_v1" and model_command_json and not partitioned_arms:
        if missing_files or scope_mismatches:
            pair_preflight_error = "registered source scope could not be confirmed"
        elif any(request_size(request) > limit for request in prepared_requests.values()):
            pair_preflight_error = "one or both paired requests exceed the byte budget"
        elif not token_counter_command_json or not token_policy:
            pair_preflight_error = "paired generation requires successful count-only preflight for both arms"
        else:
            pair_call_budgets = dict(whole_budgets)
            if any(value is None or value.get("status") != "within_budget" for value in pair_call_budgets.values()):
                pair_preflight_error = "both arm preflights did not fit; neither generation was started"
    arms: dict[str, Any] = dict(partitioned_arms)
    one_shot_peer_failed = any(value.get("status") in {"failed", "preflight_blocked"} for value in partitioned_arms.values())
    for label, additional in labels_and_contexts:
        if label in arms:
            continue
        if methodology == "one_shot_paired_v1" and one_shot_peer_failed:
            arms[label] = {"status": "skipped_after_peer_failure", "reason": "paired peer generation failed before this arm started", "generation_calls_started": 0}
            continue
        request = prepared_requests[label]
        size = request_size(request)
        if pair_preflight_error:
            arms[label] = {"status": "preflight_blocked", "request_bytes": size, "reason": pair_preflight_error}
            continue
        whole_budget = whole_budgets.get(label) if methodology == "one_shot_paired_v1" else None
        if token_counter_command_json and token_policy and not model_command_json and whole_budget is None:
            try:
                whole_budget = run_token_counter(token_counter_command_json, request, run, token_policy)
                whole_budget["request_sha256"] = digest(canonical_json(request))
            except TokenBudgetError as error:
                whole_budget = {"status": "unavailable", "error": redact_diagnostics(str(error))}
        over_tokens = whole_budget is not None and whole_budget.get("status") == "over_budget"
        if size > limit or over_tokens:
            # Offer a lossless, deterministic preflight plan. No source is dropped:
            # each partition carries complete files, and cross-partition links stay
            # in the shared structural directory for later aggregation.
            source_partitions = []
            by_package = {item.get("path"): item.get("package") for item in source_preparation["files"]}
            for source in sources:
                partition = dict(request)
                partition["source"] = [source]
                partition["source_partition"] = {"file": source["path"], "ordinal": len(source_partitions) + 1,
                                                 "total": len(sources), "complete_file": True}
                partition["source_preparation"] = dict(source_preparation)
                partition["source_preparation"]["files"] = [item for item in source_preparation["files"] if item.get("path") == source["path"]]
                partition["source_preparation"]["file_declaration_count"] = next(
                    (item.get("declaration_count", 0) for item in source_preparation["files"] if item.get("path") == source["path"]), 0)
                partition_size = request_size(partition)
                partition_budget = None
                if token_counter_command_json and token_policy and not model_command_json:
                    try:
                        partition_budget = run_token_counter(token_counter_command_json, partition, run, token_policy)
                        partition_budget["request_sha256"] = digest(canonical_json(partition))
                    except TokenBudgetError as error:
                        partition_budget = {"status": "unavailable", "error": redact_diagnostics(str(error))}
                byte_fits = partition_size <= limit
                token_fits = partition_budget is None or partition_budget.get("status") == "within_budget"
                source_partitions.append({"file": source["path"], "package": by_package.get(source["path"]),
                                          "request_bytes": partition_size, "byte_status": "within_budget" if byte_fits else "over_budget",
                                          "token_budget": partition_budget, "request": partition})
            all_parts_fit = all(item["byte_status"] == "within_budget" and
                                (item["token_budget"] is None or item["token_budget"].get("status") == "within_budget")
                                for item in source_partitions)
            arms[label] = {"status": "partition_plan_pending" if all_parts_fit else "over_budget",
                           "request_bytes": size, "token_budget": whole_budget,
                           "max_context_bytes": limit,
                           "reason": "no generation performed; complete-file partitions require separate execution and aggregation" if all_parts_fit else "request and/or a complete-file partition exceeds declared budget; source retained",
                           "source_partitions": source_partitions,
                           "aggregation_required": len(source_partitions) > 1}
            continue
        if missing_files or scope_mismatches:
            arms[label] = {
                "status": "partial",
                "request_bytes": size,
                "missing_files": missing_files,
                "scope_mismatches": scope_mismatches,
                "reason": "registered source scope could not be confirmed",
            }
            continue
        if model_command_json and phase_mismatches:
            arms[label] = {
                "status": "partial",
                "request_bytes": size,
                "phase_mismatches": phase_mismatches,
                "reason": "paired model calls require complete investigation, aggregation, and verification phases",
            }
            continue
        if model_command_json:
            response, response_path = invoke_saved_model_call(
                run, "benchmark-calls", label, model_command_json, request,
                token_counter_command_json=token_counter_command_json, token_policy=token_policy,
                preflight_budget=pair_call_budgets.get(label),
            )
            arms[label] = {
                "status": response["status"],
                "generation_calls_started": 1,
                "failed_generation_calls": int(response.get("status") != "complete"),
                "planned_generation_calls": 1,
                "artifact": str(response_path.relative_to(run)),
                "request": request,
                "request_bytes": size,
                "result": response.get("result"),
                "latency_seconds": response.get("latency_seconds"),
                "usage": response.get("usage"),
                "cost_usd": response.get("cost_usd"),
                "model": response.get("model"),
                "command": response.get("command"),
                "token_budget": response.get("token_budget"),
            }
            if response.get("status") != "complete" and methodology == "one_shot_paired_v1":
                one_shot_peer_failed = True
        elif token_counter_command_json and token_policy:
            budget = whole_budget or {"status": "unavailable", "error": "token counter was not run"}
            arms[label] = {
                "status": "preflight_complete" if budget.get("status") == "within_budget" else budget.get("status", "budget_unavailable"),
                "request": request, "request_bytes": size, "token_budget": budget,
                "usage": None, "cost_usd": None, "latency_seconds": None,
            }
        else:
            arms[label] = {"status": "pending", "request": request, "request_bytes": size, "usage": None, "cost_usd": None, "latency_seconds": None}

    evidence_index = index_evidence(run)
    blind_scores: dict[str, Any] = {}
    for blind_label, arm_label in (("A", "source_only"), ("B", "reffector_treatment")):
        arm = arms.get(arm_label, {})
        if arm.get("status") == "complete":
            claims = (arm.get("result") or {}).get("claims", [])
            blind_scores[blind_label] = claim_score(claims, manifest["rubric"], evidence_index, repo, source_manifest)
        else:
            blind_scores[blind_label] = {"status": "not_scored", "reason": f"arm status is {arm.get('status', 'missing')}"}
    scores = {"source_only": blind_scores["A"], "reffector_treatment": blind_scores["B"]}
    def current_arm_calls(label: str) -> list[dict[str, Any]]:
        arm = arms.get(label, {})
        artifact_paths = arm.get("artifacts")
        if isinstance(artifact_paths, list):
            loaded = []
            for relative in artifact_paths:
                try:
                    loaded.append(read_json(run / relative))
                except (OSError, ValueError):
                    continue
            return loaded
        request = arm.get("request")
        if not isinstance(request, dict):
            return []
        request_hash = digest(canonical_json(request))
        records = []
        for path in sorted((run / "benchmark-calls").glob(f"{label}*.json")):
            try:
                record = read_json(path)
            except (OSError, ValueError):
                continue
            if record.get("request_sha256") == request_hash:
                records.append(record)
        return records

    baseline_metrics = metrics(current_arm_calls("source_only"))
    treatment_final_calls = current_arm_calls("reffector_treatment")
    treatment_final_metrics = metrics(treatment_final_calls)
    prior_treatment_calls = model_call_records(run, {"component", "component_followup", "aggregate", "verify"})
    treatment_total_metrics = metrics(prior_treatment_calls + treatment_final_calls)
    initial_packets = []
    for packet_path in sorted((run / "packets").glob("*.json")) if (run / "packets").exists() else []:
        try:
            initial_packets.append(read_json(packet_path))
        except (OSError, ValueError):
            continue
    initial_token_counts = [item.get("token_budget", {}).get("input_tokens") for item in initial_packets]
    known_initial_tokens = [value for value in initial_token_counts if isinstance(value, int) and not isinstance(value, bool) and value >= 0]
    cap_per_call = 4.0
    final_preflight = {}
    for label, arm in arms.items():
        parts = arm.get("source_partitions")
        if isinstance(parts, list):
            counts = [item.get("token_budget", {}).get("input_tokens") for item in parts
                      if isinstance(item.get("token_budget"), dict)]
            final_preflight[label] = {"partition_input_tokens": counts, "input_tokens_total": sum(counts),
                                      "partition_calls": len(parts)}
        elif isinstance(arm.get("token_budget"), dict):
            final_preflight[label] = {"input_tokens": arm["token_budget"].get("input_tokens"), "partition_calls": 1}
    planned_final_calls = sum(
        arm["planned_generation_calls"] if "planned_generation_calls" in arm
        else (1 if arm.get("status") in {"within_budget", "preflight_complete", "complete"} else 0)
        for arm in arms.values()
    )
    generation_calls_started = sum(
        arm.get("generation_calls_started", len(arm.get("artifacts", [])) if isinstance(arm.get("artifacts"), list) else int(arm.get("status") == "complete"))
        for arm in arms.values()
    )
    failed_generation_calls = sum(arm.get("failed_generation_calls", int(arm.get("status") == "failed")) for arm in arms.values())
    source_partition_lists = [arm.get("source_partitions") for arm in arms.values()]
    matched_partition_count = 0
    if len(source_partition_lists) == 2 and all(isinstance(rows, list) for rows in source_partition_lists):
        first_files = [row.get("file") for row in source_partition_lists[0]]
        second_files = [row.get("file") for row in source_partition_lists[1]]
        if first_files == second_files:
            matched_partition_count = len(first_files)
    hypothetical_partition_calls = 2 * matched_partition_count if methodology == "one_shot_paired_v1" else 0
    complete = all(arms.get(label, {}).get("status") == "complete" for label in ("source_only", "reffector_treatment")) and not missing_files and not scope_mismatches and not phase_mismatches
    content_category_bytes = {}
    for label, arm in arms.items():
        request_value = arm.get("request")
        if not isinstance(request_value, dict):
            continue
        content_category_bytes[label] = {
            "complete_source": len(canonical_json(request_value.get("source", []))),
            "source_preparation": len(canonical_json(request_value.get("source_preparation", {}))),
            "questions_and_prompt": len(canonical_json({"questions": request_value.get("questions"), "system": request_value.get("system")})),
            "treatment_only_evidence": len(canonical_json(request_value.get("condition_data"))) if label == "reffector_treatment" else 0,
            "other_request_fields": len(canonical_json({key: value for key, value in request_value.items() if key not in {"source", "source_preparation", "questions", "system", "condition_data"}})),
            "serialized_request_total": request_size(request_value),
        }
    result = {
        "schema_version": SCHEMA_VERSION,
        "status": "complete" if complete else ("pending" if not model_command_json and all(arm.get("status") == "pending" for arm in arms.values()) else ("preflight_complete" if not model_command_json and all(arm.get("status") == "preflight_complete" for arm in arms.values()) else "partial")),
        "dataset": manifest.get("name"),
        "methodology": methodology,
        "source_revision": metadata["revision"],
        "source_scope_sha256": source_hash,
        "question_sha256": question_hash,
        "model_command": model_command_display(model_command_json),
        "reasoning_level": reasoning_level,
        "max_output_tokens": max_output_tokens,
        "max_context_bytes": limit,
        "token_budget_policy": token_policy,
        "token_counter_command": model_command_display(json.dumps(token_counter_command_json) if isinstance(token_counter_command_json, list) else token_counter_command_json),
        "scope_validation": {
            "patterns_match": patterns_match,
            "revision_match": revision_match,
            "packages_match": packages_match,
            "actual_packages": actual_packages,
            "source_files": len(source_manifest),
            "selected_source_files": len(selected_source_paths),
            "source_bytes": source_bytes,
            "source_lines": source_lines,
            "mismatches": scope_mismatches,
        },
        "treatment_phase_validation": {
            "complete": methodology == "multi_stage_v1" and not phase_mismatches,
            "required": methodology == "multi_stage_v1",
            "mismatches": phase_mismatches,
        },
        "selection_audit": selection_audit,
        "source_selection_audit": [{key: value for key, value in selection.items() if key != "selected"} | {
            "selected": [{key: value for key, value in excerpt.items() if key != "text"} for excerpt in selection["selected"]]
        } for selection in source_selections],
        "paired_inputs": {
            "same_source_scope": True,
            "same_question_digest": True,
            "source_scope_sha256": source_hash,
            "question_sha256": question_hash,
            "missing_files": missing_files,
        },
        "arms": arms,
        "content_category_bytes": content_category_bytes,
        "scores_blind_then_mapped": scores,
        "metrics": {
            "source_only": baseline_metrics,
            "treatment_final_call": treatment_final_metrics,
            "treatment_investigation_aggregation_verification_and_final": treatment_total_metrics,
        },
        "call_economics": {
            "status": "dry_run_plan_only" if not model_command_json else "observed_run",
            "guaranteed_initial_investigation": {
                "required_by_methodology": methodology == "multi_stage_v1",
                "calls_if_investigate_runs": len(initial_packets) if methodology == "multi_stage_v1" else 0,
                "exact_counted_calls": len(known_initial_tokens) if methodology == "multi_stage_v1" else 0,
                "input_tokens": sum(known_initial_tokens) if methodology == "multi_stage_v1" else 0,
                "max_input_tokens_per_call": max(known_initial_tokens, default=None) if methodology == "multi_stage_v1" else None,
                "cap_usd_per_call": cap_per_call,
                "cap_bound_usd": len(initial_packets) * cap_per_call if methodology == "multi_stage_v1" else 0,
            },
            "final_benchmark_calls": {
                "planned_generation_calls": planned_final_calls,
                "generation_calls_started": generation_calls_started,
                "failed_generation_calls": failed_generation_calls,
                "hypothetical_calls_if_all_matched_partitions_fit": hypothetical_partition_calls,
                "hypothetical_cap_exposure_usd": hypothetical_partition_calls * cap_per_call,
                "input_tokens_by_arm": final_preflight,
                "cap_usd_per_call": cap_per_call,
                "planned_cap_exposure_usd": planned_final_calls * cap_per_call,
                "started_call_cap_exposure_usd": generation_calls_started * cap_per_call,
                "cap_bound_usd": planned_final_calls * cap_per_call,
            },
            "conditional_additional_calls": ({
                "retrieval": "none; complete source is supplied in the paired request",
                "semantic_aggregation": "0 calls; partition claims are concatenated locally and deterministically",
                "model_verification": "none; only deterministic claim scoring is performed",
            } if methodology == "one_shot_paired_v1" else {
                "retrieval": "conditional on model requests; count and tokens unknown before generation",
                "aggregation": "conditional on aggregation configuration/status; count and tokens unknown before generation",
                "verification": "conditional on verification configuration/status; count and tokens unknown before generation",
            }),
            "future_retrieval_and_output": {
                "retrieval_followup_calls": "unknown; optional requests require an exact preflight before each generation call",
                "retrieval_request_tokens": "unknown until a bounded request exists",
                "actual_output_tokens": "unknown until generation; per-call reserved maximum is 32000",
                "additional_full_cap_calls_with_remaining_budget": 0,
                "budget_note": "Two planned initial calls at the $4 cap expose $8; the fixed $10.5219434 remaining leaves $2.5219434, below one additional $4 cap. No future call is authorized by this dry run.",
            },
            "no_generation_performed": not bool(model_command_json),
            "note": "A dry-run/preflight plan is not a completed run; cost bounds assume the configured per-call cap and are not observed charges.",
        },
        "rubric": manifest["rubric"],
        "scoring_limits": [
            "Lexical required-fact matching is coverage evidence, not semantic accuracy.",
            "Citation checks validate source line/hash and Reffector IDs/confidence, not all claim meaning.",
        ],
    }
    write_json(run / "benchmark.json", result)
    return result
