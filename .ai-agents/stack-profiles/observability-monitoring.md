# Stack profile: Observability and monitoring

## Scope

Applies to consumer repositories that instrument applications or infrastructure for metrics, logs, traces, dashboards, alerts, SLOs, incident response, telemetry pipelines, and monitoring tools.

## When to load

- Adding OpenTelemetry, Prometheus metrics, structured logs, traces, dashboards, or alert rules
- Building internal tools for monitoring, observability, incident triage, or service health
- Defining SLOs, SLIs, alert thresholds, runbooks, or telemetry retention/cost controls
- Debugging gaps in logs, traces, metrics, correlation IDs, or dashboard fidelity

## Detection

- `otel`, `opentelemetry`, `prometheus`, `grafana`, `jaeger`, `tempo`, `loki`, `alertmanager`, `datadog`, `newrelic`, `sentry`
- `dashboards/`, `alerts/`, `prometheus.yml`, `grafana/`, `otel-collector*.yaml`
- Application code exposing `/metrics`, tracing middleware, structured logging, or health/readiness endpoints

## Framework and tooling

- OpenTelemetry for vendor-neutral traces, metrics, logs, context propagation, and collector pipelines
- Prometheus and Alertmanager for metric scraping, time-series queries, and alert routing where configured
- Grafana or vendor dashboards for visualization
- Log aggregation/search: Loki, OpenSearch/Elasticsearch, cloud logging, or vendor platform
- Error tracking: Sentry or equivalent where present

## Repo layout conventions

- Read telemetry config, dashboard JSON, alert rules, service instrumentation, and runbooks first
- Define signals around user journeys and service dependencies, not only host-level resource usage
- Keep metric cardinality bounded; avoid labels with user IDs, request IDs, unbounded paths, or raw payload data
- Correlate logs/traces/metrics with trace IDs, request IDs, deployment versions, and environment tags
- Keep alert runbooks close to alert definitions

## Commands

- `promtool check rules <rules>`
- `promtool check config <prometheus.yml>`
- `otelcol --config <config> --dry-run`
- `terraform plan` for dashboard/monitor IaC
- Repo-documented test/build commands for instrumentation changes

## Boundaries

- Do not log secrets, tokens, PII, credentials, full payloads, or sensitive model inputs/outputs
- Do not add noisy alerts without actionability, owner, severity, and runbook
- Do not rely on dashboards as alerts; page only on user-impacting symptoms or fast-burn SLO risk
- Do not add high-cardinality metrics or unbounded log volume without retention/cost plan

## Security / performance appendix

- Prefer RED metrics for request services and USE metrics for infrastructure saturation
- Track golden signals: latency, traffic, errors, saturation; add domain SLIs where needed
- Alert on symptoms before causes; route warnings to tickets/chat, pages to on-call
- Sample traces deliberately and preserve exemplars for high-value errors/latency

## References

- https://opentelemetry.io/docs/
- https://opentelemetry.io/docs/what-is-opentelemetry/
- https://opentelemetry.io/docs/concepts/observability-primer/
- https://kubernetes.io/docs/concepts/cluster-administration/observability/
- https://prometheus.io/docs/introduction/overview/
