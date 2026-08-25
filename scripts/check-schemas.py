#!/usr/bin/env python3
"""Validate the JSON Schema contracts under schemas/.

Two layers:

1. Each file is a valid Draft 2020-12 schema.
2. Each schema actually enforces the design rules it exists for. A schema that
   parses but accepts a forbidden instance is worse than no schema, because it
   reads like a guarantee.

The reject cases below are the load-bearing part. They pin down decisions that
would otherwise survive only as prose:

  - a graph edge condition is a guard name, never an expression
  - a run-state check records real provenance, never model assertion
  - a memory record carries evidence, and procedural memory is not stored
  - a check plan entry says how the check is produced, never nothing
  - acceptsSkipped is only meaningful on a check-sourced guard

Needs jsonschema (scripts/requirements.txt).

Usage: python3 scripts/check-schemas.py [repo-root]
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

try:
    import yaml
    from jsonschema import Draft202012Validator
except ImportError as exc:
    sys.exit(
        f"check-schemas: missing dependency ({exc.name}).\n"
        "Install with: python3 -m pip install -r scripts/requirements.txt"
    )

NOW = "2026-07-29T10:00:00Z"


def base_graph() -> dict:
    return {
        "apiVersion": "vibe-agent/v1",
        "kind": "WorkflowGraph",
        "metadata": {"id": "sample", "description": "d"},
        "spec": {
            "initial": "build",
            "maxTransitions": 50,
            "guards": [
                {"name": "tests_pass", "description": "d", "source": "check", "reads": "unit"}
            ],
            "nodes": {
                "build": {"type": "agent", "command": "build"},
                "done": {"type": "terminal", "status": "done"},
            },
            "edges": [{"from": "build", "to": "done", "when": "tests_pass"}],
        },
    }


def base_run() -> dict:
    return {
        "schemaVersion": 1,
        "runId": "run_2026-07-29T10-00-00Z_sample",
        "graphId": "goal-delivery",
        "slug": "sample",
        "goal": "g",
        "currentNode": "build",
        "status": "running",
        "iteration": 0,
        "maxTransitions": 50,
        "createdAt": NOW,
        "updatedAt": NOW,
    }


def base_memory() -> dict:
    return {
        "schemaVersion": 1,
        "id": "mem_01J0000000000000000000000A",
        "workspaceId": "ws1",
        "kind": "episodic",
        "content": "Integration tests require Redis on localhost:6379.",
        "confidence": 0.95,
        "status": "proposed",
        "sourceType": "command_result",
        "evidence": ["make integration-test failed with connection refused"],
        "createdAt": NOW,
        "updatedAt": NOW,
    }


def graph_with(node: dict) -> dict:
    graph = base_graph()
    graph["spec"]["nodes"]["x"] = node
    graph["spec"]["edges"].append({"from": "x", "to": "done"})
    return graph


def with_guards(guards: list[dict]) -> dict:
    graph = base_graph()
    graph["spec"]["guards"] = guards
    return graph


def with_edges(edges: list[dict]) -> dict:
    graph = base_graph()
    graph["spec"]["edges"] = edges
    return graph


def merged(base: dict, **overrides) -> dict:
    instance = dict(base)
    instance.update(overrides)
    return instance


def base_plan() -> dict:
    return {
        "apiVersion": "vibe-agent/v1",
        "kind": "CheckPlan",
        "spec": {"checks": {"unit": {"command": "go", "args": ["test", "./..."]}}},
    }


def base_tasks() -> dict:
    return {
        "schemaVersion": 1,
        "slug": "sample",
        "date": "2026-08-21",
        "version": 1,
        "tasks": [{"id": "T1", "title": "one", "status": "queued"}],
    }


def plan_with(entry: dict) -> dict:
    """A plan whose single check is the given entry."""
    instance = base_plan()
    instance["spec"] = {"checks": {"unit": entry}}
    return instance


def cases(schemas: dict[str, dict]) -> list[tuple[dict, str, dict, bool]]:
    graph, run, memory = schemas["workflow-graph"], schemas["run-state"], schemas["memory-record"]
    plan = schemas["check-plan"]
    tasks = schemas["tasks"]
    return [
        (graph, "graph accepts a minimal valid document", base_graph(), True),
        (graph, "graph rejects an expression where a guard name belongs",
         with_edges([{"from": "build", "to": "done", "when": "state.e2eRequired == false"}]), False),
        (graph, "graph accepts a negated guard",
         with_edges([{"from": "build", "to": "done", "when": "!tests_pass"}]), True),
        (graph, "graph rejects an unknown node type", graph_with({"type": "wait"}), False),
        (graph, "graph rejects a verifier without a check",
         graph_with({"type": "verifier", "verifier": "command"}), False),
        (graph, "graph accepts a verifier with a check",
         graph_with({"type": "verifier", "verifier": "command", "check": "unit"}), True),
        (graph, "graph rejects a human gate without a prompt",
         graph_with({"type": "human_gate", "check": "approve"}), False),
        (graph, "graph rejects an unknown terminal status",
         graph_with({"type": "terminal", "status": "finished"}), False),
        (graph, "graph rejects an unknown top-level field", merged(base_graph(), extra=1), False),
        (graph, "graph rejects a wrong apiVersion",
         merged(base_graph(), apiVersion="vibe-agent/v2"), False),
        (graph, "graph rejects acceptsSkipped on a flag-sourced guard",
         with_guards([{"name": "in_scope", "description": "d", "source": "flag",
                       "acceptsSkipped": True}]), False),
        (graph, "graph accepts acceptsSkipped on a check-sourced guard",
         with_guards([{"name": "e2e_ok", "description": "d", "source": "check",
                       "reads": "e2e", "acceptsSkipped": True}]), True),
        (graph, "graph accepts a hyphenated id",
         merged(base_graph(), metadata={"id": "goal-delivery", "description": "d"}), True),

        (run, "run accepts a minimal valid document", base_run(), True),
        (run, "run rejects model as a check source",
         merged(base_run(), checks={"unit": {"passed": True, "source": "model", "at": NOW}}), False),
        (run, "run accepts exit_code as a check source",
         merged(base_run(), checks={"unit": {"passed": True, "source": "exit_code", "at": NOW}}), True),
        (run, "run rejects an unknown status", merged(base_run(), status="finished"), False),
        (run, "run rejects a malformed runId", merged(base_run(), runId="run1"), False),
        (run, "run rejects a non-boolean flag",
         merged(base_run(), flags={"e2e_required": "yes"}), False),
        (run, "run rejects a blocker without an attempt count",
         merged(base_run(), blockers=[{"node": "test", "reason": "r", "at": NOW}]), False),
        (run, "run accepts an empty currentNode before the first transition",
         merged(base_run(), currentNode=""), True),
        (run, "run rejects a malformed currentNode",
         merged(base_run(), currentNode="Build Step"), False),

        (memory, "memory accepts a minimal valid record", base_memory(), True),
        (memory, "memory rejects the procedural kind", merged(base_memory(), kind="procedural"), False),
        (memory, "memory rejects empty evidence", merged(base_memory(), evidence=[]), False),
        (memory, "memory rejects model_inference as a source",
         merged(base_memory(), sourceType="model_inference"), False),
        (memory, "memory rejects confidence above 1", merged(base_memory(), confidence=1.5), False),
        (memory, "memory accepts the confirmed status", merged(base_memory(), status="confirmed"), True),

        (plan, "plan accepts a minimal valid document", base_plan(), True),
        # The load-bearing one. An entry that declares nothing would resolve to a
        # verifier with no command, and a caller ignoring the error would read
        # that as a check with no problems.
        (plan, "plan rejects an entry that says nothing about how the check runs",
         plan_with({"timeoutSeconds": 60}), False),
        (plan, "plan rejects an entry that is only a description",
         plan_with({"description": "we run the suite"}), False),
        (plan, "plan accepts a check a person decides, stated explicitly",
         plan_with({"verifier": "human", "description": "a judgement call"}), True),
        (plan, "plan rejects an unknown verifier kind",
         plan_with({"verifier": "screenshot", "command": "true"}), False),
        (plan, "plan rejects a misspelled entry key",
         plan_with({"commnad": "go"}), False),
        (plan, "plan rejects an empty checks map", merged(base_plan(), spec={"checks": {}}), False),
        (plan, "plan rejects a check name run state would refuse to store",
         merged(base_plan(), spec={"checks": {"Unit Tests": {"command": "go"}}}), False),
        (plan, "plan rejects a graph masquerading as a plan",
         merged(base_plan(), kind="WorkflowGraph"), False),
        (plan, "plan accepts a screen check with content assertions",
         plan_with({"verifier": "screen", "screen": {
             "platform": "android",
             "launch": "adb",
             "launchArgs": ["shell", "am", "start", "-n", "com.example/.Main"],
             "settleSeconds": 8,
             "expectText": ["Total: 42.00"],
             "forbidText": ["Unhandled JS Exception"],
         }}), True),
        # A screen block with no platform cannot pick a toolchain, and defaulting
        # one would silently drive the wrong device.
        (plan, "plan rejects a screen block with no platform",
         plan_with({"verifier": "screen", "screen": {"expectText": ["hi"]}}), False),
        (plan, "plan rejects an unknown screen platform",
         plan_with({"verifier": "screen", "screen": {"platform": "web"}}), False),
        (plan, "plan rejects a misspelled screen key",
         plan_with({"verifier": "screen", "screen": {"platform": "android", "expectTest": ["hi"]}}), False),
        # docs/auto-ship-reviews: a check may stay verifier: human by default
        # and gain an auto-only alternative, active only on a run whose auto
        # flag is set. These four mirror runtime/internal/checkplan's own
        # tests, so the two validators cannot silently disagree.
        (plan, "plan accepts a well-formed auto entry",
         plan_with({"verifier": "human", "auto": {"verifier": "shipdecision"}}), True),
        (plan, "plan rejects an auto entry that is also human",
         plan_with({"verifier": "human", "auto": {"verifier": "human"}}), False),
        (plan, "plan rejects a nested auto entry",
         plan_with({"verifier": "human", "auto": {
             "verifier": "shipdecision", "auto": {"verifier": "shipdecision"},
         }}), False),
        (plan, "plan rejects an auto entry with nothing runnable",
         plan_with({"verifier": "human", "auto": {"description": "forgot a verifier"}}), False),
        (plan, "plan accepts a reviewbots auto entry with a command",
         plan_with({"verifier": "human", "auto": {
             "verifier": "reviewbots", "command": "gh", "args": ["pr", "checks"],
         }}), True),

        (tasks, "tasks accepts a minimal valid document", base_tasks(), True),
        (tasks, "tasks rejects a document without date",
         {k: v for k, v in base_tasks().items() if k != "date"}, False),
        (tasks, "tasks rejects a document without version",
         {k: v for k, v in base_tasks().items() if k != "version"}, False),
        (tasks, "tasks rejects version zero",
         merged(base_tasks(), version=0), False),
        (tasks, "tasks rejects a bad date",
         merged(base_tasks(), date="08-21-2026"), False),
    ]


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    schemas_dir = root / "schemas"
    if not schemas_dir.is_dir():
        print(f"check-schemas: missing {schemas_dir}", file=sys.stderr)
        return 1

    names = ["workflow-graph", "run-state", "memory-record", "check-plan", "tasks", "auto", "sandbox"]
    schemas: dict[str, dict] = {}
    failures = 0

    for name in names:
        path = schemas_dir / f"{name}.schema.json"
        if not path.is_file():
            print(f"  FAIL  {name}.schema.json is missing", file=sys.stderr)
            failures += 1
            continue
        schema = json.loads(path.read_text(encoding="utf-8"))
        schemas[name] = schema
        try:
            Draft202012Validator.check_schema(schema)
            print(f"  ok    {name}.schema.json is valid Draft 2020-12")
        except Exception as exc:
            print(f"  FAIL  {name}.schema.json is not valid Draft 2020-12: {exc}", file=sys.stderr)
            failures += 1

    if len(schemas) != len(names):
        print("\ncheck-schemas: FAILED (missing or unreadable schema)", file=sys.stderr)
        return 1

    print()
    all_cases = cases(schemas)
    for schema, label, instance, must_accept in all_cases:
        errors = list(Draft202012Validator(schema).iter_errors(instance))
        accepted = not errors
        if accepted == must_accept:
            print(f"  ok    {label}")
        else:
            failures += 1
            verb = "accept" if must_accept else "reject"
            print(f"  FAIL  {label}: schema should {verb} this instance", file=sys.stderr)
            for error in errors[:2]:
                print(f"          {error.message}", file=sys.stderr)

    fixtures = check_go_fixtures(root, schemas["run-state"])
    failures += fixtures[0]
    failures += check_workspace_plan(root, schemas["check-plan"])
    failures += check_auto_template(root, schemas["auto"])

    print()
    if failures:
        print(f"check-schemas: FAILED ({failures} problems)", file=sys.stderr)
        return 1
    noun = "fixture" if fixtures[1] == 1 else "fixtures"
    print(
        f"check-schemas: OK ({len(names)} schemas, {len(all_cases)} cases, "
        f"{fixtures[1]} Go {noun})"
    )
    return 0


def check_workspace_plan(root: Path, plan_schema: dict) -> int:
    """Validate this repo's own vibe-checks.yaml against the schema.

    The contract test across the language boundary, the same shape as
    check_go_fixtures. The Go loader in runtime/internal/checkplan and this schema
    are two independent statements of what a plan may contain; without a real
    document checked against both, they drift and the first person to notice is
    someone whose run stalls.
    """
    path = root / "vibe-checks.yaml"
    if not path.is_file():
        return 0

    print()
    instance = yaml.safe_load(path.read_text(encoding="utf-8"))
    errors = list(Draft202012Validator(plan_schema).iter_errors(instance))
    label = "vibe-checks.yaml validates against check-plan.schema.json"
    if not errors:
        print(f"  ok    {label}")
        return 0
    print(f"  FAIL  {label}", file=sys.stderr)
    for error in errors[:3]:
        print(f"          {list(error.path)}: {error.message}", file=sys.stderr)
    return 1


def check_auto_template(root: Path, auto_schema: dict) -> int:
    """Validate the template `auto init` writes against the auto schema.

    The template is a Go string and the schema is JSON, so nothing otherwise
    stops them describing different files. A workspace would then get an opt-in
    the runtime accepts and this checker rejects, or the reverse, and the
    disagreement would only surface when somebody tried to merge.
    """
    source = root / "runtime" / "internal" / "autoconfig" / "autoconfig.go"
    if not source.is_file():
        print(f"  FAIL  {source} is missing", file=sys.stderr)
        return 1

    text = source.read_text(encoding="utf-8")
    marker = "const Template = `"
    start = text.find(marker)
    if start < 0:
        print("  FAIL  autoconfig.go has no Template const to check", file=sys.stderr)
        return 1
    start += len(marker)
    end = text.find("`", start)
    template = text[start:end]

    label = "auto init template validates against auto.schema.json"
    errors = list(Draft202012Validator(auto_schema).iter_errors(yaml.safe_load(template)))
    if not errors:
        print(f"  ok    {label}")
        return 0
    print(f"  FAIL  {label}", file=sys.stderr)
    for error in errors[:3]:
        print(f"          {list(error.path)}: {error.message}", file=sys.stderr)
    return 1


def check_go_fixtures(root: Path, run_schema: dict) -> tuple[int, int]:
    """Validate manifests the Go writer produced against the run-state schema.

    This is the contract test across the language boundary. The Go golden test
    pins what Save emits; this pins that the schema accepts it. Without both,
    the two drift and nobody notices until a run fails to load.
    """
    fixtures_dir = root / "runtime/internal/run/infra/persistence/testdata"
    paths = sorted(fixtures_dir.glob("*.json")) if fixtures_dir.is_dir() else []
    if not paths:
        # A missing fixture is a failure, not a skip. This used to return
        # silently, so moving the Go package left the check reporting OK while
        # validating nothing: the contract across the language boundary was
        # unguarded and the output said it was fine.
        print()
        print(f"  FAIL  no Go fixtures under {fixtures_dir.relative_to(root)}; "
              "the run-state contract is unchecked")
        return 1, 1

    print()
    failures = 0
    for path in paths:
        instance = json.loads(path.read_text(encoding="utf-8"))
        errors = list(Draft202012Validator(run_schema).iter_errors(instance))
        label = f"Go fixture {path.name} validates against run-state.schema.json"
        if errors:
            failures += 1
            print(f"  FAIL  {label}", file=sys.stderr)
            for error in errors[:3]:
                print(f"          {list(error.path)}: {error.message}", file=sys.stderr)
        else:
            print(f"  ok    {label}")
    return failures, len(paths)


if __name__ == "__main__":
    sys.exit(main())
