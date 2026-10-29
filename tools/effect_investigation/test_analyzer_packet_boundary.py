"""End-to-end checks that analyzer uncertainty survives investigation packets."""
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest

from investigate import CONTROL_FLOW_STRUCTURAL_LIMITATION, build


REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]


@unittest.skipUnless(shutil.which("go"), "Go is required for the analyzer integration test")
class AnalyzerPacketBoundaryTests(unittest.TestCase):
    def test_candidate_effect_and_cfg_evidence_stay_unpromoted(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            binary = root / "reffector"
            fixture = root / "fixture"
            fixture.mkdir()
            (fixture / "go.mod").write_text(
                "module example.test/evidence-boundary\n\ngo 1.26.0\n", encoding="utf-8")
            (fixture / "boundary.go").write_text(
                '''package boundary

import "os"

func maybe(flag bool) {
\tcallback := os.Chdir
\tif flag {
\t\tcallback = os.Remove
\t}
\t_ = callback("target")
}

func invoke(callback func(bool), flag bool) { callback(flag) }
func root(flag bool) { invoke(maybe, flag) }
''', encoding="utf-8")

            env = os.environ.copy()
            env.update({"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local"})
            built = subprocess.run(
                ["go", "build", "-o", str(binary), "./cmd/reffector"],
                cwd=REPO_ROOT, env=env, text=True, capture_output=True, timeout=120)
            self.assertEqual(built.returncode, 0, built.stderr)

            def analyze(*flags):
                completed = subprocess.run(
                    [str(binary), "-exact", *flags, "."], cwd=fixture, env=env,
                    text=True, capture_output=True, timeout=60)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                return json.loads(completed.stdout)

            report = analyze("-json")
            provenance = analyze("-argument-provenance")
            control_flow = analyze("-control-flow", "-operation", "os.Remove")

            site = next(site for site in report["sites"]
                        if site["function"] == "maybe" and "os.Remove" in site["apis"])
            remove = next(operation for operation in site["operations"]
                          if operation["api"] == "os.Remove")
            self.assertEqual(site["effects"], [])
            self.assertIn("fs.remove", site["possible_effects"])
            self.assertEqual(remove["effects"], [])
            self.assertEqual(remove["recognition"], "candidate_function_value")

            candidate_edge = next(edge for edge in report["edges"]
                                  if edge["source"]["caller"]["function"] == "invoke"
                                  and edge["source"]["callee"]["function"] == "maybe")
            self.assertTrue(candidate_edge["candidate"])
            self.assertEqual(candidate_edge["source"]["resolution"], "candidate_function_value")

            analyzer_row = next(row for row in control_flow["rows"]
                                if row["site"] == site["id"] and row["api"] == "os.Remove")
            self.assertEqual(analyzer_row["resolution"], "candidate_function_value")
            self.assertEqual(len(analyzer_row["branches"]), 1)
            branch = analyzer_row["branches"][0]
            self.assertEqual(branch["condition_kind"], "ssa.If")
            self.assertTrue(branch["controls"])
            self.assertTrue(all(control["block"] != analyzer_row["block"]
                                for control in branch["controls"]))

            result = build(report, argument_provenance=provenance,
                           control_flow=control_flow, max_packet_bytes=100_000)
            packet = next(packet for packet in result["packets"]
                          if packet["operation"]["id"] == site["id"])
            self.assertEqual(packet["operation"]["effects"], [])
            self.assertIn("fs.remove", packet["operation"]["possible_effects"])

            packet_edge = next(edge for edge in packet["graph"]["edges"]
                               if edge["caller"]["function"] == "invoke"
                               and edge["callee"]["function"] == "maybe")
            self.assertTrue(packet_edge["candidate"])
            self.assertEqual(packet_edge["resolution"], "candidate_function_value")

            packet_provenance = next(row for row in packet["argument_provenance"]["sites"]
                                     if row["api"] == "os.Remove")
            self.assertEqual(packet_provenance["resolution"], "candidate_function_value")

            packet_control = packet["control_flow"]
            self.assertIn(CONTROL_FLOW_STRUCTURAL_LIMITATION, packet_control["limitations"])
            packet_row = next(row for row in packet_control["rows"] if row["api"] == "os.Remove")
            if "branch_count" in packet_row:
                self.assertEqual(packet_row["branch_count"], 1)
                self.assertEqual(packet_row["controlling_branch_count"], 0)
            else:
                self.assertEqual(len(packet_row["branches"]), 1)
                self.assertNotEqual(packet_row["branches"][0]["controls"][0]["block"],
                                    packet_row["block"])
            self.assertNotIn("reachable", packet_row)
            self.assertNotIn("path_condition", packet_row)


if __name__ == "__main__":
    unittest.main()
