"""Offline supervisor tests: no real Pi/provider calls."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SOURCE = Path(__file__).with_name("ralph.py")


class SupervisorTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name)
        self.loop = self.path / "loop"
        self.loop.mkdir()
        self.repo = self.path / "repo"
        (self.repo / "internal" / "analyzer").mkdir(parents=True)
        (self.repo / "go.mod").write_text("module example.test/reffector\n")
        for filename in ("ralph.py", "TASK.md", "state.md"):
            (self.loop / filename).write_bytes(SOURCE.with_name(filename).read_bytes())
        self.fake = self.path / "fake_pi.py"
        self.fake.write_text("""#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args=sys.argv
prov=args[args.index('--provider')+1]
model=args[args.index('--model')+1]
Path(os.environ['FAKE_TRACE']).open('a').write(prov+':'+model+'\\n')
print(json.dumps({'type':'session','version':3,'id':'test'}))
print(json.dumps({'type':'message_end','message':{'role':'assistant','provider':prov,'model':model,'stopReason':'stop','content':[{'type':'text','text':'OK'}]}}))
print(json.dumps({'type':'agent_settled','aborted':False}))
""")
        self.fake.chmod(0o755)

    def invoke(self, *flags, env=None):
        import os
        e = dict(os.environ)
        e['FAKE_TRACE'] = str(self.path / "trace.txt")
        if env: e.update(env)
        return subprocess.run([sys.executable, str(self.loop / 'ralph.py'), '--repo', str(self.repo),
                               '--model', 'test-codex', '--pi', str(self.fake), *flags],
                              text=True, capture_output=True, env=e, timeout=20)

    def test_dry_run_no_state(self):
        r=self.invoke('--dry-run')
        self.assertEqual(r.returncode,0,r.stderr)
        self.assertIn('--provider openai-codex',r.stdout)
        self.assertFalse((self.loop/'run_state.json').exists())

    def test_one_iteration_and_resume_does_not_restart(self):
        r=self.invoke('--iterations','1','--minutes','0.1','--hours','0.01')
        self.assertEqual(r.returncode,0,r.stderr)
        state=json.loads((self.loop/'run_state.json').read_text())
        self.assertEqual(state['attempts'],1)
        self.assertEqual((self.path/'trace.txt').read_text().strip(),'openai-codex:test-codex')
        r=self.invoke('--iterations','1','--minutes','0.1','--hours','0.01')
        self.assertEqual(r.returncode,0,r.stderr)
        self.assertEqual(len((self.path/'trace.txt').read_text().splitlines()),1)

    def test_custom_provider_is_passed_and_verified(self):
        r=self.invoke('--provider','anthropic','--model','anthropic/test-model',
                      '--iterations','1','--minutes','0.1','--hours','0.01')
        self.assertEqual(r.returncode,0,r.stderr)
        self.assertEqual((self.path/'trace.txt').read_text().strip(),'anthropic:test-model')
        state=json.loads((self.loop/'run_state.json').read_text())
        self.assertEqual(state['provider'],'anthropic')
        self.assertEqual(state['model'],'test-model')

    def test_preexisting_deadline_not_extended(self):
        r=self.invoke('--iterations','2','--minutes','0.1','--hours','0.01')
        self.assertEqual(r.returncode,0,r.stderr)
        state=json.loads((self.loop/'run_state.json').read_text())
        self.assertEqual(state['attempts'],2)
        r=self.invoke('--iterations','2','--minutes','0.1','--hours','8')
        self.assertEqual(r.returncode,0,r.stderr)
        self.assertEqual(json.loads((self.loop/'run_state.json').read_text())['deadline_at'],state['deadline_at'])

    def test_reject_wrong_provider_response(self):
        text=self.fake.read_text().replace('prov=args[args.index(\'--provider\')+1]','prov="anthropic"')
        self.fake.write_text(text)
        r=self.invoke('--iterations','2','--minutes','0.1','--hours','0.01')
        self.assertNotEqual(r.returncode,0)
        self.assertIn('provider/model not verified',(self.loop/'BLOCKED').read_text())
        self.assertEqual(json.loads((self.loop/'run_state.json').read_text())['attempts'],1)

    def test_elapsed_deadline_marks_timebox_only(self):
        from datetime import datetime, timedelta, timezone
        past=(datetime.now(timezone.utc)-timedelta(seconds=5)).isoformat()
        (self.loop/'run_state.json').write_text(json.dumps({
            'repo':str(self.repo.resolve()),'provider':'openai-codex','model':'test-codex',
            'deadline_at':past,'iterations_max':4,'attempts':1}))
        r=self.invoke('--iterations','4')
        self.assertEqual(r.returncode,0,r.stderr)
        self.assertTrue((self.loop/'TIMEBOX_EXPIRED').exists())
        self.assertFalse((self.loop/'DONE').exists())


    def test_stops_after_no_progress(self):
        r=self.invoke('--iterations','6','--minutes','0.1','--hours','0.01',
                      '--max-no-progress','2')
        self.assertNotEqual(r.returncode,0)
        self.assertIn('No state.md handoff change', (self.loop/'BLOCKED').read_text())
        state=json.loads((self.loop/'run_state.json').read_text())
        self.assertEqual(state['attempts'], 2)

    def test_done_marker_prevents_launch(self):
        (self.loop/'DONE').write_text('completed with evidence')
        r=self.invoke('--iterations','3')
        self.assertNotEqual(r.returncode, 0)
        self.assertFalse((self.path/'trace.txt').exists())

if __name__=='__main__':
    unittest.main()
