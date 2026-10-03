# 8. Compose only, and inferred path mappings

## Decision
- Compose services are the only target.
- Without explicit path mappings, they are inferred from the bind mounts of the running container whose source is in the project.
- Mappings are read from the container, not from the compose config.

## Why
- A plain container would be created by hand, with its mounts repeated in the dockshim config. A short compose file does the same, and starts on demand.
- Inference keeps the compose file the single source of truth for mounts.
- Commands run in the existing container, which keeps the mounts it was created with. After an edit to the compose volumes, or with another environment interpolating them, the compose config describes mounts that don't exist.

## Consequences
- Each run with inferred mappings inspects the container, after starting it and before building the command.
- With a remote daemon, or a Docker Desktop setup that rewrites bind sources, nothing matches: mappings must be explicit.
