# Paths

A shim behaves like a host command, so the working directory and path arguments are host paths. dockshim makes them usable in the container.

## Path mapping

`path_mapping` says where host directories (relative to the project root) appear in the container:

```yaml
aliases:
  php:
    service: app
    path_mapping:
      .: /app
      assets: /assets/build
```

It sets the working directory: the current directory is translated through the longest matching mapping. When nothing matches, as with an IDE running from `/`, the container's default applies.

**Inference.** Without `path_mapping`, dockshim reads the bind mounts of the running container (`docker inspect`) and maps those whose source is inside the project. Mounts from elsewhere, such as `/var/run/docker.sock`, are skipped silently, and `dockshim config` lists them while the service runs. Set `path_mapping`, even to `{}`, to choose the mappings yourself: inference is then off.

> **Limited support:** inference compares the mount sources docker reports with the project path. With a remote daemon, or a Docker Desktop setup that reports bind sources under its own paths, nothing matches. Set `path_mapping` explicitly there.

## Path translation

Each argument (or the value of `--opt=value`) that resolves to a host path is handled as follows:
- **Under a mapping**, the path is rewritten: `/home/me/proj/src/a.php` becomes `/app/src/a.php`. A relative path stays as typed when the working directory already resolves it to the same file.
- **A file in an allowed directory** is copied into `/tmp/dockshim-<random>/` in the container, and the copy is removed after the command. The allowed directories are the system temporary ones (`TMPDIR`, `/tmp`, `/var/tmp`) plus `allow`. Copying is meant for the throwaway files a tool hands a command, such as an IDE test runner script. Copies are one-way: changes made in the container are lost.
- **Anything else** keeps its value, so it means the container's own path. This is silent.

Directories are never copied. dockshim warns when a path in an allowed directory can't be copied: a directory, an unreadable or special file, or a file over the size cap.

A symlink counts where it points: a link in `/tmp` to a file elsewhere is not copied, unless `follow_symlinks` is set.

```yaml
global:
  path_translation:
    enabled: true               # default
    allow: [/srv/fixtures]      # extra directories whose files may be copied
    follow_symlinks: false      # true lets a link in an allowed directory point out of it
    max_copy_mb: 100            # default; larger files are left untouched
```

`allow: ["/"]` copies from anywhere. Cleanup needs `rm` in the container.

## Windows paths (WSL)

A Windows tool driving a shim in WSL passes Windows paths. PhpStorm, for example, gives `C:/Users/me/AppData/Local/Temp/ide-phpunit.php` and `\\wsl.localhost\Ubuntu\home\me\proj\tests`, sometimes spelled `//wsl.localhost/Ubuntu/...`. dockshim translates both before the rules above:
- A mounted drive becomes its mount point (`/mnt/c/...`).
- A path in this distribution becomes its Linux path.
- The Windows temporary directories are allowed like the Linux ones.

A Windows path this distribution can't reach, such as an unmounted drive, another distribution or a network share, is left as typed, with a warning.
