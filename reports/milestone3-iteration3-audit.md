# Milestone 3 audit sample — iteration 3

## Scope and reproducibility

This is a preliminary, source-grounded audit sample, not completion of the milestone gate. The available `/Users/rob/Development/rclone` checkout is at `b551a5c1688282eb2ee4274dff96e95d91d075f4`, not the required pinned revision `81e7372f6fbedfa80592e9a564593d63a8dc32f3`; it was not analyzed or modified. Instead, this bounded sample uses the current Reffector checkout (including its resolved Go module dependencies), keeping the task's existing source scope. No source root/manifest was supplied to the packet builder; source facts below were independently checked against the current working tree and module cache.

Reproduce from the repository root:

```sh
go build -o /tmp/reffector-audit ./cmd/reffector
/tmp/reffector-audit -json -all -max-depth 100 . > /tmp/effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
result = build(json.load(open('/tmp/effects.json')), max_candidates=100)
print(result['counts'])
PY
```

Observed EffectReport: 59 operation sites, 99 nodes, 238 edges; no report truncation. Candidate inventory: 57 of 59 sites (2 sites do not satisfy the current candidate-category rule). All 57 stable-ID candidates fell within the 100-ID selection limit and emitted packets: 0 `deferred_max_candidates`, 0 `deferred_over_budget`. This synthetic-ish small scope therefore cannot audit candidate-count deferrals or formerly size-deferred candidates. The packet builder had no source manifest, so emitted packet statuses are not evidence that source excerpts were available.

Category denominator (candidates may occur in multiple categories): `cross-component-shared-operation-site` 57/57; `multi-frontier-reachability` 52/57; `branching-convergence` 52/57; `async-edges` 7/57; `candidate-unresolved-relationships` 6/57. Effect labels across the 59 report sites: `fs.read` 9, `fs.create` 9, `concurrency.lock` 7, `concurrency.unlock` 7, `fs.write` 6, `env.read` 5, `concurrency.wait` 5, `concurrency.task` 4, `channel.receive` 3, `concurrency.atomic` 3, `channel.send` 2, `concurrency.goroutine` 2, and one each of `process.exit`, `process.exec`, `fs.remove`, `channel.select`, `channel.close`.

## Deterministic sample rule

From the complete stable-ID-sorted candidate inventory, take (a) the largest candidate by edge count, (b) the lexicographically first candidate ID containing both `async-edges` and `candidate-unresolved-relationships`, and (c) the lexicographically first `process.exec` candidate. These roles yielded three distinct candidates; the chosen IDs and their full inventory records remain reproducible from the command above. This is purposive feature coverage, not a random or prevalence-representative sample. It did not yield a direct candidate-only call-only case with no established async edge; the unresolved/candidate relation case should not be misreported as that.

## Source checks: mechanical facts vs inference

1. **Largest graph / async plus unresolved candidate:** `candidate:d61cb9715e8e6ca08aec1c8035f7e6f0442e92e3847afe0d09fae63da7c94205`, site `site:854353ab6c24302ab51c183d8f83d0f654b15e623dffbdc7f04e54419aa2217e`, `(*Package).Build` → `sync.(*Once).Do` (`concurrency.wait`); inventory has 52 distinct nodes and 165 relationships. In `$(go list -f '{{.Dir}}' golang.org/x/tools/go/ssa)/builder.go:3148-3157`, source explicitly performs `wg.Add(1)`, starts `go func`, calls `p.Build()`, `wg.Done()`, releases a semaphore token, and then `wg.Wait()`. The target is at `builder.go:3172` (`p.buildOnce.Do(p.build)`). The report also contains `candidate_vta` relationships, which remain candidate edges, not established runtime calls. **Inference/unknown:** graph size and edge categories do not establish a feasible path or a defect; this is a useful provenance/CFG challenge candidate, not a reported bug.

2. **Candidate-unresolved + async cleanup:** `candidate:3e6912d4ffc8d1c4921236f8340e8003057cab19a109be352fb707e9aa01493f`, site `site:caabda1e10a0a2a60bb7e87599f7085384e6c85afe1e6891e1b604d234273737`, `loadConfig$3` → `os.RemoveAll` (`fs.remove`), 6 nodes/8 edges. In `internal/analyzer/loading.go:74-82`, `loadConfig` creates a temporary directory and defines `cleanup := func() { _ = os.RemoveAll(tmp) }`; cleanup is called on a write failure. The selected operation is inside the closure, while a deferred/candidate-function-value relationship is not proof that every control-flow path registers or executes it. **Potential hazard/hypothesis:** ignored cleanup errors and cleanup timing could merit inspection; this source alone does not establish a leak, unsafe deletion, or reachable defect. The temp path originates from `os.MkdirTemp` and is joined with `go.work` at lines 74, 79.

3. **Different effect category (process execution):** `candidate:7702b3ea4289d2ed760ef4336eeffc179567001708182276f46e7d488e142071`, site `site:9987f9efdcc4bb8dc49de643245b0d8c59e2ae65096ec93c4253a7046d663bb7`, `loadConfig` → `os/exec.(*Cmd).Output` (`process.exec`), 5 nodes/4 edges. `internal/analyzer/loading.go:22-26` constructs `exec.Command("go", "env", "-json", "GOWORK")` and invokes `cmd.Output()`, returning an error if the command fails. **False lead/control:** an effect label for executing a child process is mechanically accurate; it does not imply attacker-controlled command selection or unsafe shell invocation. The literal executable and arguments are visible here; environmental influence and deployment behavior were not investigated.

## Bias, false leads, and unknowns

This sample overrepresents graph-rich Go dependencies: package cross-boundary structure is a category on every candidate, although that label is not evidence of architectural components. The sample is intentionally skewed toward edge count, async/unresolved evidence, and process execution. A whole-repository EffectReport includes dependencies and test/build-related code, so dependency operations are not automatically application-relevant. `candidate_vta` must remain separate from established call evidence. The current analyzer's candidate IDs/category labels alone do not indicate severity or vulnerability.

The sample has no count- or byte-deferred candidates, does not cover high fan-in separately from broad graph size, and only lightly samples effect kinds. There is no pinned-rclone comparison in this iteration: the available checkout revision mismatches the required pin. No model/provider was invoked. These limitations prevent passing the milestone-3 diverse audit gate or concluding selection quality is adequate. No selection algorithm change is justified by this small biased sample.
