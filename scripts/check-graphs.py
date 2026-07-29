#!/usr/bin/env python3
"""Validate every workflow graph under .ai-agents/graphs/.

Two layers:

1. JSON Schema, from schemas/workflow-graph.schema.json. Catches shape errors:
   unknown node types, missing required fields, expression syntax where a guard
   name belongs.
2. Structural invariants a schema cannot express: reachability, termination,
   declared-but-unused guards, fallback-edge uniqueness.

Needs pyyaml and jsonschema (scripts/requirements.txt). The router check in
check-ai-agents-routers.sh stays dependency-free on purpose; this one does not,
because hand-parsing YAML to avoid two well-known packages trades a small
install for a large class of parser bugs.

Usage: python3 scripts/check-graphs.py [repo-root]
"""

from __future__ import annotations

import json
import sys
from collections import deque
from pathlib import Path

try:
    import yaml
    from jsonschema import Draft202012Validator
except ImportError as exc:
    sys.exit(
        f"check-graphs: missing dependency ({exc.name}).\n"
        "Install with: python3 -m pip install -r scripts/requirements.txt"
    )


class Report:
    """Collects verdicts so a silent skip is impossible to hide."""

    def __init__(self, label: str) -> None:
        self.label = label
        self.failures: list[str] = []

    def check(self, name: str, ok: bool, detail: str = "") -> None:
        if ok:
            print(f"  ok    {name}")
        else:
            self.failures.append(name)
            print(f"  FAIL  {name}" + (f": {detail}" if detail else ""), file=sys.stderr)


def guard_name(condition: str) -> str:
    return condition.lstrip("!")


def reachable_from(start: str, adjacency: dict[str, list[str]]) -> set[str]:
    seen = {start}
    queue = deque([start])
    while queue:
        for nxt in adjacency.get(queue.popleft(), []):
            if nxt not in seen:
                seen.add(nxt)
                queue.append(nxt)
    return seen


def validate_graph(path: Path, schema: dict) -> list[str]:
    print(f"\n[{path.as_posix()}]")
    report = Report(path.name)

    try:
        graph = yaml.safe_load(path.read_text(encoding="utf-8"))
    except yaml.YAMLError as exc:
        report.check("parses as YAML", False, str(exc).splitlines()[0])
        return report.failures

    errors = list(Draft202012Validator(schema).iter_errors(graph))
    report.check(
        "schema validation",
        not errors,
        "; ".join(f"{list(e.path)}: {e.message}" for e in errors[:4]),
    )
    if errors:
        # Structural checks below assume a well-shaped graph.
        return report.failures

    spec = graph["spec"]
    nodes: dict[str, dict] = spec["nodes"]
    edges: list[dict] = spec["edges"]
    declared_guards = {g["name"] for g in spec.get("guards", [])}
    initial = spec["initial"]

    report.check(
        "graph id matches filename",
        graph["metadata"]["id"] == path.stem,
        f'{graph["metadata"]["id"]} vs {path.stem}',
    )

    report.check("initial node exists", initial in nodes, str(initial))

    bad_endpoints = [
        f'{e["from"]} -> {e["to"]}'
        for e in edges
        if e["from"] not in nodes or e["to"] not in nodes
    ]
    report.check(
        "every edge endpoint is a declared node", not bad_endpoints, ", ".join(bad_endpoints)
    )

    edge_guards = {guard_name(e["when"]) for e in edges if "when" in e}
    skip_guards = {guard_name(n["skipWhen"]) for n in nodes.values() if "skipWhen" in n}
    used_guards = edge_guards | skip_guards

    undeclared = sorted(used_guards - declared_guards)
    report.check("every guard used is declared", not undeclared, ", ".join(undeclared))

    unused = sorted(declared_guards - used_guards)
    report.check("no guard is declared but unused", not unused, ", ".join(unused))

    adjacency: dict[str, list[str]] = {}
    for edge in edges:
        adjacency.setdefault(edge["from"], []).append(edge["to"])

    unreachable = sorted(set(nodes) - reachable_from(initial, adjacency))
    report.check(
        "every node is reachable from initial", not unreachable, ", ".join(unreachable)
    )

    terminals = {name for name, node in nodes.items() if node["type"] == "terminal"}
    report.check("at least one terminal node", bool(terminals))

    dead_ends = sorted(n for n in nodes if n not in terminals and n not in adjacency)
    report.check("no non-terminal node is a dead end", not dead_ends, ", ".join(dead_ends))

    leaving_terminal = sorted(n for n in terminals if n in adjacency)
    report.check(
        "terminal nodes have no outgoing edges", not leaving_terminal, ", ".join(leaving_terminal)
    )

    # Reverse walk: a node that cannot reach any terminal can spin forever.
    reverse: dict[str, list[str]] = {}
    for edge in edges:
        reverse.setdefault(edge["to"], []).append(edge["from"])
    can_end: set[str] = set(terminals)
    queue = deque(terminals)
    while queue:
        for previous in reverse.get(queue.popleft(), []):
            if previous not in can_end:
                can_end.add(previous)
                queue.append(previous)
    stuck = sorted(set(nodes) - can_end)
    report.check("every node can reach a terminal", not stuck, ", ".join(stuck))

    fallback_counts: dict[str, int] = {}
    for edge in edges:
        if "when" not in edge:
            fallback_counts[edge["from"]] = fallback_counts.get(edge["from"], 0) + 1
    multiple = sorted(n for n, count in fallback_counts.items() if count > 1)
    report.check(
        "at most one unconditional edge per node", not multiple, ", ".join(multiple)
    )

    # A node with only conditional edges strands the run when no guard matches.
    # The safe shape is a guard paired with its negation.
    def covers_both_polarities(node: str) -> bool:
        conditions = [e["when"] for e in edges if e["from"] == node and "when" in e]
        positive = {c for c in conditions if not c.startswith("!")}
        negative = {guard_name(c) for c in conditions if c.startswith("!")}
        return bool(positive & negative)

    stranding = sorted(
        n
        for n in adjacency
        if n not in fallback_counts and n not in terminals and not covers_both_polarities(n)
    )
    report.check(
        "conditional-only nodes pair a guard with its negation",
        not stranding,
        ", ".join(stranding),
    )

    # Two nodes writing the same check silently overwrite each other's evidence.
    produced: dict[str, list[str]] = {}
    for name, node in nodes.items():
        if "check" in node:
            produced.setdefault(node["check"], []).append(name)
    collisions = sorted(
        f"{key} ({', '.join(writers)})" for key, writers in produced.items() if len(writers) > 1
    )
    report.check("no two nodes write the same check", not collisions, "; ".join(collisions))

    # A guard that reads a check nothing produces is always false, silently.
    guard_specs = {g["name"]: g for g in spec.get("guards", [])}
    orphaned = sorted(
        f'{g["name"]} reads {g.get("reads", g["name"])}'
        for g in guard_specs.values()
        if g.get("source") == "check" and g.get("reads", g["name"]) not in produced
    )
    report.check(
        "every check-sourced guard reads a check some node produces",
        not orphaned,
        "; ".join(orphaned),
    )

    return report.failures


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    schema_path = root / "schemas/workflow-graph.schema.json"
    graphs_dir = root / ".ai-agents/graphs"

    if not schema_path.is_file():
        print(f"check-graphs: missing {schema_path}", file=sys.stderr)
        return 1
    if not graphs_dir.is_dir():
        print("check-graphs: no .ai-agents/graphs directory, nothing to check")
        return 0

    schema = json.loads(schema_path.read_text(encoding="utf-8"))
    try:
        Draft202012Validator.check_schema(schema)
    except Exception as exc:
        print(f"check-graphs: workflow-graph.schema.json is not valid: {exc}", file=sys.stderr)
        return 1

    graph_paths = sorted(
        p for p in graphs_dir.iterdir() if p.suffix in (".yaml", ".yml")
    )
    if not graph_paths:
        print("check-graphs: no graph files, nothing to check")
        return 0

    total_failures = 0
    for path in graph_paths:
        total_failures += len(validate_graph(path, schema))

    print()
    if total_failures:
        print(f"check-graphs: FAILED ({total_failures} problems)", file=sys.stderr)
        return 1
    noun = "graph" if len(graph_paths) == 1 else "graphs"
    print(f"check-graphs: OK ({len(graph_paths)} {noun})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
