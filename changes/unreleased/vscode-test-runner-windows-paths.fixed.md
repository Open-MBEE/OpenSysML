- **The VS Code extension's test runner finds `src/` on Windows.** `tools/test.mjs` derived its
  `src/` and `out/` directories from a file URL's `pathname`, which on Windows carries a leading
  slash before the drive letter, so `path.resolve` prefixed the current drive again and
  `npm test` / `npm run package` failed with `ENOENT … scandir 'C:\C:\…\src'`. The paths now come
  from `fileURLToPath`, which yields a native path on every platform.
