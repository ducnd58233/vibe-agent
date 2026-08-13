# Token and memory efficiency

<context>

Use this when deciding how an agent should read a codebase, a web page, or a
document, and when weighing whether a memory or caching layer earns its cost. The
question is always the same one: cut what the agent spends without cutting what
it gets right.

Sibling references: [`context-management-patterns.md`](context-management-patterns.md)
for budgets and progressive disclosure, [`agent-harness-engineering.md`](agent-harness-engineering.md)
for where a control plane's authority ends.
</context>

## Four techniques, and who owns each

<rules>

- **Offloading** - store the payload, put a reference in context. An outer loop
  can own this: it is about what sits on disk between turns.
- **Retrieval** - fetch the relevant slice at runtime. Also the outer loop's.
- **Reduction** - compact or summarize the running conversation. **The host owns
  this.** A control plane that rewrote the model's context would be fighting the
  loop it does not own.
- **Isolation** - give a sub-agent its own context. Also the host's.

Draw the line before building. Two of these are a runtime's business and two are
not. ([LangChain](https://www.langchain.com/blog/context-engineering-for-agents))
</rules>

## Measured reductions

<context>

Order-of-magnitude figures worth knowing before designing anything:

| Move | Reduction | Source |
|---|---|---|
| Raw HTML → extracted text | 80-90% | [worked example: 16,180 → 3,150 tokens](https://dev.to/stevengonsalvez/markdownnew-jina-reader-stop-feeding-your-llm-raw-html-46m8) |
| PDF/DOCX → markdown | 50-70% | [tool comparison](https://www.danilchenko.dev/posts/markitdown-vs-docling-vs-marker/) |
| Reading files → structural index | ~10x, and 2.3 vs 4.8 tool calls | [Codebase-Memory](https://arxiv.org/html/2603.27277v1) |
| Symbol lookup off an index vs file-by-file | ~200 vs ~45,000 tokens | same |

This toolkit's own numbers, on its own repository: `vibe-agent map` renders 545
files for ~5,000 tokens against ~743,000 to read them; `vibe-agent fetch` returns
a documentation page 94% smaller than the raw response. See
[`runtime/README.md`](../../runtime/README.md).
</context>

## Rules that survived contact

<rules>

- **Cache on content hash, not mtime.** A checkout moves mtime without changing
  content; a restored file changes content without moving mtime. Pair it with a
  cache version, because when extraction logic changes every row is stale while
  every hash still matches.
- **Clip to a budget and say what was left.** Silent truncation is what makes an
  agent assert things about content it never saw. Return the clip plus a handle
  to the whole. ([Arize](https://arize.com/blog/context-management-in-agent-harnesses/),
  [truncation failure mode](https://dev.to/gabrielanhaia/tool-result-truncation-the-silent-bug-that-makes-agents-lie-3epe))
- **Never summarize into a cache.** An index of declarations at named lines is
  evidence with a `file_content` provenance; a paraphrase is model output wearing
  an index's authority.
- **Refuse formats you cannot read, by name.** Emitting a PDF's bytes as text
  spends a great many tokens on mojibake the agent cannot detect.
- **Drop ambiguous ranking signals rather than down-weighting them.** A symbol
  several files declare attributes a mention to none of them. Every script
  defines `main`.
</rules>

## Anti-patterns

<antipatterns>

- **Preloading the repository into context.** Give the agent search and targeted
  reads instead. ([Arize](https://arize.com/blog/context-management-in-agent-harnesses/))
- **Buying a graph before measuring keyword search.** A temporal knowledge graph
  is strong on temporal queries and expensive: Zep's footprint is measured above
  600,000 tokens per conversation. FTS5 plus rank fusion costs kilobytes and is
  the right default until it is *measured* insufficient.
  ([Zep](https://arxiv.org/abs/2501.13956),
  [2026 comparison](https://blog.devgenius.io/ai-agent-memory-systems-in-2026-mem0-zep-hindsight-memvid-and-everything-in-between-compared-96e35b818da8))
- **Writing fixtures instead of capturing them.** Twice in this toolkit a green
  suite hid a total failure, because the payload in the test was written from
  documentation and never matched the real host. Capture the real thing first.
- **Hand-rolling a parser for a format that has a spec and a library.** HTML is
  not a regular language: attribute values hold `>`, script bodies hold `<`,
  comments and CDATA nest, and browsers run an error-correction algorithm over
  all of it. A hand-written tokenizer here passed every test and returned an
  empty body for the first real page, then glued table cells into values that
  read like single facts. Reach for the spec-compliant parser
  ([`x/net/html`](https://pkg.go.dev/golang.org/x/net/html)) first, and justify a
  hand-rolled one only against a measured cost, not a remembered rule.
- **Inventing a constraint and then citing it.** The rule quoted here for
  avoiding dependencies — "the whole dependency set is a SQLite driver and a YAML
  reader" — was written into this repository's own docs during the work it was
  used to justify. The only real constraint was `CGO_ENABLED=0`, which every
  pure-Go library satisfies. Check whether a constraint predates the decision it
  is being used to defend.
- **Trusting a guard nobody registered.** An unwired guard is indistinguishable
  from a passing one. An assurance system needs a check that its own checks are
  connected.
</antipatterns>

## Before adding a caching or memory layer

<verification>

- Can you state the saving as a measured number on a real input, not an estimate?
- Is the cached artifact evidence, or is it a paraphrase?
- Does it invalidate on content, and does it invalidate when your own extraction
  logic changes?
- Does a truncated result say it was truncated?
- Does the diagnostic that reports on it avoid creating it?
- Is anything ever deleted, or does the store only grow?
</verification>

## References

<references>

- [Context engineering for agents](https://www.langchain.com/blog/context-engineering-for-agents) - the four techniques.
- [Context management in agent harnesses](https://arize.com/blog/context-management-in-agent-harnesses/) - read caps, pagination, summary-plus-handle.
- [Aider repo map](https://aider.chat/2023/10/22/repomap.html) - tree-sitter tags, PageRank, cache versioning.
- [Codebase-Memory](https://arxiv.org/html/2603.27277v1) - AST knowledge graph in SQLite, measured token and tool-call savings.
- [Zep](https://arxiv.org/abs/2501.13956) - temporal knowledge graph memory, and what it costs.
- [Trafilatura vs ReaderLM](https://www.contextractor.com/trafilatura-vs-jina-readerlm/) - heuristic against model-based extraction.
</references>
