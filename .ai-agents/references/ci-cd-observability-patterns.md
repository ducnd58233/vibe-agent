# CI/CD and observability patterns

Use this reference when reviewing delivery automation, monitoring, alerting, and release readiness.

## Delivery patterns

<rules>

- Validate before deploy: format/lint/typecheck/unit tests before integration and deploy.
- Deploy immutable artifacts: build once, promote the same artifact across environments.
- Add environment gates and concurrency locks for production deploys.
- Prefer plan/diff before infrastructure mutation.
- Document rollback and disable paths before rollout.

## Observability patterns

- Define user-impacting SLIs before dashboards.
- Use RED metrics for request services: rate, errors, duration.
- Use USE metrics for infrastructure: utilization, saturation, errors.
- Correlate logs, metrics, and traces with deployment version and request/trace IDs.
- Alert on symptoms and fast-burn SLO risk; include owner and runbook.
</rules>

## Review checklist

<verification>

- [ ] CI separates validation, build, artifact, deploy.
- [ ] Deploy credentials are scoped and short-lived where possible.
- [ ] Production jobs cannot overlap unsafely.
- [ ] Dashboards show deploy/version markers.
- [ ] Alerts are actionable and not merely dashboard thresholds.
- [ ] Rollback path has been tested or explicitly risk-accepted.
</verification>

## References

<references>

- https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions
- https://developer.hashicorp.com/terraform/intro/core-workflow
- https://opentelemetry.io/docs/concepts/observability-primer/
- https://prometheus.io/docs/introduction/overview/
- https://dora.dev/guides/dora-metrics/
</references>
