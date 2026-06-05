# Stack profiles router

Lookup table for **repo-pinned stack** markdown files in this folder. Generic skills and references stay portable; profiles name **frameworks, layout, and tooling** here.

**After you add, rename, or remove a profile `*.md`, update this table in the same change.**

## Composing profiles

Many tasks span several layers:

1. Open this table.
2. Select **every** row whose **When to load** fits the current task (e.g. UI change + HTTP API change + compose two profiles).
3. Read each matching `*.md` file listed in **Profile**.
4. If no row fits, fall back to **manifest + directory scanning** (`package.json`, `pyproject.toml`, `go.mod`, `apps/`, `backend/`, ...) until you author a matching profile ([`TEMPLATE.md`](TEMPLATE.md)).

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md).

| Profile | Layer / concern | When to load | Detection / notes |
|---------|-----------------|--------------|-------------------|
| [`frontend-nextjs-ts.md`](frontend-nextjs-ts.md) | Frontend web (Next.js + TypeScript) | UI/routing/component boundary work | `package.json` has `next`; `tsconfig.json`; `app/` or `pages/` present |
| [`backend-fastapi.md`](backend-fastapi.md) | Backend HTTP APIs (Python/FastAPI) | Endpoint/service/validation persistence work | FastAPI in dependency manifests; `uv.lock`/`alembic.ini` may exist |
| [`backend-golang.md`](backend-golang.md) | Backend services (Go) | Go API/service layering and module changes | `go.mod` present; `cmd/` + `internal/` common |
| [`backend-rust-axum.md`](backend-rust-axum.md) | Backend HTTP APIs (Rust/Axum/Tokio) | Rust API, router, extractor, middleware, async service work | `Cargo.toml` has `axum`; `tokio`, `tower`, `tower-http`, `tracing` common |
| [`realtime-concurrency-high-traffic.md`](realtime-concurrency-high-traffic.md) | Realtime, concurrency, streaming, high-traffic systems | WebSockets, SSE, WebRTC, live/video streaming, fan-out, overload, slow consumers | `ws`, `webrtc`, `sse`, `streaming`, brokers/queues, high connection counts |
| [`frontend-reactjs.md`](frontend-reactjs.md) | Frontend web (React.js SPA/component UI) | React component, hook, routing, state, forms, accessibility, render performance | `package.json` has `react`/`react-dom`; Vite/React Router common |
| [`design-tools-mcp.md`](design-tools-mcp.md) | Design tools, design systems, MCP handoff | Figma/Canva/design-token/design-to-code workflows | Figma, Canva, MCP, design tokens, Storybook, visual regression, `tokens.json` |
| [`mobile-react-native.md`](mobile-react-native.md) | Mobile apps (React Native / Expo) | RN screens, navigation, native modules, mobile performance, Expo/bare app changes | `package.json` has `react-native` or `expo`; `ios/`, `android/`, Metro config |
| [`mobile-flutter.md`](mobile-flutter.md) | Mobile apps (Flutter/Dart) | Flutter widgets, state, navigation, platform channels, mobile performance | `pubspec.yaml` has `flutter`; `lib/main.dart`; `android/`, `ios/` |
| [`mobile-android-native.md`](mobile-android-native.md) | Mobile apps (Native Android) | Kotlin/Java, Compose/XML, ViewModel, coroutines, Gradle, Android performance | `build.gradle*`, `AndroidManifest.xml`, `androidx.*`, `kotlinx-coroutines` |
| [`mobile-ios-native.md`](mobile-ios-native.md) | Mobile apps (Native iOS) | Swift/SwiftUI/UIKit, concurrency, Xcode project, iOS performance/accessibility | `.xcodeproj`, `.xcworkspace`, `Package.swift`, Swift files |
| [`devops-platform-cicd.md`](devops-platform-cicd.md) | DevOps platform, CI/CD, IaC, deploy automation | Pipelines, containers, Kubernetes, Terraform/OpenTofu, release gates | `.github/workflows`, `.gitlab-ci.yml`, `Dockerfile`, `*.tf`, `k8s/`, `helm/` |
| [`system-administration.md`](system-administration.md) | System administration and host operations | systemd, Ansible, shell scripts, service logs, backups, host runbooks | `ansible.cfg`, `playbooks/`, `*.service`, cron/timers, ops scripts |
| [`observability-monitoring.md`](observability-monitoring.md) | Observability, monitoring, alerting, dashboards | OpenTelemetry, Prometheus/Grafana, logs/traces/metrics, SLOs, runbooks | `otel`, `prometheus`, `grafana`, `alerts/`, `dashboards/`, `/metrics` |
| [`sql-databases.md`](sql-databases.md) | SQL databases and query optimization | Schemas, migrations, indexes, transactions, SQL errors, slow queries | `*.sql`, migrations, ORM configs, PostgreSQL/MySQL/SQLite/SQL Server deps |
| [`nosql-databases.md`](nosql-databases.md) | NoSQL databases and datastore optimization | MongoDB/Redis/Cassandra/DynamoDB/Search query/data-model/cache issues | NoSQL deps/configs, `collections/`, `cache/`, `datastore/`, slow logs |
| [`ai-modeling-multimodal.md`](ai-modeling-multimodal.md) | AI/ML model engineering and multimodal modeling | CV, NLP/LLM, speech/audio, recommender, tabular, multimodal, generative AI model build/eval/serve/monitor work | ML/AI deps, `models/`, `training/`, `inference/`, `evals/`, `prompts/`, `rag/`, model/data cards |
| [`mlops.md`](mlops.md) | MLOps lifecycle and model operations | ML pipelines, model registry, training/eval/serving, drift monitoring | `mlflow`, `kubeflow`, `models/`, `training/`, `inference/`, `features/` |
| [`product-lifecycle-delivery.md`](product-lifecycle-delivery.md) | Product lifecycle, launch, metrics, rollout | Specs, flags, success metrics, staged launch, deprecation/EOL | `docs/specs`, release plans, feature flags, analytics/observability configs |
| [`finance-analyzer.md`](finance-analyzer.md) | Finance research and metrics analysis | Public-company, valuation, fundamentals tasks | Mentions 10-K/10-Q/8-K, EDGAR, FRED, market metrics |
| [`finance-advisor.md`](finance-advisor.md) | Advisory-safe finance responses | User requests action-oriented investment guidance | Same as analyzer plus suitability/disclaimer constraints |
| [`datascience.md`](datascience.md) | Data science / ML workflows | Dataset analysis, model training/evaluation, notebooks | Data/ML libs in manifests, notebooks present, `data/`/`models/` hints |
