# 6. Environment interpolation in the config

## Decision
- Every scalar **value** is interpolated from the environment with Compose's syntax. Keys (alias names, mapping paths, var names) are not.
- A variable that is unset and has no default is an error, not an empty string. `${VAR:-}` opts into empty.
- Interpolation runs on the parsed YAML tree, after a strict decode of the raw file. A value can never change the document structure, and errors keep the file's line numbers.

## Why
- Compose users already know the syntax.
- Failing on unset variables avoids silently running with an empty service, user or path.
