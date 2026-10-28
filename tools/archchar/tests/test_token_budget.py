from __future__ import annotations

import json
import pathlib
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT))

from tools.archchar.core import invoke_model  # noqa: E402
from tools.archchar.token_budget import TokenBudgetError, run_token_counter  # noqa: E402


class TokenBudgetTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        self.log = self.root / "generation.log"

    def counter(self, input_tokens: int, context: int = 1000, output: int = 200) -> list[str]:
        script = self.root / f"counter-{input_tokens}-{context}-{output}.py"
        script.write_text(
            "import json, sys\n"
            "payload=json.load(sys.stdin)\n"
            f"print(json.dumps({{'input_tokens':{input_tokens},'token_count_type':'exact','model':'fake-model','context_window_tokens':{context},'max_output_tokens':{output}}}))\n",
            encoding="utf-8",
        )
        return [sys.executable, str(script)]

    def policy(self, context: int = 1000, output: int = 200, safety: int = 100) -> dict[str, int]:
        return {"context_window_tokens": context, "output_reserve_tokens": output, "safety_margin_tokens": safety}

    def model(self) -> list[str]:
        script = self.root / "model.py"
        script.write_text(
            "import json, pathlib, sys\n"
            f"pathlib.Path({str(self.log)!r}).write_text('called')\n"
            "print(json.dumps({'claims':[],'model':'fake-model'}))\n",
            encoding="utf-8",
        )
        return [sys.executable, str(script)]

    def test_exact_token_count_includes_output_and_safety_reserves(self) -> None:
        budget = run_token_counter(self.counter(600), {"source": "x"}, self.root, self.policy())
        self.assertEqual(budget["status"], "within_budget")
        self.assertEqual(budget["total_reserved_tokens"], 900)
        self.assertEqual(budget["token_count_type"], "exact")

    def test_over_budget_blocks_generation_before_model_process_starts(self) -> None:
        result = invoke_model(
            json.dumps(self.model()), {"phase": "component", "source": "x"}, self.root,
            token_counter_command_json=self.counter(701), token_policy=self.policy(),
        )
        self.assertEqual(result["status"], "failed")
        self.assertEqual(result["token_budget"]["status"], "over_budget")
        self.assertFalse(self.log.exists())

    def test_runtime_context_is_authoritative_for_preflight_fit(self) -> None:
        within = run_token_counter(self.counter(600, context=900), {}, self.root, self.policy())
        self.assertEqual(within["status"], "within_budget")
        self.assertEqual(within["declared_context_window_tokens"], 1000)
        self.assertEqual(within["context_window_tokens"], 900)
        over = run_token_counter(self.counter(801, context=900), {}, self.root, self.policy())
        self.assertEqual(over["status"], "over_budget")
        self.assertEqual(over["total_reserved_tokens"], 1101)

    def test_missing_runtime_limits_fail_closed(self) -> None:
        script = self.root / "incomplete-counter.py"
        script.write_text(
            "import json\n"
            "print(json.dumps({'input_tokens':1,'token_count_type':'exact','model':'fake-model'}))\n",
            encoding="utf-8",
        )
        with self.assertRaisesRegex(TokenBudgetError, "must report the runtime context window"):
            run_token_counter([sys.executable, str(script)], {}, self.root, self.policy())

    def test_missing_counter_fails_closed(self) -> None:
        with self.assertRaisesRegex(TokenBudgetError, "requires an explicit token-count command"):
            run_token_counter(None, {}, self.root, self.policy())


if __name__ == "__main__":
    unittest.main()
