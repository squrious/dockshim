# 8. Compose only, and inferred path mappings

## Decision
**Compose services are the only target.** A plain container would have to be created by hand, with mounts that `path_mapping` would have to repeat. A five-line `compose.yaml` does the same and starts on demand.

**When `path_mapping` is absent, mappings are inferred.** Once the service is up, `docker inspect` lists the container mounts:
- Bind mounts whose source is in the project root (symlinks resolved) become mappings.
- Bind mounts from outside the project (sockets, config files) are skipped. They aren't reported at run time, since IDEs read stderr, but `dockshim config` lists them.
- Volumes and tmpfs have no host path and are ignored.

Any `path_mapping`, even `{}`, disables inference.

**Order.** Start the service, read its mounts, then build the command, since the workdir and path translation depend on the mappings. The container id is looked up once per invocation.

## Consequences
- Each invocation with inferred mappings costs one `docker inspect` (~30 ms).
- Mappings follow the running container: an edit to the compose volumes applies once the container is recreated.
- With a remote daemon, or a Docker Desktop setup that rewrites bind sources, nothing matches and `path_mapping` must be set.
