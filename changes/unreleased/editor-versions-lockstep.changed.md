- **The editors carry the core version, in lockstep with the clients.** The VS
  Code and SysON frontend package.json files (and their locks), the Cameo and
  SysON poms and their children's `<parent><version>` moved from `0.1.0` /
  `0.1.0-SNAPSHOT` to `0.9.0`, the version `_version.py` declares. Nothing
  publishes the editors, but `check_version.py --editors` now fails a release
  early when any of them disagrees, alongside a pytest gate on every PR.
