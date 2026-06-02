# Stack profile: System administration

## Scope

Applies to consumer repositories that manage hosts, Linux services, packages, users, filesystems, process supervision, configuration management, backups, shell automation, and operational runbooks.

## When to load

- Editing systemd unit files, service scripts, cron/timer jobs, shell scripts, or host bootstrap automation
- Working with Ansible inventories/playbooks/roles or other configuration-management assets
- Debugging service startup, logs, permissions, filesystem, network, package, or resource-limit issues
- Writing runbooks for operators or on-call system administration

## Detection

- `ansible.cfg`, `inventory`, `playbooks/`, `roles/`, `group_vars/`, `host_vars/`
- `systemd/`, `*.service`, `*.timer`, `*.socket`, `cron`, `scripts/*.sh`
- `ops/`, `runbooks/`, `backup/`, `restore/`, `nginx/`, `caddy/`, `haproxy/`, `supervisord`

## Framework and tooling

- Linux service management: systemd, systemctl, journalctl
- Configuration management: Ansible inventories, playbooks, roles, collections, Vault where present
- Host diagnostics: ps/top, ss, lsof, df/du, free, journalctl, dmesg, curl, dig, iptables/nftables
- Reverse proxy/runtime tooling: Nginx, Caddy, HAProxy, Docker/Compose depending on manifests

## Repo layout conventions

- Read README/runbooks, inventory, variables, unit files, and scripts before changing host automation
- Keep idempotent automation: repeated runs should converge without manual cleanup
- Keep host-specific values in inventory/group/host vars; keep reusable logic in roles or shared scripts
- Prefer logging to stdout/stderr for supervised services; let systemd/container infrastructure collect logs
- Document backup, restore, and rollback steps next to operational automation

## Commands

- `ansible-lint`
- `ansible-playbook --check`
- `shellcheck <script>`
- `systemd-analyze verify <unit>`
- `journalctl -u <service>`

## Boundaries

- Do not run host-mutating commands against production without explicit target inventory, dry-run/plan, and approval
- Do not store SSH keys, passwords, tokens, or vault secrets in plaintext
- Do not make scripts depend on interactive prompts unless the runbook says they are manual-only
- Do not hide service logs in bespoke files when supervisor/container logging is the repo convention

## Security / performance appendix

- Enforce least privilege for service users, file ownership, sudoers, SSH, firewall, and exposed ports
- Add resource limits, restart policy, health checks, and log rotation for long-running services
- Keep backups encrypted, restorable, and periodically tested
- Verify time sync, DNS, TLS renewal, disk pressure, memory pressure, and dependency service reachability

## References

- https://docs.ansible.com/projects/ansible-core/devel/playbook_guide/playbooks_intro.html
- https://docs.ansible.com/projects/ansible/latest/getting_started/index.html
- https://www.freedesktop.org/software/systemd/man/latest/journalctl.html
- https://docs.docker.com/compose
