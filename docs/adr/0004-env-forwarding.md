# 4. Environment forwarding

## Decision
- Every valid host variable name is forwarded as `--env NAME`, without a value. Docker reads the value from its own environment, so values don't show up in `ps`.
- Configured `vars` are set in the docker process environment and forwarded the same way. They override host values and bypass deny rules.
- A built-in denylist removes host-specific variables (`PATH`, `HOME`, `DOCKER_*`, `SSH_*`...). `MISE_` is deliberately kept.
- Config `deny`, `deny_prefixes` and `allow` extend it (built-in, then global, then alias). `allow` wins over any deny.
- `user` defaults to `host`, the uid:gid running dockshim, so the files it creates belong to the caller.
