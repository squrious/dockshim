# 4. Environment forwarding

Status: accepted (2026-09-16)

## Decision
- Every valid host env var name is forwarded as `--env NAME`, with no value. Docker reads the value from its own env, so values don't show up in `ps`.
- Configured `vars` are injected into the docker process env and forwarded the same way. They override host values and bypass deny rules.
- Built-in denylist, always applied:
  - names: `PATH HOME PWD OLDPWD TMPDIR TMP TEMP USER LOGNAME HOSTNAME SHELL SHLVL _ LS_COLORS`
  - prefixes: `COMPOSE_ DOCKER_ SSH_ XDG_ WSL BASH_FUNC_`
  - `MISE_` is deliberately not denied.
- Config `deny`, `deny_prefixes` and `allow` are appended: builtin, then global, then alias. `allow` wins over any deny.
- `user` defaults to `host`, the uid:gid running dockshim. It can be overridden globally or per alias.
