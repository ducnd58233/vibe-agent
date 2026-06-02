---
name: system-administration-ops
description: >-
  Operates and automates hosts, Linux services, configuration management, shell scripts, backups, permissions, process supervision, and operational runbooks. Use when working on systemd units, Ansible, server bootstrap, service logs, host diagnostics, backup/restore, cron/timers, or sysadmin incident tasks.
disable-model-invocation: true
---

# System Administration Ops

## What

Automate and troubleshoot host operations: services, users, packages, filesystems, logs, backups, network basics, and configuration management.

## Why

Host automation must be idempotent, reversible, least-privilege, observable, and safe to run repeatedly. Manual one-off fixes create drift and fragile production systems.

## How

1. **Load stack context**
   - Inspect runbooks, scripts, inventories, unit files, and host docs.
   - Open [`system-administration.md`](../../stack-profiles/system-administration.md) from the stack router when host operations are involved.
2. **Classify the operation**
   - Read-only diagnosis, local dev mutation, staging mutation, or production mutation.
   - For production, require target inventory, dry-run/check mode, rollback, and explicit approval.
3. **Prefer idempotent automation**
   - Encode desired state in Ansible/roles/scripts instead of manual shell sequences.
   - Make scripts non-interactive for automation unless marked manual-only.
4. **Debug with evidence**
   - Check service status, journal logs, resource pressure, ports, DNS, TLS, filesystem, permissions, and dependency reachability.
   - Record exact failing command/output before changing automation.
5. **Harden operations**
   - Use least-privilege service users, file ownership, resource limits, restart policies, health checks, log rotation, backups, and restore tests.
6. **Document operator path**
   - Update runbooks with symptom, diagnosis command, remediation, rollback, and escalation.

## When

Use for sysadmin tasks, host automation, service supervision, backups, and operational incidents. Do not use for app-layer feature work unless host/runtime automation is touched.

## Routing & discovery

- Pair with [`debugging-and-error-recovery`](../debugging-and-error-recovery/SKILL.md) for incidents.
- Pair with [`security-and-hardening`](../security-and-hardening/SKILL.md) for access, secrets, ports, and service users.
- Pair with [`documentation-and-adrs`](../documentation-and-adrs/SKILL.md) for runbooks and operational decisions.

## Permissions & authority

- Tools: Read, Grep, Glob, Edit; Shell for local validation.
- Paths: ops scripts, inventories, service files, runbooks; no plaintext secrets.
- Ask before host-mutating commands, package installs, service restarts, production inventory runs, or destructive filesystem operations.

## Verification

- [ ] Operation classification and target environment are explicit.
- [ ] Automation is idempotent or clearly manual-only.
- [ ] Logs/status/resource evidence supports the diagnosis.
- [ ] Service security, restart, logging, backup, and rollback concerns are addressed.
- [ ] Runbook or operator note is updated when operational behavior changes.

## References

- https://docs.ansible.com/projects/ansible-core/devel/playbook_guide/playbooks_intro.html
- https://docs.ansible.com/projects/ansible/latest/getting_started/index.html
- https://www.freedesktop.org/software/systemd/man/latest/journalctl.html
