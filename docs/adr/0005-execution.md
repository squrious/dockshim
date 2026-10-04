# 5. Execution

## Decision
- **Start on demand.** A stopped service is started before the command runs.
- **Behave like a child process.** Stdio, TTY, signals and the exit code pass through, so a shim can't be told apart from a local command.
- **Never retry.** The command runs once, even when the service stopped during it.

## Why
- IDEs run shims at any time. Requiring the service to be started first would make them fail opaquely.
- A failed command may have had side effects, and has consumed its stdin: running it again is unsafe.
