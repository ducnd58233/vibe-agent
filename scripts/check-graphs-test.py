#!/usr/bin/env python3
"""Prove scripts/check-graphs.py rejects broken graphs.

A validator that has only ever seen a valid graph proves nothing. Each case
mutates the real goal-delivery.yaml into a known-bad shape, runs the checker
against a throwaway tree, and requires a non-zero exit.

Needs pyyaml and jsonschema (scripts/requirements.txt).

Usage: python3 scripts/check-graphs-test.py [repo-root]
"""

from __future__ import annotations

import copy
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

try:
    import yaml
except ImportError as exc:
    sys.exit(
        f"check-graphs-test: missing dependency ({exc.name}).\n"
        "Install with: python3 -m pip install -r scripts/requirements.txt"
    )


def unreachable_node(graph: dict) -> dict:
    graph["spec"]["nodes"]["orphan"] = {"type": "agent", "command": "build"}
    return graph


def cannot_reach_terminal(graph: dict) -> dict:
    graph["spec"]["nodes"]["spin_a"] = {"type": "agent", "command": "build"}
    graph["spec"]["nodes"]["spin_b"] = {"type": "agent", "command": "build"}
    graph["spec"]["edges"] += [
        {"from": "build", "to": "spin_a", "when": "blocked"},
        {"from": "spin_a", "to": "spin_b"},
        {"from": "spin_b", "to": "spin_a"},
    ]
    return graph


def undeclared_guard(graph: dict) -> dict:
    graph["spec"]["edges"].append({"from": "build", "to": "done", "when": "nobody_declared_me"})
    return graph


def expression_instead_of_guard(graph: dict) -> dict:
    graph["spec"]["edges"].append(
        {"from": "build", "to": "done", "when": "state.e2eRequired == false"}
    )
    return graph


def terminal_with_outgoing_edge(graph: dict) -> dict:
    graph["spec"]["edges"].append({"from": "done", "to": "build"})
    return graph


def two_unconditional_edges(graph: dict) -> dict:
    graph["spec"]["edges"].append({"from": "test", "to": "done"})
    graph["spec"]["edges"].append({"from": "test", "to": "failed"})
    return graph


def unknown_node_type(graph: dict) -> dict:
    graph["spec"]["nodes"]["waiter"] = {"type": "wait", "command": "poll"}
    graph["spec"]["edges"].append({"from": "build", "to": "waiter", "when": "blocked"})
    return graph


def id_filename_mismatch(graph: dict) -> dict:
    graph["metadata"]["id"] = "something-else"
    return graph


def human_gate_without_prompt(graph: dict) -> dict:
    del graph["spec"]["nodes"]["approve_spec"]["prompt"]
    return graph


def duplicate_check_writer(graph: dict) -> dict:
    # Two nodes writing checks.unit means one silently overwrites the other.
    graph["spec"]["nodes"]["second_test"] = {
        "type": "verifier",
        "verifier": "command",
        "check": "unit",
    }
    graph["spec"]["edges"].append({"from": "build", "to": "second_test", "when": "blocked"})
    graph["spec"]["edges"].append({"from": "second_test", "to": "done"})
    return graph


def guard_reads_missing_check(graph: dict) -> dict:
    for guard in graph["spec"]["guards"]:
        if guard["name"] == "unit_passed":
            guard["reads"] = "no_node_writes_this"
    return graph


def unused_guard(graph: dict) -> dict:
    graph["spec"]["guards"].append(
        {"name": "never_referenced", "description": "d", "source": "flag"}
    )
    return graph


MUTATIONS = [
    ("unreachable node", unreachable_node),
    ("node that cannot reach a terminal", cannot_reach_terminal),
    ("undeclared guard", undeclared_guard),
    ("expression instead of a guard name", expression_instead_of_guard),
    ("terminal with an outgoing edge", terminal_with_outgoing_edge),
    ("two unconditional edges from one node", two_unconditional_edges),
    ("unknown node type", unknown_node_type),
    ("graph id not matching the filename", id_filename_mismatch),
    ("human gate without a prompt", human_gate_without_prompt),
    ("two nodes writing the same check", duplicate_check_writer),
    ("guard reading a check no node produces", guard_reads_missing_check),
    ("guard declared but never used", unused_guard),
]


def run_checker(root: Path, checker: Path, graph: dict, name: str) -> tuple[int, str]:
    """Write the graph into a throwaway tree shaped like the repo and check it."""
    tmp = Path(tempfile.mkdtemp(prefix="check-graphs-test-"))
    try:
        (tmp / "schemas").mkdir()
        shutil.copy(
            root / "schemas/workflow-graph.schema.json",
            tmp / "schemas/workflow-graph.schema.json",
        )
        graphs_dir = tmp / ".ai-agents/graphs"
        graphs_dir.mkdir(parents=True)
        (graphs_dir / name).write_text(
            yaml.safe_dump(graph, sort_keys=False), encoding="utf-8"
        )
        proc = subprocess.run(
            [sys.executable, str(checker), str(tmp)],
            capture_output=True,
            text=True,
        )
        return proc.returncode, proc.stdout + proc.stderr
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    checker = root / "scripts/check-graphs.py"
    source = root / ".ai-agents/graphs/goal-delivery.yaml"

    if not checker.is_file() or not source.is_file():
        print("check-graphs-test: run from the toolkit repository root", file=sys.stderr)
        return 1

    good = yaml.safe_load(source.read_text(encoding="utf-8"))
    failures = 0

    for label, mutate in MUTATIONS:
        code, output = run_checker(root, checker, mutate(copy.deepcopy(good)), source.name)
        if code == 0:
            failures += 1
            print(f"  FAIL  checker accepted a graph with: {label}", file=sys.stderr)
        else:
            caught = next(
                (line.strip() for line in output.splitlines() if line.strip().startswith("FAIL")),
                "(no FAIL line)",
            )
            print(f"  ok    rejected {label}")
            print(f"          {caught}")

    code, output = run_checker(root, checker, copy.deepcopy(good), source.name)
    if code != 0:
        failures += 1
        print("  FAIL  checker rejected the unmodified graph", file=sys.stderr)
        print(output, file=sys.stderr)
    else:
        print("  ok    accepted the unmodified graph")

    print()
    if failures:
        print(f"check-graphs-test: FAILED ({failures} problems)", file=sys.stderr)
        return 1
    print(f"check-graphs-test: OK ({len(MUTATIONS) + 1} cases)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
