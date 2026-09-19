# 9. Compose only, and inferred path mappings

Status: accepted (2026-09-19)

Amends [0005](0005-autostart-and-retry.md) (no retry, no plain container target) and [0003](0003-config-discovery.md) (the workdir comes from the inferred mappings too).

## Decision
**Compose services are the only target.** The `container:` alias key is gone. A plain container had to be created beforehand, by hand, with mounts that `path_mapping` had to repeat; dockshim could neither create nor check it. A five-line `compose.yaml` does the same and is started on demand.

**No retry.** See 0005.

**Path mappings are inferred when `path_mapping` is absent.** Once the service runs, `docker inspect` lists the container mounts:
- Bind mounts whose source is in the project root (symlinks resolved) become mappings: source → destination.
- Bind mounts from outside the project are skipped. Mapping them would reach beyond what the project owns, and they are usually sockets or config files. They are not reported at run time, where a message would repeat on every call and pollute tool output (IDEs read stderr): `dockshim config` lists them instead.
- Volumes and tmpfs are ignored: they have no host path.
- A directory mounted twice maps to the destination that sorts first.

Any `path_mapping`, even `{}`, disables inference entirely, so mounts can be left out deliberately.

**Order.** The service is started first, then its mounts are read, then the command is built: the workdir and path translation depend on the mappings. The compose container id is looked up once per invocation and reused by inspect and the copy steps; only the post-failure check asks again.

**`dockshim config` prints a summary.** Service, user and path mappings of each alias, and the other settings only where they differ from the defaults; var names without their values, paths relative to the root. Inferred mappings are runtime data, so it prints "inferred", not paths. When the service is running, it adds the skipped bind mounts; it never starts the service. `--full` prints the resolved YAML, with `infer_path_mapping`.

## Consequences
- Every invocation with inferred mappings costs one `docker inspect` (~30 ms).
- Mappings follow the running container: after editing the compose volumes, they apply once the container is recreated.
- The mount source is the daemon's view of the path. With a remote daemon, or a Docker Desktop setup that rewrites bind sources, sources don't match the project and nothing is mapped: set `path_mapping`. Support there is knowingly limited, and documented in the README.
