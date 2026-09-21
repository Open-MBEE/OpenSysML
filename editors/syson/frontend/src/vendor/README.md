The ambient declarations in this directory mirror the small Sirius Web surface used by this
package and are compile-only. Runtime test doubles live under `src/test/sirius-doubles` and are
aliased only by Vitest. The real Sirius packages can be installed with `npm run install:syson`;
run `npm run build:syson` to compile against them and catch API drift. Their `exports` omit a
`types` condition, so the real-artifact typecheck uses Node module resolution.
