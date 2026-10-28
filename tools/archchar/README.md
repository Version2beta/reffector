# Architectural characterization harness

`tools.archchar` is a Python 3.10+ standard-library harness for bounded, confidence-aware characterization of Go repositories using Reffector JSON/audit evidence and optional isolated model subprocesses. Its tests use `unittest`; it adds no Go or Python dependencies. Preparation is offline and read-only for the analyzed source tree. The harness invokes configured model commands exactly as argv; those commands may use the network or other capabilities, so configure them deliberately. Run tests from the Reffector repository root with bytecode generation disabled:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover -s tools/archchar/tests -v
PYTHONDONTWRITEBYTECODE=1 python3 -B -m tools.archchar --help
```

## Prepare a bounded evidence run

```sh
python3 -B -m tools.archchar prepare /path/to/repository \
  --out /tmp/archchar-run \
  --reffector /path/to/reffector \
  --pattern './vfs/vfscache/...' \
  --pattern './vfs/vfscommon' \
  --max-sites 50000 --max-edges 200000 --max-depth 100 \
  --max-context-bytes 500000 \
  --max-retrieval-rounds 2 --max-retrieval-bytes 100000 \
  --token-counter-command-json '["/path/to/count-only-adapter"]' \
  --context-window-tokens 200000 \
  --output-reserve-tokens 32000 --safety-margin-tokens 30000
```

`--pattern` is repeatable and defaults to `./...`. The output directory must be outside the analyzed repository and empty or new. Preparation uses an offline Go environment (`GOENV=off`, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOFLAGS=-mod=readonly`, `GOWORK=off`), passes only required Go/toolchain environment settings, and places temporary files under the output directory. It does not install dependencies or write into the analyzed source tree. The standard-library Go AST indexer runs locally and uses no analysis dependencies.

The effects command receives the site/edge/depth caps. The audit command is invoked separately without graph caps because that interface does not accept those flags. `go list -json` uses the same package patterns. Effects coverage package IDs must exactly match the selected source package IDs; a mismatch makes the run partial and blocks a complete benchmark. Source hashes and repository revision/dirty state are recorded. `--effects FILE --audit FILE` imports saved outputs without executing Reffector; imported data must still match the current source package scope. `import` is an alias that requires both files. Raw Reffector, audit, and `go list` output is preserved under `raw-*`; malformed or incomplete evidence is reported, not silently repaired.

Packages are deterministic initial analysis units, not proven architectural components. Preparation runs a local, dependency-free Go AST indexer over the selected source and saves `program-structure.json` and `program-model.json`: package/file membership, imports, declarations, function/method signatures and bodies' exact line ranges, types, struct fields, interface methods, and exported entry points. It builds `evidence-index.json` from normalized, deduplicated Reffector records while preserving stable references to the raw exports. Repeated reverse summaries are represented once and retain full origin/edge/node detail by reference; no graph record is silently truncated.

If a package request does not fit the byte or token budget, preparation groups declarations by receiver type and observed package-local relationship, then falls back to declaration-sized source fragments with the package preamble and structural symbol directory. Every fragment has original line numbers and source hashes; all selected source and evidence remain addressable. A single declaration that still cannot fit is retained as over budget and is never sent. Token-aware partitioning requires a configured count-only command; without one, packets are `pending_unbudgeted` and generation is blocked. Retrieval can request either selected source files or `need_evidence` references, within an explicit package allowlist and shared byte/round/token budgets. Package imports are exact; Reffector effect-site reverse relationships are not a complete call graph. Unmatched references, exclusions, coverage and truncation are explicit.

## Optional isolated model investigations

A model command is a JSON argv array—not shell text; the harness does not invoke a shell. The configured executable itself may use a shell or network. The harness sends one UTF-8 JSON request on stdin and expects one JSON object on stdout. It does not choose a provider or contact a model by default. Every generation requires a separate count-only command and a token policy; the count command receives `{protocol: reffector-archchar/token-count/v1, request, context_window_tokens, output_reserve_tokens, safety_margin_tokens}` and must return `{input_tokens, token_count_type: exact|estimate, model, context_window_tokens, max_output_tokens}` without generating text. The runtime model reported by the counter must match the generation adapter. Missing/invalid counts, runtime-limit mismatches, or `input + output reserve + safety > context` block generation. The same token controls apply to component, retrieval, aggregation, verification, and benchmark requests. Count-only commands may contact a provider's token-count endpoint; no generation is performed by preparation or preflight.

```sh
python3 -B -m tools.archchar investigate /tmp/archchar-run \
  --model-command-json '["/path/to/adapter","--model","configured-model"]' \
  --token-counter-command-json '["/path/to/count-only-adapter"]' \
  --context-window-tokens 200000 --output-reserve-tokens 32000 --safety-margin-tokens 30000
python3 -B -m tools.archchar aggregate /tmp/archchar-run \
  --model-command-json '["/path/to/adapter","--model","configured-model"]' \
  --token-counter-command-json '["/path/to/count-only-adapter"]' \
  --context-window-tokens 200000 --output-reserve-tokens 32000 --safety-margin-tokens 30000
python3 -B -m tools.archchar verify /tmp/archchar-run \
  --model-command-json '["/path/to/independent-verifier"]' \
  --token-counter-command-json '["/path/to/count-only-adapter"]' \
  --context-window-tokens 200000 --output-reserve-tokens 32000 --safety-margin-tokens 30000
```

Component responses have this shape:

```json
{
  "claims": [
    {
      "id": "stable-claim-id",
      "statement": "A bounded statement about the supplied source",
      "category": "established",
      "domain": "architecture",
      "evidence": [
        {"kind": "source", "file": "relative/path.go", "line": 12, "quote": "exact source line"},
        {"kind": "reffector", "id": "site-or-edge-id", "field": "effects"}
      ]
    }
  ],
  "need_sources": ["directly-related/other.go"],
  "usage": {"input_tokens": 100, "output_tokens": 40},
  "cost_usd": 0.001,
  "model": "configured-model"
}
```

`need_sources` and `need_evidence` are optional. Requests are accepted only for manifest-listed source files or evidence refs in the unit's explicit allowlist; path traversal, unrelated packages, unknown refs, changed files, duplicates, and retrieval over configured byte/round/token limits are rejected and reported. Requested source is re-hashed before disclosure; evidence records are read from the preserved canonical index. Re-running `investigate` resumes completed units and preserves each model-call artifact. A changed source scope stops investigation; prepare a new run instead of mixing revisions.

Categories are `established`, `possible`, `inferred`, and `unknown`; domains are `architecture`, `relationship`, `effect`, `runtime`, and `concurrency`. `concurrency` is a valid domain for claims about synchronization, shared state, cancellation, and lifecycle obligations; it does not by itself assert runtime execution. Static reachability, candidate edges, `possible_effects`, and observed callback arguments do not establish runtime execution or completion. Missing callers, unsupported flows, omitted evidence and budget limits remain open. Empty claims do not establish purity.

Completed aggregate, verification and benchmark model calls are reused only for the same saved request and displayed command. Component investigations also resume completed units. Use a new output directory when changing model/provider configuration. Model output and diagnostics are stored with common credential patterns redacted; the original request is retained. Requests intentionally include analyzed source and evidence, so configure the adapter/provider for the repository's data sensitivity.

`--token-counter-command-json` is a separate JSON argv program that reads the request envelope from stdin and emits counts only; it must not call text generation. Context, output, and safety limits must all be positive integers and reflect the selected runtime. The harness records exact versus estimated counting, preflight status, model identity, input count, output reserve, safety margin, and the total. `prepare` writes `token-budget-report.json`; downstream call artifacts retain their own per-request counts. Without a counter, `prepare` may write offline evidence packets, but they remain `pending_unbudgeted` and `investigate` will not send them.

## Aggregation and verification

`aggregate` retains leaf claims, detects conflicting claims, and constructs package plus import-path namespace groups. Namespace groups are explicit heuristics, not architecture facts. Optional package/namespace summaries must cite child claim IDs and may not strengthen child confidence; verification checks ancestry and confidence promotions.

`verify` validates claim shape, exact source-line quotes against the prepared hash, Reffector IDs and evidence fields, candidate/possible-only effect claims, runtime assertions, completeness, truncation, aggregation ancestry, and confidence compatibility before semantic review. Deterministically rejected claims are not sent for model validation. An optional verifier receives only valid claims' cited excerpts and referenced records and returns `{"verdicts":[{"claim_id":"...","verdict":"supported|unsupported|uncertain","reason":"..."}]}`. Unsupported, uncertain, missing, malformed, or over-budget verdicts keep the verification partial. These checks are bounded review signals, not proof of semantic truth; same-model verification is not independent review.

## Paired benchmark

A JSON manifest fixes source patterns/revision/inventory (where provided), questions and a deterministic lexical/citation rubric before model calls. By default, the legacy `multi_stage_v1` method blocks paired calls unless component investigation, hierarchical aggregation, and verification completed. The explicit `--methodology one_shot_paired_v1` method makes one paired request per arm when both fit. Both arms receive exactly the same question-directed, hash-checked source excerpts selected from the canonical AST index, source-derived package/file/import references, questions, prompt, and token policy; neither gets source tools. Full source files and indexes remain intact in the run, outside the requests. AST selection is identical in both arms and attributes excerpts to questions. Treatment alone receives selected Reffector evidence. The evidence selector ranks records deterministically and reserves uncertainty/record-type coverage; selected references are fetched locally from the canonical evidence index, revision-checked, byte-bounded, and recorded in the treatment selection audit. Invalid refs or budget overflow fail closed. Reffector edges/classifications are never included in shared source preparation. If a request exceeds configured limits, no paired generation starts. Count-only runs are preflight evidence, not completed comparisons. Reports distinguish cost caps from observed charges.

```sh
python3 -B -m tools.archchar benchmark /tmp/archchar-run \
  --manifest tools/archchar/benchmark-rclone-vfs.json \
  --model-command-json '["/path/to/adapter","--model","configured-model"]' \
  --token-counter-command-json '["/path/to/count-only-adapter"]' \
  --context-window-tokens 200000 --output-reserve-tokens 32000 --safety-margin-tokens 30000 \
  --reasoning-level medium --max-output-tokens 4000
```

The checked-in rclone manifest targets four selected VFS cache/common packages at a pinned revision, not the entire rclone repository. Confirm `scope_validation` before interpreting results. For `one_shot_paired_v1`, both fixed questions share the explicitly named `rclone-vfs-cache-subsystem` analysis unit (the four pinned packages/files; not a claimed architectural component). Source and Reffector evidence are independently selected per question using deterministic policies, then unioned for one request per arm; per-question attribution remains in the audit. The selectors report selected, deferred, and out-of-scope/not-inspected references, source declaration locations, coverage, uncertainty, and bounded retrieval logs. Lexical relevance or a fitting request is not semantic sufficiency. Control and treatment receive identical source and AST context; only treatment receives Reffector records. Both requests are preflighted atomically. A benchmark without `--model-command-json` is pending infrastructure setup, not comparative evidence. A fact with `citation_options` counts only when a lexically matching claim cites at least one configured, valid source line or Reffector record type. Lexical matches and citation checks are coverage measures, not semantic-accuracy scores; benchmark results require independent interpretation. No paid/remote model call is part of tests or implementation validation.
