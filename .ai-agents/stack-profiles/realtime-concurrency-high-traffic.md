# Stack profile: Realtime concurrency and high-traffic systems

## Scope

<routing>

Applies to consumer repositories implementing concurrent services, WebSockets, SSE, WebRTC signaling, media/video/live-streaming control planes, pub/sub fan-out, queues, or high-traffic APIs where latency, backpressure, capacity, and failure isolation are first-class concerns.

## When to load

- Designing or debugging WebSocket, SSE, WebRTC, live-streaming, or realtime collaboration flows
- Implementing fan-out, presence, chat, notifications, media signaling, or stream control planes
- Handling high connection counts, burst traffic, slow consumers, queue growth, or overload
- Choosing concurrency primitives, worker pools, partitioning, rate limits, and backpressure policy
</routing>

## Detection

<context>

- Keywords or dependencies: `websocket`, `ws`, `sse`, `webrtc`, `rtmp`, `hls`, `dash`, `mse`, `ffmpeg`, `gstreamer`, `redis`, `nats`, `kafka`, `rabbitmq`, `pubsub`
- Source paths such as `realtime/`, `streaming/`, `sockets/`, `workers/`, `queues/`, `events/`
- Runtime symptoms: dropped frames, tail latency spikes, unbounded memory, blocked event loop, queue lag, reconnect storms

## Framework and tooling

- Use the repo's runtime profile first: Tokio/Axum, Go, Node, Python async, JVM, or mobile/web client
- Protocols by fit: WebSockets for bidirectional messaging, SSE for server-to-client streams, WebRTC for low-latency peer/media, HLS/DASH/MSE for scalable playback, message brokers for durable fan-out
- Observability: RED/USE metrics, distributed tracing, queue depth, event-loop lag, per-connection buffer size, dropped-message counters
- Load testing: k6, wrk, vegeta, Locust, Gatling, custom socket simulators, or repo-pinned tools

## Repo layout conventions

- Keep protocol adapters separate from domain event handlers and transport-independent services
- Define explicit message schemas, versioning, idempotency keys, and retry semantics
- Isolate connection/session registries behind a small interface; do not scatter global socket maps
- Separate hot-path ingest, validation, authorization, fan-out, persistence, and analytics
- Add shutdown/drain paths for workers, sockets, subscriptions, and media pipelines
</context>

## Commands

<procedure>

- Use repo-documented lint, test, and build commands first
- Add or run a focused load/smoke command only when the repo already defines one
- Typical examples: `cargo test --all`, `go test ./...`, `npm run test`, `npm run build`, `k6 run <script>`
</procedure>

## Boundaries

<required>

- Do not mix media data-plane processing with request/response business logic unless the repo intentionally embeds both
- Do not accept unbounded per-user queues, broadcast channels, request bodies, or retry loops
- Do not use realtime delivery as a substitute for durable state when clients need replay or exactly-once-like behavior
- Do not optimize throughput before defining SLOs, bottleneck evidence, and failure budget

## Security / performance appendix

- Define overload behavior: rate limit, shed, degrade, batch, sample, disconnect, or queue with bounded depth
- Prefer pull/backpressure-aware streams where available; otherwise enforce bounded buffers and slow-consumer policy
- Track p50/p95/p99 latency, connection count, CPU, memory, GC/runtime pauses, queue lag, broker partitions, and dropped messages
- For video/live streaming, separate signaling, ingest, transcoding/packaging, CDN playback, and analytics concerns
- For WebRTC, account for STUN/TURN, NAT traversal, renegotiation, stats, and fallback behavior
</required>

## References

<references>

- https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API
- https://developer.mozilla.org/docs/Web/API/WebRTC_API
- https://developer.mozilla.org/en-US/docs/Web/API/Media_Source_Extensions_API
- https://tokio.rs/tokio/tutorial/channels
- https://tokio.rs/tokio/tutorial/shared-state
- https://docs.rs/tokio/latest/tokio/task/fn.spawn_blocking.html
</references>
