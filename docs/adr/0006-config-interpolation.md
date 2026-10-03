# 6. Environment interpolation in the config

## Decision
- Config values are interpolated from the environment, with Compose's syntax. Keys are not.
- An unset variable without a default is an error.
- Interpolation applies to the parsed values, never to the raw file.

## Why
- Compose users already know the syntax.
- An empty service, user or path would silently run the wrong thing.
- Expanding the raw file would let a value change the document's structure, and lose error positions.
