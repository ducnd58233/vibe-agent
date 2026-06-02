---
name: observability-monitoring
description: >-
  Designs and implements observability, monitoring, alerting, dashboards, telemetry pipelines, SLIs/SLOs, logs, metrics, traces, and incident visibility tools. Use when adding OpenTelemetry, Prometheus/Grafana, structured logging, health checks, alerts, runbooks, service dashboards, or monitoring for products and infrastructure.
disable-model-invocation: true
---

# Observability Monitoring

## What

Build monitoring and observability systems that explain production behavior through metrics, logs, traces, dashboards, alerts, and runbooks.

## Why

Observability is not “add logs.” Experts design signals around user impact, service dependencies, failure modes, and operator action. Poor telemetry creates blind spots, noisy pages, and high costs.

## How

1. **Load stack context**
   - Inspect telemetry config, instrumentation, alert rules, dashboards, health endpoints, and runbooks.
   - Open [`observability-monitoring.md`](../../stack-profiles/observability-monitoring.md).
2. **Define questions first**
   - What is broken? Who is affected? Since when? Which dependency changed? What is the rollback/mitigation?
   - Convert questions into SLIs, metrics, spans, logs, and dashboard panels.
3. **Instrument the service**
   - Add request latency, traffic, errors, saturation, dependency calls, queue depth, and domain events.
   - Use structured logs with correlation IDs; propagate trace context across service boundaries.
4. **Control cardinality and cost**
   - Avoid unbounded labels and sensitive fields.
   - Set retention, sampling, aggregation, and dashboard scope deliberately.
5. **Alert on symptoms**
   - Page only on user-impacting symptoms or fast-burn SLO risk.
   - Include owner, severity, threshold rationale, dashboard link, and runbook.
6. **Verify observability**
   - Trigger or simulate a known failure where safe.
   - Confirm metrics/logs/traces correlate and the alert/runbook leads to action.

## When

Use for monitoring tools, telemetry instrumentation, dashboards, alerting, SLOs, and incident visibility. Do not use when the task is only cosmetic dashboard formatting without signal design.

## Routing & discovery

- Pair with [`performance-optimization`](../performance-optimization/SKILL.md) for latency/throughput tuning.
- Pair with [`shipping-and-launch`](../shipping-and-launch/SKILL.md) before risky rollout.
- Pair with [`devops-platform-delivery`](../devops-platform-delivery/SKILL.md) for CI/CD and deploy observability.

## Permissions & authority

- Tools: Read, Grep, Glob, Edit; Shell for repo-documented checks such as promtool or tests.
- Paths: telemetry config, dashboards, alert rules, source instrumentation, runbooks.
- Do not access production logs or sensitive telemetry unless explicitly authorized.

## Verification

- [ ] Signals answer concrete operational questions.
- [ ] Metrics avoid high-cardinality labels and sensitive data.
- [ ] Logs, traces, and metrics share correlation context where possible.
- [ ] Alerts are actionable, owned, severity-labeled, and linked to runbooks.
- [ ] Dashboards include user-impact, dependencies, deploy/version markers, and saturation.

## References

- https://opentelemetry.io/docs/
- https://opentelemetry.io/docs/concepts/observability-primer/
- https://kubernetes.io/docs/concepts/cluster-administration/observability/
- https://prometheus.io/docs/introduction/overview/
