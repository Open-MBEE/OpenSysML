// Flat config: type-aware rules over src/, test/ and conformance/; generated
// stubs are excluded because their shape is protoc-gen-es's business, not ours.
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist/", "build/", "src/generated/", "packages/"] },
  ...tseslint.configs.strictTypeChecked,
  {
    languageOptions: {
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
    },
    rules: {
      "@typescript-eslint/restrict-template-expressions": [
        "error",
        { allowNumber: true, allowBoolean: true },
      ],
      // node:test's test() returns a promise the runner awaits itself.
      "@typescript-eslint/no-floating-promises": [
        "error",
        {
          allowForKnownSafeCalls: [
            { from: "package", package: "node:test", name: ["test", "describe", "it"] },
          ],
        },
      ],
    },
  },
  {
    files: ["**/*.mjs"],
    ...tseslint.configs.disableTypeChecked,
  },
);
