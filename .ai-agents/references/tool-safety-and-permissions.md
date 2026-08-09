# Tool safety and permissions

Use this reference when hardening `.claude/settings.json`, subagent `tools:`, hooks, and commands that may run shell or external tools.

## Permission model

<rules>

- Prefer **least privilege**: allow common read/validation paths; ask for mutating, network, install, deploy, or destructive operations.
- Treat hooks as code execution. Hook paths must exist, be router-listed, and document stdin/stdout behavior.
- Deny secrets paths where supported; never rely on model judgment alone to avoid secrets.
- Review `allow` entries for broad wildcards such as `Bash(*)`, `Edit(*)`, `Write(*)`, `WebFetch(domain:*)`, and `mcp__*`.

## Risk classes

| Class | Examples | Default posture |
|---|---|---|
| Read-only discovery | `git status`, `git diff`, router checks | allow |
| Local validation | lint/test/build/check scripts | allow or ask by command family |
| Local mutation | edits, generated links, formatting | allow only in scoped paths |
| Dependency/network | package managers, curl/wget, web fetch all domains | ask |
| Infra/deploy | Docker, kubectl, Terraform apply, cloud CLIs | ask |
| Destructive | rm/rmdir/dd/shred/force push | deny or ask with explicit user request |
</rules>

## Review checklist

<verification>

- [ ] No missing hook command paths.
- [ ] Broad allow rules are justified or removed.
- [ ] Secrets paths are denied or explicitly marked out of scope.
- [ ] Subagent `tools:` maps match router documentation.
- [ ] Commands that may mutate infra/deploy state require approval.
- [ ] Hook scripts have tests or smoke checks when they transform files.
</verification>

## References

<references>

- https://docs.claude.com/en/docs/claude-code/settings
- https://docs.claude.com/en/docs/claude-code/hooks
- https://docs.claude.com/en/docs/claude-code/sub-agents
- https://openai.github.io/openai-agents-python/guardrails/
</references>
