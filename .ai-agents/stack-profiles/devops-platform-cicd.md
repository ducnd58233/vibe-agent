# Stack profile: DevOps platform and CI/CD

## Scope

<routing>

Applies to consumer repositories that define delivery pipelines, deployment automation, containers, infrastructure-as-code, environments, release gates, and platform workflows.

## When to load

- Editing `.github/workflows/`, GitLab CI, Jenkins, Buildkite, CircleCI, or other pipeline definitions
- Adding build, test, package, deploy, rollback, release, or artifact-publishing automation
- Working on Dockerfiles, Compose, Kubernetes manifests, Helm/Kustomize, Terraform/OpenTofu, or cloud deploy scripts
- Designing deployment promotion across dev, staging, production, canary, or blue/green environments
</routing>

## Detection

<context>

- `.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, `buildkite/`, `.circleci/`
- `Dockerfile`, `compose.yaml`, `docker-compose.yml`, `k8s/`, `helm/`, `charts/`, `kustomization.yaml`
- `terraform/`, `infra/`, `*.tf`, `.terraform.lock.hcl`, `opentofu`, `pulumi`, cloud deployment scripts

## Framework and tooling

- CI/CD: GitHub Actions, GitLab CI, Jenkins, Buildkite, CircleCI, or repo-pinned runner
- Containers: Docker / Docker Compose; Kubernetes with Helm or Kustomize when present
- IaC: Terraform/OpenTofu, Pulumi, or cloud-native templates
- Supply chain: lockfiles, SBOM, image signing/scanning, dependency audit, least-privilege deployment credentials

## Repo layout conventions

- Read CI files, deployment scripts, manifests, environment docs, and release/runbook docs before editing
- Keep CI fast and deterministic: lint/typecheck/unit tests before integration/e2e/deploy
- Keep deploy jobs separate from PR validation unless the repo intentionally combines them with clear gates
- Store artifacts immutably; deploy built artifacts/images, not source re-builds in production jobs
- Keep environment-specific values in environment configs or secret stores, not pipeline logic
</context>

## Commands

<procedure>

- Use repo-documented pipeline validation first
- Typical examples: `docker build .`, `docker compose config`, `terraform fmt -check`, `terraform validate`, `terraform plan`, `helm lint`, `kubectl diff`
- Never run apply/deploy commands without explicit user approval and environment confirmation
</procedure>

## Boundaries

<required>

- Do not put secrets in workflow YAML, Dockerfiles, Terraform variables, logs, or committed env files
- Do not grant broad cloud/admin credentials to PR workflows or untrusted forks
- Do not auto-deploy from unreviewed branches unless the repo policy explicitly allows it
- Do not mutate production infrastructure from local shell without reviewed plan, rollback, and approval

## Security / performance appendix

- Prefer OIDC/federated identity over long-lived cloud keys where supported
- Add concurrency controls to prevent overlapping deploys to the same environment
- Cache dependencies only with correct keys and invalidation; treat cache poisoning as a risk
- Track DORA-style delivery metrics: deployment frequency, lead time, change failure rate, and recovery time
</required>

## References

<references>

- https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions
- https://developer.hashicorp.com/terraform/intro/core-workflow
- https://developer.hashicorp.com/terraform/language/state
- https://docs.docker.com/compose
- https://kubernetes.io/docs/
- https://dora.dev/guides/dora-metrics/
</references>
