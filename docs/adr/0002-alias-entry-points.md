# 2. Alias entry points: symlinks or wrapper scripts

Superseded by ADR 10.

Entry points were either symlinks to the dockshim binary, dispatching on `argv[0]`, or wrapper scripts embedding its path. Both stored the binary's location.
