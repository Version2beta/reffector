"""Command-line entry point for the architectural characterization harness."""
from __future__ import annotations

import argparse
import json
import sys

from .core import ArchcharError, aggregate, benchmark, investigate, prepare, token_policy_from_args, verify


def positive_integer(value: str) -> int:
    number = int(value)
    if number <= 0:
        raise argparse.ArgumentTypeError("value must be greater than zero")
    return number


def nonnegative_integer(value: str) -> int:
    number = int(value)
    if number < 0:
        raise argparse.ArgumentTypeError("value must not be negative")
    return number


def add_token_budget_arguments(command: argparse.ArgumentParser) -> None:
    command.add_argument("--token-counter-command-json", help="count-only command argv JSON; must not generate model output")
    command.add_argument("--context-window-tokens", type=positive_integer)
    command.add_argument("--output-reserve-tokens", type=positive_integer)
    command.add_argument("--safety-margin-tokens", type=positive_integer)


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(prog="python3 -B -m tools.archchar")
    commands = root.add_subparsers(dest="command", required=True)

    for name in ("prepare", "run", "import"):
        command = commands.add_parser(name, help="analyze/import Reffector evidence and prepare bounded component packets")
        command.add_argument("repo", help="Go repository root")
        command.add_argument("--out", required=True, help="new artifact directory outside the analyzed repository")
        command.add_argument("--reffector", help="Reffector executable; defaults to PATH lookup")
        command.add_argument("--effects", help="existing effects JSON input (import mode)")
        command.add_argument("--audit", help="existing audit JSONL input (required with --effects)")
        command.add_argument("--pattern", action="append", default=None, help="Go package pattern; repeatable, defaults to ./...")
        command.add_argument("--max-sites", type=positive_integer, default=50000)
        command.add_argument("--max-edges", type=positive_integer, default=200000)
        command.add_argument("--max-depth", type=positive_integer, default=100)
        command.add_argument("--max-context-bytes", type=positive_integer, default=500000)
        command.add_argument("--max-retrieval-rounds", type=nonnegative_integer, default=2)
        command.add_argument("--max-retrieval-bytes", type=positive_integer, default=100000)
        add_token_budget_arguments(command)

    investigation = commands.add_parser("investigate", help="resume isolated model investigations from saved packets")
    investigation.add_argument("run", help="artifact directory from prepare/run/import")
    investigation.add_argument("--model-command-json", required=True, help="JSON argv array for the generic model subprocess")
    investigation.add_argument("--max-rounds", type=nonnegative_integer)
    investigation.add_argument("--max-retrieval-bytes", type=positive_integer)
    add_token_budget_arguments(investigation)

    aggregation = commands.add_parser("aggregate", help="aggregate file/package results into a deterministic hierarchy")
    aggregation.add_argument("run")
    aggregation.add_argument("--model-command-json", help="optional isolated model aggregator command")
    aggregation.add_argument("--max-context-bytes", type=positive_integer)
    add_token_budget_arguments(aggregation)

    verification = commands.add_parser("verify", help="validate evidence refs, confidence, coverage and aggregation ancestry")
    verification.add_argument("run")
    verification.add_argument("--model-command-json", help="optional isolated model verifier command")
    verification.add_argument("--max-context-bytes", type=positive_integer)
    add_token_budget_arguments(verification)

    paired = commands.add_parser("benchmark", help="run a paired source-only and Reffector-assisted comparison")
    paired.add_argument("run")
    paired.add_argument("--manifest", required=True, help="pre-registered questions and deterministic rubric JSON")
    paired.add_argument("--model-command-json", help="generic model subprocess argv JSON; omit for a pending dry run")
    paired.add_argument("--max-context-bytes", type=positive_integer)
    paired.add_argument("--reasoning-level", default="medium")
    paired.add_argument("--max-output-tokens", type=positive_integer, default=4000)
    paired.add_argument("--methodology", choices=("multi_stage_v1", "one_shot_paired_v1"), default="multi_stage_v1")
    add_token_budget_arguments(paired)
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        if args.command in ("prepare", "run", "import"):
            if args.command == "import" and (not args.effects or not args.audit):
                raise ArchcharError("import requires both --effects and --audit")
            if args.effects and not args.audit:
                raise ArchcharError("--audit is required when importing --effects")
            result = prepare(args)
            print(json.dumps({"status": "prepared", "run": str(result)}, ensure_ascii=False))
        elif args.command == "investigate":
            result = investigate(args.run, args.model_command_json, args.max_rounds, args.max_retrieval_bytes, args.token_counter_command_json, token_policy_from_args(args))
            print(json.dumps(result, ensure_ascii=False))
        elif args.command == "aggregate":
            result = aggregate(args.run, args.model_command_json, args.max_context_bytes, args.token_counter_command_json, token_policy_from_args(args))
            print(json.dumps({"status": result["status"], "semantic_aggregation": result["semantic_aggregation"]}, ensure_ascii=False))
        elif args.command == "verify":
            result = verify(args.run, args.model_command_json, args.max_context_bytes, args.token_counter_command_json, token_policy_from_args(args))
            print(json.dumps({"status": result["status"], "issues": len(result["issues"])}, ensure_ascii=False))
        elif args.command == "benchmark":
            result = benchmark(args.run, args.manifest, args.model_command_json, args.max_context_bytes, args.reasoning_level, args.max_output_tokens, args.token_counter_command_json, token_policy_from_args(args), args.methodology)
            print(json.dumps({"status": result["status"], "dataset": result["dataset"]}, ensure_ascii=False))
        return 0
    except (ArchcharError, OSError, ValueError, KeyError) as error:
        print(f"archchar: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
