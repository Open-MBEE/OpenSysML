# Editor integrations

One directory per editor that OpenSysML plugs into. The language server `sysml-lsp` is what
most editors talk to, over standard input and output; how to point any LSP-capable editor at it
is in [the guide's editors chapter](../docs/guide/08-editors.md).

## VS Code — [`vscode/`](vscode/)

A side-loaded extension, deliberately not published to a marketplace: TextMate grammars for
`.sysml` and `.kerml`, an LSP client that finds or starts `sysml-lsp`, a live diagram panel and
Markdown rendering of native document definitions. The nightly `develop` build is attached to
the [`nightly`](https://github.com/Open-MBEE/OpenSysML/releases/tag/nightly) prerelease as
`opensysml-sysml.vsix` beside the `sysml-lsp` it was built with; [`vscode/README.md`](vscode/README.md)
covers installing that or building from source, and the design of the diagram panel is in
[`docs/internals/design/vscode-visual-modeling.md`](../docs/internals/design/vscode-visual-modeling.md).

## OpenCode

No code lives here: the checkout's [`opencode.json`](../opencode.json) declares `sysml-lsp` to
[OpenCode](https://opencode.ai) so its coding agent receives the server's diagnostics for
`.sysml` and `.kerml` files, and the same block in a user's global configuration enables it for
every project. See [the guide](../docs/guide/08-editors.md#opencode).

## Eclipse SysON — `syson/` (planned)

A plugin for [Eclipse SysON](https://github.com/eclipse-syson/syson), the Sirius Web based
graphical SysML v2 workbench, that lets a user right-click an element and instantiate, execute,
explore or verify it on OpenSysML through the Java client in
[`client/java/opensysml-client`](../client/java/opensysml-client), with the results shown in
SysON and its diagnostics attached to the elements they concern; optionally OpenSysML also
checks textual notation SysON imports. Nothing is implemented yet. The discovery of SysON's
extension points against release `v2026.9.0`, the module layout, the call sequence and the
phased plan are in [`docs/internals/design/syson-plugin.md`](../docs/internals/design/syson-plugin.md).
