---
name: concurrency-realtime-systems
description: >-
  Designs, debugs, and implements concurrency-heavy, realtime, streaming, and high-traffic systems with explicit backpressure, cancellation, capacity, protocol, and observability choices. Use when working on WebSockets, SSE, WebRTC, video/live streaming, pub/sub fan-out, queues, async runtimes, slow consumers, overload, tail latency, or high connection-count products.
disable-model-invocation: true
---

# Concurrency Realtime Systems

## How

<procedure>

1. **Load stack context**
   - Inspect manifests and existing patterns.
   - Open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md) and compose all matching profiles, especially [`realtime-concurrency-high-traffic.md`](../../stack-profiles/realtime-concurrency-high-traffic.md) and runtime-specific profiles such as [`backend-rust-axum.md`](../../stack-profiles/backend-rust-axum.md).
2. **Define the realtime contract**
   - Protocol: WebSocket, SSE, WebRTC, HLS/DASH/MSE, queue, broker, or hybrid.
   - Directionality, ordering, replay, idempotency, delivery semantics, and schema versioning.
   - Auth, authorization, tenant isolation, and abuse/rate-limit model.
3. **Map concurrency ownership**
   - Name every task/worker, who owns it, what cancels it, and what resources it holds.
   - Prefer structured concurrency and explicit shutdown/drain paths.
   - Avoid detached tasks unless ownership and lifecycle are documented.
4. **Design backpressure and overload behavior**
   - Make all queues, channels, request bodies, buffers, and per-connection outbound mailboxes bounded.
   - Define slow-consumer policy: drop, coalesce, sample, disconnect, degrade, or persist for replay.
   - Add capacity assumptions: connection count, message rate, payload size, p95/p99 latency, broker lag, memory budget.
5. **Separate planes**
   - Keep control plane, data plane, analytics, and persistence independent where possible.
   - For media/live streaming, separate signaling, ingest, transcoding/packaging, CDN playback, and player telemetry.
6. **Instrument before tuning**
   - Track p50/p95/p99 latency, queue depth, dropped messages, connection count, per-connection buffer size, runtime pauses, CPU/memory, reconnects, and error causes.
   - Add spans/correlation IDs across handshake, auth, fan-out, persistence, and outbound send.
7. **Implement surgically**
   - Start with the smallest vertical slice: connect, authenticate, subscribe, send/receive, close, cleanup.
   - Add tests for cancellation, reconnect, duplicate delivery, slow consumers, and bounded-queue behavior.
   - Only then add load tests or capacity scripts matching repo tooling.
</procedure>

## Routing & discovery

<routing>

- Pair with [`backend-engineering`](../backend-engineering/SKILL.md) for service/repository boundaries.
- Pair with [`performance-optimization`](../performance-optimization/SKILL.md) for profiling and optimization.
- Pair with [`security-and-hardening`](../security-and-hardening/SKILL.md) for auth, abuse controls, and untrusted message handling.
- Pair with [`source-driven-development`](../source-driven-development/SKILL.md) when framework APIs are version-sensitive.

Use for realtime or high-traffic architecture, implementation, debugging, and review. Do not use for ordinary CRUD endpoints unless concurrency, streaming, or overload behavior is part of the task.
</routing>

## Permissions & authority

<required>

- Tools: Read, Grep, Glob, Edit, and Shell for repo-documented tests/build/load smoke checks.
- Paths: source, tests, manifests, docs, and local benchmark scripts only; no secrets.
- Suggested rules: allow documented test/build commands; ask before long-running load tests or commands that hit external services.
</required>

## Verification

<verification>

- [ ] Protocol, schema, ordering, replay, idempotency, and delivery semantics are explicit.
- [ ] Every task/worker/connection has an owner, cancellation path, cleanup path, and bounded resources.
- [ ] Slow-consumer and overload behavior is deliberate and observable.
- [ ] Metrics/spans cover latency, drops, queue depth, connections, runtime health, and error causes.
- [ ] Tests or smoke checks cover connect, close, cancellation, reconnect, backpressure, and at least one failure mode.
</verification>

## References

<references>

- https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API
- https://developer.mozilla.org/docs/Web/API/WebRTC_API
- https://developer.mozilla.org/en-US/docs/Web/API/Media_Source_Extensions_API
- https://tokio.rs/tokio/tutorial/spawning
- https://tokio.rs/tokio/tutorial/shared-state
- https://tokio.rs/tokio/tutorial/channels
- https://docs.rs/tokio/latest/tokio/task/fn.spawn_blocking.html
</references>
