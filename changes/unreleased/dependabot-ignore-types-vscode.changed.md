- Dependabot no longer offers `@types/vscode` bumps for the VS Code extension: `vsce package`
  refuses a typing newer than the extension's `engines.vscode` floor (`^1.86.0`), so the
  devDependency stays at the floor and moves with it. A hygiene test holds the pin to the floor
  and the Dependabot configuration to the ignore rule.
