---
name: devops-platform-delivery
description: >-
  Designs and implements DevOps platform workflows for CI/CD, infrastructure-as-code, containers, deployment gates, release automation, rollback, and delivery metrics. Use when editing pipelines, Docker/Kubernetes/Terraform assets, deployment automation, environment promotion, or platform engineering workflows.
disable-model-invocation: true
---

# DevOps Platform Delivery

## How

<procedure>

1. **Load stack context**
   - Inspect CI files, manifests, deploy scripts, and runbooks.
   - Open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md); load [`devops-platform-cicd.md`](../../stack-profiles/devops-platform-cicd.md) and any runtime/cloud profiles that apply.
2. **Map the delivery path**
   - Source event → validation → build → artifact/image → deploy target → smoke test → monitoring → rollback.
   - Identify environments, approvals, owners, secrets, and protected branches.
3. **Make CI deterministic**
   - Validate formatting/lint/typecheck/unit tests before expensive jobs.
   - Cache safely with exact lockfile/toolchain keys.
   - Upload immutable artifacts; do not rebuild different artifacts during deploy.
4. **Make CD controlled**
   - Separate PR validation from production mutation.
   - Add environment gates, concurrency locks, deployment history, and rollback/disable path.
   - Prefer plan/diff jobs for IaC before apply.
5. **Harden credentials and supply chain**
   - Prefer OIDC/federated identity, least privilege, short-lived tokens, scoped environments.
   - Check dependency audits, container scans, SBOM/signing only when repo policy supports them.
6. **Measure delivery health**
   - Track deployment frequency, lead time, change failure rate, recovery time, flaky jobs, queue time, and deploy duration.
</procedure>

## Routing & discovery

<routing>

- Pair with [`shipping-and-launch`](../shipping-and-launch/SKILL.md) for go/no-go rollout decisions.
- Pair with [`security-and-hardening`](../security-and-hardening/SKILL.md) for credentials, supply chain, and environment access.
- Pair with [`observability-monitoring`](../observability-monitoring/SKILL.md) when delivery dashboards or deploy telemetry are required.

Use for pipeline, IaC, container, deployment, environment, and platform engineering changes. Do not use for app-only code unless delivery automation is affected.
</routing>

## Permissions & authority

<required>

- Tools: Read, Grep, Glob, Edit; Shell for repo-documented validation.
- Paths: CI, scripts, infra, manifests, docs; no secrets.
- Ask before running deploy/apply commands, cloud mutations, long-running jobs, or production-impacting commands.
</required>

## Verification

<verification>

- [ ] Pipeline has clear validation/build/deploy separation and artifact immutability.
- [ ] Secrets are not committed or printed; credentials are least-privilege.
- [ ] Deploys have gates, environment targeting, concurrency protection, and rollback path.
- [ ] IaC changes have fmt/validate/plan or equivalent evidence.
- [ ] Delivery metrics and deploy observability are available or explicitly deferred.
</verification>

## References

<references>

- https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions
- https://developer.hashicorp.com/terraform/intro/core-workflow
- https://docs.docker.com/compose
- https://kubernetes.io/docs/
- https://dora.dev/guides/dora-metrics/
</references>
