# Third-party notices

The landing-page diagram uses these third-party packages:

- **elkjs** 0.12.0 — licensed under EPL-2.0 OR GPL-3.0-or-later. Source:
  https://github.com/kieler/elkjs
- **libavoid-js** 0.4.5 — licensed under LGPL-2.1-or-later. The full
  [licence text](../assets/landing/libavoid-js.LICENSE.txt) ships with the
  landing-page assets. The licence covers both:
  - `assets/landing/libavoid.wasm`: libavoid compiled to WebAssembly;
  - the libavoid-js JavaScript bindings compiled into
    `assets/landing/diagram.js`.

  Copyright:
  - libavoid: Copyright (C) 2004–2015 Monash University; authors Michael
    Wybrow, Tim Dwyer and Vladyslav Hnatiuk. Its geometry code is partly based
    on "Computational Geometry in C" (Second Edition), Copyright (C) 1998
    Joseph O'Rourke.
  - libavoid-js: Vladyslav Hnatiuk.

  Source:
  - libavoid-js 0.4.5:
    https://github.com/Aksem/libavoid-js/tree/v0.4.5 (commit
    `28a6fe20c2f1f809afdbeca8d789e65357614c4f`).
  - libavoid as that release builds it (`ADAPTAGRAMS_VERSION = "1.0.4"` in
    `tools/generate.py`): `cola/libavoid` in
    https://github.com/Aksem/adaptagrams/tree/v1.0.4 (commit
    `dd3236536399e5297b353d02c0b751eeaaf63aee`), a fork of
    https://github.com/mjwybrow/adaptagrams.
    Before compiling, libavoid-js's build (`tools/generate.py` at v0.4.5) adds
    two forward declarations, `class Router;` and
    `struct HyperedgeNewAndDeletedObjectLists;`, to
    `cola/libavoid/hyperedgeimprover.h`; no other libavoid source is changed.

To use modified libavoid sources, replace `assets/landing/libavoid.wasm` with
one built against the libavoid-js 0.4.5 bindings. To modify the JavaScript
bindings, rebuild `diagram.js` from source with `make docs-landing-assets`.
