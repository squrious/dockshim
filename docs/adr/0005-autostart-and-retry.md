# 5. Auto-start and retry

Status: accepted (2026-09-16)

## Decision
- Before exec, dockshim checks the target and starts it if it isn't running:
  - compose: `ps --status running -q` / `up --detach` (existing containers and volumes are kept). `up` gets the same environment as exec, alias vars included, so compose interpolates the project identically.
  - container: `inspect` / `start`
- The command runs as a child process with stdio inherited. SIGTERM and SIGHUP are forwarded to it. SIGINT and SIGQUIT are caught and dropped by dockshim, because the terminal already delivers them to docker. They are not ignored: an ignored signal stays ignored in the child, and docker would then depend on reinstalling its own handler. The child's exit code is propagated (128+n when killed by signal n).
- If exec exits with 1 and the target is no longer running, the target is started, `Plan.PreRun` steps are replayed (e.g. files copied again) and the command retried once. The retry reuses the same stdin: whatever the first attempt consumed from a pipe is lost.
- `Plan.PostRun` steps always run, in reverse order. Their failures are printed as warnings and don't change the exit code.
- TTY: allocated only when stdin and stdout are both terminals (compose `-T` otherwise, docker `-it` vs `-i`).

## Extension points
- Argument rewrites register `PreRun` (run once the target is up) and `PostRun` steps on the `Plan`. Path translation (ADR 0007) is the only one, called directly by `Build`; an interface can come back with a second.
- **Completion (not implemented yet):** planned as a `dockshim completion <shell>` generator that emits per-alias functions calling a hidden `dockshim complete-alias <alias> <words...>`. That command would run the container-side completion. Cobra already reserves `__complete` for dockshim's own completion.
