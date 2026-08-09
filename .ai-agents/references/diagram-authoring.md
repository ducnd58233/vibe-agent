# Diagram authoring

<context>

Reusable guidance for diagrams in generated markdown docs, specs, plans, research digests, architecture notes, and handoff reports.
</context>

## What

<context>

Use text-based diagrams when they make a workflow, relationship, architecture boundary, state transition, or timeline easier to understand than prose alone.

Mermaid is the default diagram format for markdown deliverables because it keeps diagrams editable, versionable, and close to the docs that explain them. Official docs: https://mermaid.js.org/intro/ and syntax pages under https://mermaid.js.org/syntax/.
</context>

## How

<procedure>

1. Decide whether a diagram helps.
   - Add a diagram for flows, state maps, sequence interactions, architecture boundaries, ER-style relationships, timelines, or release plans.
   - Prefer prose or a table when the idea is linear, short, or mostly numeric.
2. Use Mermaid by default for markdown docs.
   - Use fenced `mermaid` code blocks.
   - Keep labels short and concrete.
   - Avoid styling unless it serves readability.
3. MUST read current official Mermaid docs before writing or changing Mermaid.
   - Start at https://mermaid.js.org/intro/.
   - Then read the syntax page for the selected diagram type.
   - Treat remembered syntax as stale until checked against docs.
4. Pick the diagram type by intent.
   - Flowchart: process, decision tree, control flow.
   - Sequence diagram: actors or services exchanging messages over time.
   - State diagram: lifecycle states and transitions.
   - Entity relationship diagram: data entities and relationships.
   - Timeline or Gantt: date/order planning.
   - Architecture or C4 diagram: system boundaries, if the target renderer supports it.
5. Verify render after writing.
   - Use an available renderer, markdown preview, Mermaid Live Editor, Mermaid CLI, or documented syntax check.
   - If no renderer or syntax checker is available, write `UNVERIFIED: Mermaid render not checked - <reason>` near the diagram or in the report verification section.
6. Check human-readable visibility after render.
   - Labels are readable without zooming.
   - Direction matches the story: usually top-to-bottom for workflows, left-to-right for sequences or pipelines.
   - Nodes are not cramped.
   - Edges do not cross so much that the flow is hard to follow.
   - Color and styling, if any, has enough contrast.
7. Keep diagrams maintainable.
   - Prefer a small diagram plus a short caption over one large diagram.
   - Split diagrams when more than one concern is shown.
   - Do not duplicate the same information in multiple diagrams unless each view serves a different reader need.
</procedure>

## Required verification note

<verification>

When a command or agent writes a markdown deliverable containing Mermaid, include a short verification note:

```markdown
Diagram verification: Mermaid docs checked (<URLs>). Render checked with <tool/preview>. Readability checked: <brief note>.
```

If render could not be checked:

```markdown
Diagram verification: Mermaid docs checked (<URLs>). UNVERIFIED: render not checked - <reason>.
```
</verification>

## When to use

<routing>

Use this reference from commands, skills, and agents that create or update docs, specs, plans, reports, ADRs, or research digests with diagrams.
</routing>

## Permissions and authority

<rules>

- Reading official Mermaid docs may need web access.
- Render checks may use an installed preview, local CLI, or browser tool when available.
- Do not install Mermaid tooling only for verification unless the user approves dependency changes.
</rules>
