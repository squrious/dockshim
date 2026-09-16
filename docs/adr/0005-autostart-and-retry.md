# 5. Auto-start and retry

Status: accepted (2026-09-16)

## Decision
- Before exec, dockshim checks the target and starts it if it isn't running:
  - compose: `ps --status running -q` / `up --detach` (existing containers and volumes are kept)
  - container: `inspect` / `start`
- The command runs as a child process with stdio inherited. SIGTERM and SIGHUP are forwarded to it. SIGINT and SIGQUIT are ignored by dockshim, because the terminal already delivers them to docker. The child's exit code is propagated (128+n when killed by signal n).
- If exec exits with 1 and the target is no longer running, the target is started and the command retried once. The retry reuses the same stdin: whatever the first attempt consumed from a pipe is lost.
- TTY: allocated only when stdin and stdout are both terminals (compose `-T` otherwise, docker `-it` vs `-i`).

## Extension points (not implemented yet)
- **Path translation:** `execplan.ArgTransformer` rewrites args and can register `Plan.PreRun` / `Plan.PostRun` steps.
  - A transformer can copy host files outside the project into `/tmp/<tmpdir>` in the container (`docker cp` / `docker compose cp`).
  - `PostRun` steps always run, which guarantees cleanup.
  - `PreRun` runs after the target is up.
  - A retry currently does not replay `PreRun`. Revisit this when adding transformers.
- **Completion:** planned as a `dockshim completion <shell>` generator that emits per-alias functions calling a hidden `dockshim complete-alias <alias> <words...>`. That command would run the container-side completion. Cobra already reserves `__complete` for dockshim's own completion.
