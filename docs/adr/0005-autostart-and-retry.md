# 5. Auto-start and retry

Status: accepted (2026-09-16), amended by [0009](0009-compose-only-and-inferred-path-mappings.md): no retry, compose only

## Decision
- Before exec, dockshim checks the service and starts it if it isn't running: `ps --status running -q` / `up --detach` (existing containers and volumes are kept). `up` gets the same environment as exec, alias vars included, so compose interpolates the project identically.
- The command runs as a child process with stdio inherited. SIGTERM and SIGHUP are forwarded to it. SIGINT and SIGQUIT are caught and dropped by dockshim, because the terminal already delivers them to docker. They are not ignored: an ignored signal stays ignored in the child, and docker would then depend on reinstalling its own handler. The child's exit code is propagated (128+n when killed by signal n).
- The command runs once. When it fails and the service is no longer running, dockshim warns that the exit code may come from the container stopping. Stopping a container kills exec'd commands (137, or 143) a moment before docker reports it stopped, so for those codes the state is polled for up to a second.
- `Plan.PostRun` steps always run, in reverse order. Their failures are printed as warnings and don't change the exit code.
- TTY: allocated only when stdin and stdout are both terminals (compose `-T` otherwise).

## Why no retry
It was dropped (0009). A retry ran the command a second time without the user asking, after a failure that may have had side effects, and with whatever stdin the first attempt left. Since the service is started right before exec, it only covered a stop in between.

## Extension points
- Argument rewrites register `PreRun` (run once the target is up) and `PostRun` steps on the `Plan`. Path translation (ADR 0007) is the only one, called directly by `Build`; an interface can come back with a second.
- **Completion (not implemented yet):** planned as a `dockshim completion <shell>` generator that emits per-alias functions calling a hidden `dockshim complete-alias <alias> <words...>`. That command would run the container-side completion. Cobra already reserves `__complete` for dockshim's own completion.
