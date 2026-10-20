#!/usr/bin/env python3
"""Five fresh Pi CLI processes. No continuation, forks, or inherited conversations."""
import concurrent.futures, datetime, hashlib, json, os, pathlib, shutil, signal, subprocess, time
ROOT = pathlib.Path(__file__).resolve().parent
PROJECT = ROOT.parent.parent
PI = '/Users/rob/.pi/agent/bin/pi'
SKILL = pathlib.Path('/Users/rob/.agents/skills/reffector/SKILL.md')
BIN = ROOT / 'bin/reffector'
CASES = {
'A': ('caddy', 'Architectural simplification', '''Find the most defensible opportunity, if one exists, to reduce independently implemented correctness obligations associated with consequential HTTP operations. Do not prefer fewer operation sites as an end in itself. Compare shared implementations with superficially similar but legitimate independent responsibilities. Produce a concrete, source-anchored refactoring proposal and behavioral-preservation plan. If no useful consolidation is found, explain why.'''),
'B': ('restic', 'Side-effect isolation', '''Find a meaningful location where computation, decisions, and consequential filesystem/backend operations are entangled. Assess whether separating them would improve reasoning, testability, or ownership without creating a misleading abstraction or unsafe time-of-check/time-of-use split. Propose the smallest change worth making, or reject the opportunity with evidence. Do not assume purity is always better.'''),
'C': ('caddy', 'Change-impact analysis', '''Hypothetical requirement: add an X-Audit-Trace response header consistently to responses produced by reverse-proxy behavior, including normal, error, handled-response, and protocol-upgrade paths where HTTP semantics permit that header. Do not implement. Enumerate independently relevant emission paths, callers, exceptional cases, and necessary tests. State uncertain coverage and cases outside the chosen package scope. This is impact analysis, not a claim that such a header is advisable.'''),
'D': ('restic', 'Test design', '''Choose one consequential local-backend persistence or replacement boundary. Use Reffector evidence and source inspection to design the smallest set of behavioral and failure-injection tests that meaningfully increase confidence in preservation, cleanup, and retry behavior. Distinguish existing tests from new proposals. Do not count ideas as coverage or claim defects without reproduction.'''),
'E': ('restic', 'Code review', '''Find a suitably bounded historical commit at or before the pinned revision that changes consequential code in the selected scope. Use git log / git show read-only; record the exact SHA and preserve its exact diff in your artifact directory. Review the change using current Reffector source-site and reverse-caller evidence plus ordinary code inspection. Assess effects added, removed, moved, or reached by different callers. Report meaningful risks and unknowns. Reffector is not assumed to have a revision-diff feature. Do not check out history or analyze an altered worktree; distinguish current pinned-tree evidence from historical diff evidence.'''),
}
PINS = {'caddy': '0e392c89baa50f558bc24440482c5d57fff4499f', 'restic': 'dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4'}
SCOPES = {'caddy': './modules/caddyhttp/...', 'restic': './internal/repository/... ./internal/backend/local/...'}
def sha(path): return hashlib.sha256(path.read_bytes()).hexdigest()
def cmd(args, cwd=PROJECT):
    return subprocess.run(args, cwd=cwd, capture_output=True, text=True, check=True).stdout

def prepare():
    (ROOT / 'bin').mkdir(exist_ok=True)
    if not BIN.exists(): shutil.copy2('/Users/rob/go/bin/reffector', BIN)
    if not (ROOT/'skill.md').exists(): shutil.copy2(SKILL, ROOT/'skill.md')
    metadata = {'created_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'reffector_binary': str(BIN), 'binary_sha256': sha(BIN), 'installed_binary_sha256': sha(pathlib.Path('/Users/rob/go/bin/reffector')),
        'build_info': cmd(['go','version','-m',str(BIN)]), 'source_head':cmd(['git','rev-parse','HEAD']).strip(),
        'source_status':cmd(['git','status','--short']), 'skill_path':str(SKILL),'skill_sha256':sha(SKILL),
        'model_requested':'openai-codex/gpt-6.1-sol','thinking_requested':'high','pi_version':cmd([PI,'--offline','--no-extensions','--no-mcp','--version']).strip(),
        'context':'Fresh processes; default Pi system prompt + explicit global AGENTS.md; Caddy additionally explicit Caddy AGENTS.md. No discovered context, extensions, MCP, prompt templates or other skills. Only read,bash,write tools. No prior reports or findings supplied.',
        'source_diff_sha256': hashlib.sha256(subprocess.check_output(['git','diff','--binary','--','.',':!experiments'],cwd=PROJECT)).hexdigest()}
    sources = {}
    for p in list((PROJECT/'cmd').rglob('*.go')) + list((PROJECT/'internal').rglob('*.go')) + [PROJECT/'go.mod', PROJECT/'go.sum',PROJECT/'skills/reffector/SKILL.md']:
        sources[str(p.relative_to(PROJECT))] = sha(p)
    metadata['source_file_sha256'] = sources
    for repo in PINS:
        wd=PROJECT/'experiments'/repo
        actual=cmd(['git','rev-parse','HEAD'],wd).strip(); status=cmd(['git','status','--porcelain=v1'],wd)
        if actual != PINS[repo] or status: raise RuntimeError(f'BLOCKER {repo}: head={actual}, status={status!r}')
    (ROOT/'preflight.json').write_text(json.dumps(metadata,indent=2)+'\n')
    (ROOT/'reffector-help.txt').write_text(subprocess.run([str(BIN),'-help'],capture_output=True,text=True).stderr)
    for case,(repo,title,objective) in CASES.items():
        d=ROOT/case; d.mkdir(exist_ok=True)
        prompt=f'''# Phase 3 / Case {case} — {title}

This is an independent, read-only exploratory engineering evaluation, NOT a benchmark or vulnerability hunt. A justified no-change decision is useful. You are the execution worker; execute directly (do not delegate).

Repository/cwd: {PROJECT/'experiments'/repo}
Required pinned HEAD: {PINS[repo]}
Required analyzer package scope: {SCOPES[repo]}
Exact frozen current locally installed executable: {BIN}
Expected executable SHA256: {sha(BIN)}
Reffector source base: 5553a1d13011bf638148661f6a23bf561c3eef5a, dirty Phase 1/2 source; this is not a clean-revision build. Inspect go version -m for actual build provenance.
Required Phase 2 skill: {SKILL} (phase2-1, SHA256 {sha(SKILL)}). Read it fully before analysis. It is the only advertised skill.
Your writable artifact directory ONLY: {d}
You may read {ROOT/'preflight.json'}, but not other cases or prior reports.

## Objective
{objective}

## Common evaluation contract
1. Verify HEAD and a clean status before analysis; if dirty/wrong, report blocker, never reset/checkout/clean/overwrite. Confirm actual model/provider/reasoning/session identity via printf of PI_PROVIDER, PI_MODEL, PI_REASONING_LEVEL, PI_SESSION_ID, PI_SESSION_FILE (no broad environment dump). Confirm the skill, go version, binary hash/build info and actual -help. Record loaded context (default Pi + global AGENTS.md{' + Caddy AGENTS.md' if repo=='caddy' else ''}, explicit skill only; read,bash,write tools; no extensions/MCP or discovered context). Do not infer model identity solely from provider metadata; these identify the selected Pi model, not an independently attested backend model.
2. Set GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly for EVERY Go/Reffector load. Dependencies are prewarmed. Use analyzer loading as preflight when appropriate; no dependency downloads, environment repair, module/workspace edits, running the analyzed app, or broad tests. If required scope cannot load, the run is INVALID; report errors, not zero effects or a silently narrower scope.
3. Quickly survey effect groups and direct sites with SELECTIVE queries, then inspect source/callers/invariants/tests/failure paths. Use documented flags only; record exact commands, cwd, build settings, output limits, load status, selected-vs-dependency coverage, uncertainty, depth/edges/sites truncation. Persist raw analyzer outputs under your artifact directory; do not overwhelm context with whole-repository dumps. Multiple callers can share one direct site; categories do not establish correctness or justify consolidation. Search for reasons NOT to recommend a change.
4. Prioritize strongest likely engineering payoff, not frequency. Reprioritize/repeat if time permits; preserve a candidate ledger in {d/'candidates.md'} after each meaningful lead (explored/rejected/unresolved with reasons). Do not stop after an arbitrary number of positive findings. No idling to meet a time limit.
5. Five-minute wall-clock budget starts at process launch: read {d/'timing.json'} for actual start, investigation deadline (+240 seconds), report deadline (+300 seconds). Check clock between meaningful steps. Reserve last 60 seconds for reporting and stop new investigation at investigation deadline. The supervisor enforces a 300-second cap. Save notes early so partial runs remain usable.
6. Effects are NOT defects; proposals are NOT verified improvements; test ideas are NOT coverage. Mark important claims [R] directly in analyzer output, [F] focused source follow-up prompted by output, or [I] independently discovered in source. Distinguish observation, inference, uncertainty and what might falsify the proposal.
7. Third-party trees, Reffector source and skills are READ-ONLY. Only write your own new artifacts under {d}. No commits, pushes, remote changes, worktree mutations, installs, repairs, network dependency loads or source edits. Never read earlier Reffector benchmark reports, known example defects, other agents' findings or other case artifacts. All investigation source inspection must stay within this repository, its pinned history, relevant dependencies and required tool/skill provenance. Do not inspect parent reports/HANDOFF files. Tool write access is for your artifact notes/report ONLY.

## Short final Markdown report
Write {d/'report.md'} before the deadline, then give its concise outcome as final response. Include:
- Case, exact repository SHA/scope, binary revision/hash/dirty provenance and commands, skill used, actual Pi model/provider/reasoning/context, elapsed time and completion status.
- Prioritized candidates (including rejections), objective, exact API/file:line/site IDs and caller relationships; shared vs independent paths; uncertainty, coverage, graph-depth and truncation limitations.
- Strongest supported recommendation or justified no-change outcome; concrete before/after responsibilities and independent correctness obligations where relevant; tradeoffs and plausible falsifiers.
- Existing tests vs proposed validation; no unexecuted-test success claims.
- Evidence provenance ledger [R]/[F]/[I]; source files inspected, tool calls (transcript can provide exact commands/counts), tokens/cost only if reliably available, unexplored areas.
Keep the report focused: roughly 800–1300 words maximum, fewer when evidence is narrow. Record the binary command lines literally, not just flag summaries. Preserve historical diff for E. Do not implement anything.
'''
        (d/'prompt.md').write_text(prompt)
        (d/'prompt.sha256').write_text(sha(d/'prompt.md')+'  prompt.md\n')
    (ROOT/'QUICKSTART.md').write_text('''# Phase 3 quickstart

`python3 run.py` launches five independent Pi CLI sessions concurrently, each with an explicit prompt, fresh persistent session storage, and raw JSONL/stderr capture. Do not rerun in this directory: the launcher refuses existing run metadata. Copy this runner to a NEW artifact directory first; it generates five standalone `A/prompt.md` through `E/prompt.md` files.

Each per-case `launch.json` records the complete runnable Pi argv, cwd, safe environment overrides, deadlines and PID. For a manual run, use that argv from its cwd, redirect stdout/stderr into new artifact files, and update `timing.json` to the new start/deadlines. Do not use --continue/--resume/--fork. Preserve existing artifacts.

The launcher copies the installed binary byte-for-byte, records build provenance and source/skill fingerprints, verifies repository pins/cleanliness, and reads no earlier reports. Go loading is offline; Pi model inference still uses its configured provider. Requested model is openai-codex/gpt-6.1-sol, reasoning high. Actual model and effective reasoning must be checked in session records and worker shell output.

Read-only investigation is a prompt constraint, NOT an OS sandbox: bash can write. Source invariants are checked before/after. Extensions/MCP and automatic context discovery are disabled. Global instructions and (for Caddy) its AGENTS.md are appended explicitly; only Phase 2 Reffector skill is loaded. No cross-case findings are supplied.

Five-minute cap includes tool/runtime/report writing. Final minute is reserved by prompt; this JSON-mode CLI cannot inject deadline steering. Notes and all streamed events survive interruption. `completion.json` distinguishes process exit, assistant completion and report presence; a zero process exit alone is not proof of successful analysis. No automated follow-up investigation is performed on timeout.
''')

def execute(case):
    repo=CASES[case][0]; d=ROOT/case
    if (d/'launch.json').exists(): raise RuntimeError(f'Refusing to overwrite existing run {case}')
    args=[PI,'--offline','--no-extensions','--no-mcp','--no-skills','--skill',str(SKILL),
          '--no-prompt-templates','--no-themes','--no-context-files','--no-approve',
          '--append-system-prompt','/Users/rob/.pi/agent/AGENTS.md']
    if repo=='caddy': args += ['--append-system-prompt',str(PROJECT/'experiments/caddy/AGENTS.md')]
    args += ['--provider','openai-codex','--model','gpt-6.1-sol','--thinking','high',
             '--tools','read,bash,write','--mode','json','--session-dir',str(d/'sessions'),
             '--name',f'Phase3-{case}', '@'+str(d/'prompt.md')]
    env=os.environ.copy()
    for key in ['PI_SESSION_ID','PI_SESSION_FILE','PI_PROVIDER','PI_MODEL','PI_REASONING_LEVEL']:
        env.pop(key,None)
    overrides={'GOPROXY':'off','GOSUMDB':'off','GOTOOLCHAIN':'local','GOFLAGS':'-mod=readonly','PI_OFFLINE':'1'}
    env.update(overrides)
    start=time.time(); mono=time.monotonic()
    timing={'start_epoch':start,'investigation_deadline_epoch':start+240,'report_deadline_epoch':start+300}
    (d/'timing.json').write_text(json.dumps(timing,indent=2)+'\n')
    with (d/'events.jsonl').open('wb') as out, (d/'stderr.txt').open('wb') as err:
        p=subprocess.Popen(args,cwd=PROJECT/'experiments'/repo,env=env,stdin=subprocess.DEVNULL,stdout=out,stderr=err,start_new_session=True)
        (d/'launch.json').write_text(json.dumps({'argv':args,'cwd':str(PROJECT/'experiments'/repo),'environment_overrides':overrides,'pid':p.pid,**timing},indent=2)+'\n')
        timeout=False
        try: code=p.wait(timeout=300)
        except subprocess.TimeoutExpired:
            timeout=True
            os.killpg(p.pid,signal.SIGTERM)
            try: code=p.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(p.pid,signal.SIGKILL); code=p.wait()
    records=[]; malformed=0
    for line in (d/'events.jsonl').read_text().split('\n'):
        if not line: continue
        try: records.append(json.loads(line))
        except json.JSONDecodeError: malformed+=1
    messages=[r['message'] for r in records if r.get('type')=='message_end' and r.get('message',{}).get('role')=='assistant']
    finals=[m for m in messages if m.get('stopReason')=='stop']
    if finals:
        text='\n'.join(x.get('text','') for x in finals[-1].get('content',[]) if x.get('type')=='text')
        (d/'final-response.md').write_text(text+'\n')
    usage=[m.get('usage',{}) for m in messages]
    result={'case':case,'elapsed_seconds':round(time.monotonic()-mono,3),'exit_status':code,'timed_out':timeout,
        'report_present':(d/'report.md').exists(),'assistant_stop_reasons':[m.get('stopReason') for m in messages],
        'settled_events':[r for r in records if r.get('type')=='agent_settled'], 'malformed_jsonl_records':malformed,
        'actual_message_models':sorted(set((m.get('provider',''),m.get('model','')) for m in messages)),
        'tool_calls':sum(r.get('type')=='tool_execution_start' for r in records),
        'provider_reported_total_tokens':sum(u.get('totalTokens',0) for u in usage),
        'catalog_cost_total_not_billing_evidence':sum(u.get('cost',{}).get('total',0) for u in usage)}
    (d/'completion.json').write_text(json.dumps(result,indent=2)+'\n')
    return result

def main():
    if any((ROOT/c/'launch.json').exists() for c in CASES): raise RuntimeError('Refusing to overwrite original runs')
    prepare()
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as ex:
        results=list(ex.map(execute,CASES))
    post={'finished_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(), 'cases':results,
          'repositories':{repo:{'head':cmd(['git','rev-parse','HEAD'],PROJECT/'experiments'/repo).strip(),
             'status':cmd(['git','status','--porcelain=v1'],PROJECT/'experiments'/repo)} for repo in PINS},
          'binary_sha256':sha(BIN),'installed_binary_sha256':sha(pathlib.Path('/Users/rob/go/bin/reffector')),'skill_sha256':sha(SKILL),
          'source_diff_sha256':hashlib.sha256(subprocess.check_output(['git','diff','--binary','--','.',':!experiments'],cwd=PROJECT)).hexdigest()}
    pre=json.loads((ROOT/'preflight.json').read_text())
    post['source_fingerprints_unchanged']=all((PROJECT/p).exists() and sha(PROJECT/p)==h for p,h in pre['source_file_sha256'].items())
    (ROOT/'postflight.json').write_text(json.dumps(post,indent=2)+'\n')
if __name__=='__main__': main()
