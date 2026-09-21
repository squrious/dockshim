# 5. Execution

## Decision
- **Auto-start.** If the service isn't running, dockshim runs `docker compose up --detach` for it, with the same environment as the exec, so compose interpolates the project identically.
- **Process.** The command runs as a child with inherited stdio:
  - SIGTERM and SIGHUP are forwarded to it.
  - SIGINT and SIGQUIT are caught and dropped, since the terminal already delivers them to docker. They are not ignored, because an ignored disposition would be inherited by docker.
  - The exit code is propagated, as 128+n when the child is killed by signal n.
- **TTY** is allocated only when stdin and stdout are both terminals.
- **No retry.** The command runs once. When it fails and the service has stopped, a warning says the exit code may come from the container stopping. For 137/143 the state is polled for up to a second, since docker reports the stop late.
- `PreRun` / `PostRun` steps on the `Plan` wrap the command, for path translation copies and their cleanup. `PostRun` failures are warnings.

## Why no retry
A retry would run the command a second time after a failure that may have had side effects, and with a stdin the first attempt already consumed.
