# 6. Environment interpolation in the config

Status: accepted (2026-09-17)

## Decision
- Every scalar **value** in the config is interpolated from the environment dockshim runs in. Mapping keys (alias names, `path_mapping` host paths, var names) are not interpolated.
- Compose-like syntax:
  - `$VAR`, `${VAR}`
  - `${VAR:-default}`: default when unset or empty
  - `${VAR-default}`: default when unset only
  - `${VAR:?message}`, `${VAR?message}`: fail with the message
  - defaults can nest: `${A:-${B}}`
  - `$$` is a literal `$`. A `$` not followed by a name or `{` is kept as is.
- A variable that is unset and has no default is an **error**, not an empty string. `${VAR:-}` opts into empty.
- Interpolation runs on the parsed YAML tree, after a strict decode of the raw file:
  - A variable's value can never change the document structure.
  - Unknown-field errors keep the file's own line numbers.
  - Interpolation errors report the line of the value.
- Validation and `dockshim config` see the interpolated values.

## Why
- Compose users already know the syntax.
- Failing on unset variables avoids silently running with an empty service, user or path.
