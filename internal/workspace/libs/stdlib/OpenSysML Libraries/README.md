# OpenSysML extension libraries (vendored)

The libraries in this directory are maintained upstream at
[Open-MBEE/OpenSysML-Extensions-Library](https://github.com/Open-MBEE/OpenSysML-Extensions-Library).
This copy is pinned by `scripts/extension-libraries-pin.sh` and written by
`scripts/sync-extension-libraries.sh`.

Do not edit these files here: change them upstream, bump the pin, and sync.
CI fails when this copy drifts from the pinned upstream.

`engine-contract.json` lists the qualified names in these libraries that the
engine binds to; the contract is checked here and upstream.
