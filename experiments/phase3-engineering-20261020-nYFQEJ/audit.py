#!/usr/bin/env python3
"""Read completed runs; add supervisor indexes without changing worker artifacts."""
import hashlib,json,pathlib
ROOT=pathlib.Path(__file__).resolve().parent

def main():
    overall={}
    for c in 'ABCDE':
        d=ROOT/c
        if not (d/'completion.json').exists(): raise RuntimeError(f'{c}: not completed')
        records=[]
        for line in (d/'events.jsonl').read_text().split('\n'):
            if line:
                try: records.append(json.loads(line))
                except ValueError: pass
        tools=[v for v in records if v.get('type')=='tool_execution_start']
        ends={v['toolCallId']:v for v in records if v.get('type')=='tool_execution_end'}
        queries=[]; lines=[f'# Supervisor evidence index — {c}', '', 'Exact tool calls and results remain in `events.jsonl`; this is a derived index.', '']
        lines += ['## Tool calls', '']
        writes=[]; reads=[]
        for i,t in enumerate(tools,1):
            a=t.get('args',{}); name=t['toolName']; end=ends.get(t['toolCallId'],{})
            lines.append(f'### {i}. {name}; error={end.get("isError", "incomplete")}; duration_ms={end.get("durationMs", "unknown")}')
            if name=='bash':
                command=a.get('command',''); lines += ['```sh',command,'```','']
                if '/reffector ' in command: queries.append(command)
            elif name=='read':
                reads.append(a); lines += ['```json',json.dumps(a),'```','']
            elif name=='write':
                p=a.get('path',''); writes.append(p)
                lines += [f'Artifact: `{p}`','']
            else: lines += ['```json',json.dumps(a),'```','']
        lines += ['## Analyzer artifact coverage', '']
        coverages={}
        for p in sorted(d.glob('*.json')):
            try: v=json.loads(p.read_text())
            except ValueError: continue
            if isinstance(v,dict) and 'coverage' in v:
                cov=v['coverage']; subset={k:cov[k] for k in ('packages_selected','source_functions','operation_sites_identified','selected_operation_sites','dependency_operation_sites','unresolved_sites_identified','sites_matching_filters','sites_emitted') if k in cov}
                data={'coverage':subset,'truncated':v.get('truncated'), 'emitted_sites':len(v.get('sites',[])), 'emitted_unresolved':len(v.get('unresolved_sites',[]))}
                coverages[p.name]=data
                lines += [f'### `{p.name}`','```json',json.dumps(data,indent=2),'```','']
        sessions=[]
        for p in sorted((d/'sessions').glob('*.jsonl')):
            state=[]
            for line in p.read_text().split('\n'):
                if not line: continue
                try: v=json.loads(line)
                except ValueError: continue
                if v.get('type') in ('session','model_change','thinking_level_change'): state.append(v)
            sessions.append({'file':str(p.relative_to(ROOT)), 'state_entries':state})
        comp=json.loads((d/'completion.json').read_text())
        data={'completion':comp,'session_metadata':sessions,'explicit_read_calls':reads,'write_paths':writes,
              'write_tools_within_case':all(pathlib.Path(p).resolve().is_relative_to(d) for p in writes),
              'analyzer_commands':queries,'analyzer_artifacts':coverages,
              'skill_read':any(pathlib.Path(v.get('path','')).resolve()==pathlib.Path('/Users/rob/.agents/skills/reffector/SKILL.md') for v in reads),
              'note':'Write-path check covers write tool only. Shell side effects need transcript/source-state review. Tokens are reported by provider; catalog cost is not verified billing.'}
        (d/'supervisor-evidence.md').write_text('\n'.join(lines)+'\n')
        overall[c]=data
    (ROOT/'audit.json').write_text(json.dumps(overall,indent=2)+'\n')
    hashes=[]
    for p in sorted(ROOT.rglob('*')):
        if p.is_file() and p.name!='SHA256SUMS' and '__pycache__' not in p.parts:
            hashes.append(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+str(p.relative_to(ROOT)))
    (ROOT/'SHA256SUMS').write_text('\n'.join(hashes)+'\n')
if __name__=='__main__': main()
