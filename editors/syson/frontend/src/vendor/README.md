These declarations mirror the small Sirius Web surface used by this package and are compile/test-only.
They keep the default npm build independent of the private Sirius registry.
The real Sirius packages can be installed with `npm run install:syson`.
Run `npm run build:syson` to compile against the installed packages and catch API drift.
The vendor implementations are externalized and never bundled into the published library.
