- The library index cache (`~/.cache/sysml-ls/libs`) no longer serves one build's records to
  another build of the same commit. The key's build identity fell back to the main module's
  version when the working tree was modified, and since Go 1.24 that version is
  `<pseudo-version>+dirty`, one value for every edit of a commit, so a dirty-tree build and a
  differently patched dirty-tree build — or a worktree build of the same commit — read each
  other's records and answered with facts the other build had derived. The identity is now a
  clean checkout's revision, which its builds share however the link was stamped, and otherwise
  the linker's build ID of the running executable, which the toolchain derives from every
  compiled input, the embedded library included: builds of different code never share a record,
  and a modified tree is never identified by its module version. Existing
  caches are not deleted; records of the old key are unreachable and are pruned after thirty
  idle days.
