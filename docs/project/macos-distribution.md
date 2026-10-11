# macOS Distribution: Gatekeeper, Signing, and Notarization

Status: **decision record** — records what was measured, what was changed, and what a
maintainer must provide to remove the macOS security prompt entirely.
Last updated: 2026-08-06.

## 1. The symptom

A user who downloads `sysml-darwin-arm64.tar.gz` (or the Intel/`sysml-lsp` equivalents)
from GitHub Releases in a browser, extracts it, and runs it gets:

> "sysml-darwin-arm64" cannot be opened because the developer cannot be verified.

and has to go to System Settings → Privacy & Security → **Open Anyway**.

## 2. Root cause: quarantine, not a missing signature

There are two distinct macOS mechanisms that can block a downloaded binary. Only one of
them is biting us.

### 2.1 Executability (code signature) — already satisfied

macOS on Apple silicon refuses to execute arm64 code with no valid signature attached
([Apple Platform Security, "Rosetta 2 on a Mac with Apple silicon"](https://support.apple.com/guide/security/rosetta-2-on-a-mac-with-apple-silicon-secebb113be1/web)).
An *ad-hoc* signature is enough to satisfy this.

Go's linker emits that ad-hoc signature itself, including when cross-compiling from Linux
(see [golang/go#42684](https://github.com/golang/go/issues/42684) and
[`cmd/internal/codesign`](https://go.dev/src/cmd/internal/codesign/codesign.go)).

Measured on a Linux host with Go 1.25.0, cross-compiling this repository's `cmd/sysml`
exactly as the `build-release` CircleCI job does, then parsing the Mach-O load commands:

| Target | `LC_CODE_SIGNATURE` | CodeDirectory flags |
| --- | --- | --- |
| `GOOS=darwin GOARCH=arm64` | present (57 KB blob, `0xfade0cc0` superblob) | `0x20002` = `adhoc, linker-signed`, identifier `a.out` |
| `GOOS=darwin GOARCH=amd64` | absent | n/a (x86-64 does not require one) |

Those are the same flags `codesign -dv` reports as `flags=0x20002(adhoc,linker-signed)` in
the Go issue above. **Untested claim (no macOS host available here):** that the binary
therefore launches on Apple silicon once quarantine is cleared — this follows from the
Apple/Go documentation cited, but it was not executed on a Mac as part of this work.

Consequence: adding our own ad-hoc signing step (`codesign -s -`) would change nothing. It
does not, and cannot, satisfy Gatekeeper for a quarantined download — an ad-hoc signature
carries no Developer ID and no notarization ticket.

### 2.2 Gatekeeper / quarantine — this is the prompt users see

Programs that download files on behalf of a user (Safari, Chrome, Mail, and anything else
that opts into `LSFileQuarantineEnabled`) tag the downloaded file with the
`com.apple.quarantine` extended attribute. Archive utilities propagate that attribute to
the files they extract, so the extracted `sysml-darwin-arm64` is quarantined too. On first
launch, Gatekeeper evaluates a quarantined binary and blocks anything that is not signed
with a Developer ID *and* notarized — which is exactly the prompt in §1
(Apple: ["Safely open apps on your Mac"](https://support.apple.com/en-us/102445),
["Notarizing macOS software before distribution"](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)).

Two consequences worth knowing:

- The attribute is set by the *downloader*, not by the file. `curl`, `wget`, `git`, and
  `go install` do not set it, so binaries obtained those ways never trigger the prompt.
  This is why `brew install` is prompt-free (Homebrew applies quarantine only to *casks* —
  hence the cask-only `--no-quarantine` flag — not to formulae).
- `xattr -d com.apple.quarantine <file>` removes it, which is why that workaround "works".

## 3. Options considered

| Option | Removes the prompt for a browser download? | Cost | Requires credentials we don't have |
| --- | --- | --- | --- |
| Developer ID signing + notarization (stapling only if we ship `.pkg`/`.dmg`, see §5.3) | **Yes** — the only option that does | $99/yr Apple Developer Program + macOS CI minutes | Yes |
| Homebrew tap/formula (**accepted stopgap**) | Yes, for users who install via `brew` (curl download, no quarantine) | Free (a tap repo) | No, but needs a new repo |
| `go install .../cmd/sysml@latest` | Yes (built locally, never quarantined) | Free; user needs a Go toolchain | No |
| Ad-hoc signing in CI (`codesign -s -`) | **No** — see §2.1; also already effectively done by the Go linker | Needs a macOS runner | No |
| Documented `xattr -d com.apple.quarantine` | Yes, but by having the user disable a security check | Free | No |

Notes on the last row: telling users to strip quarantine is a poor default because it
trains them to bypass Gatekeeper on any download, and it gives them no way to tell our
archive from a tampered one. It is documented in `docs/guide/01-install.md` as an escape hatch
*next to* the checksum verification step, not as the recommended path.

## 4. What this change actually lands

1. **Docs** (`README.md`, `docs/guide/01-install.md`): a macOS section that recommends
   `brew install Open-MBEE/tap/opensysml`, with `curl`/`go install` and the
   quarantine-clearing + checksum-verification commands as fallbacks.
2. **Release artifacts** (`.circleci/config.yml`): in addition to the existing per-binary
   archives (unchanged, so existing links keep working), the `build-release` job now
   publishes `opensysml-<os>-<arch>.tar.gz`/`.zip` bundles containing `sysml` and
   `sysml-lsp` under their plain names, plus a `SHA256SUMS.txt` covering every archive.
   Both are prerequisites for a Homebrew formula and for users who want to verify a
   download.
3. **Homebrew formula** (`packaging/homebrew/Formula/opensysml.rb`) plus
   `scripts/render-homebrew-formula.sh`, which renders it for a tag from `SHA256SUMS.txt`.
   Homebrew is the accepted macOS install path until notarization exists. The tap repository
   `Open-MBEE/homebrew-tap` now exists and carries the rendered 0.0.4 formula; per-release
   maintenance steps are in `packaging/homebrew/README.md`.

## 5. Decision record: notarization

**Decision: not implemented in this change, because it cannot be done without credentials
the maintainer must obtain. Homebrew is the accepted stopgap in the meantime** (maintainer
decision, 2026-08-06). Nothing about the change above conflicts with adding it later; the
notarization step would run after the existing build step and re-tar the signed binaries.

### 5.1 What the maintainer must provide

| # | Item | How to obtain | Where it goes |
| --- | --- | --- | --- |
| 1 | Apple Developer Program membership | https://developer.apple.com/programs/ — $99/year, organization enrollment needs a D-U-N-S number and takes days-to-weeks | prerequisite for 2–4 |
| 2 | **Developer ID Application** certificate (not "Apple Development", not "Mac App Distribution") | Certificates, Identifiers & Profiles → create a CSR on a Mac → download `.cer` → export as `.p12` with a password | CI secret `MACOS_CERT_P12` (base64) + `MACOS_CERT_PASSWORD` |
| 3 | App Store Connect **API key** for `notarytool` (preferred over an Apple ID + app-specific password: no 2FA interaction, revocable) | App Store Connect → Users and Access → Integrations → Keys → download the `.p8` **once** | CI secrets `APPLE_API_KEY_P8` (base64), `APPLE_API_KEY_ID`, `APPLE_API_ISSUER_ID` |
| 4 | Team ID | Apple Developer account membership page | CI secret `APPLE_TEAM_ID` |
| 5 | A decision on where it runs (see §5.2) | — | — |

Without item 2 specifically, nothing works: only a Developer ID certificate produces a
signature Gatekeeper accepts for software distributed outside the App Store.

### 5.2 Which CI provider it would run on

`codesign`, `notarytool`, and `stapler` are Xcode command-line tools and only run on macOS.
The current release pipeline is CircleCI (`.circleci/config.yml`, `release` workflow on
`v*` tags), which cross-compiles on the Linux `cimg/go:1.25` executor; GitHub Actions runs the PR
checks (`.github/workflows/pr.yml`), the Windows installer and signing workflow
(`release-windows.yml`, see the Windows section below), the nightly snapshot and the
documentation site, none of which mirrors the release workflow.

- **CircleCI macOS VM**: `macos: {xcode: ...}` with `resource_class: m4pro.medium` costs
  **200 credits/minute**, and macOS VMs are not available on the Free plan; CircleCI's
  400,000 free open-source credits/month apply to Linux, Arm, and Docker only, so macOS
  jobs draw on the 30,000-credit free allowance
  ([CircleCI price list](https://circleci.com/pricing/price-list/),
  [pricing](https://circleci.com/pricing/)).
- **GitHub Actions `macos-latest`**: a standard GitHub-hosted runner, and standard runners
  are **free for public repositories**
  ([GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)).
  (`macos-*-large`/`-xlarge` are *larger* runners and are always billed — do not use them.)

**Recommendation:** add a tag-triggered GitHub Actions release workflow on `macos-latest`
for the darwin artifacts only, and keep CircleCI for everything else — or move the whole
release there. Doing it on CircleCI is functionally fine but costs credits and, on the Free
plan, is not available at all. This only stays free if the repository is public.

### 5.3 What the pipeline steps would look like

Sketch, not tested (no macOS host available here); step order and flags follow Apple's
["Customizing the notarization workflow"](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow):

```yaml
# .github/workflows/release-macos.yml (sketch), on: push: tags: ['v*']
runs-on: macos-latest
steps:
  - uses: actions/checkout@v4
  - uses: actions/setup-go@v5
    with: { go-version-file: go.mod }

  # 1. Import the Developer ID certificate into a throwaway keychain.
  - run: |
      echo "$MACOS_CERT_P12" | base64 -d > cert.p12
      security create-keychain -p "$KEYCHAIN_PW" build.keychain
      security default-keychain -s build.keychain
      security unlock-keychain -p "$KEYCHAIN_PW" build.keychain
      security import cert.p12 -k build.keychain -P "$MACOS_CERT_PASSWORD" \
        -T /usr/bin/codesign
      security set-key-partition-list -S apple-tool:,apple: -s -k "$KEYCHAIN_PW" build.keychain
      rm cert.p12

  # 2. Build both darwin architectures.
  - run: |
      GOOS=darwin GOARCH=arm64 make build-sysml VERSION="$GITHUB_REF_NAME" ...
      GOOS=darwin GOARCH=amd64 make build-sysml VERSION="$GITHUB_REF_NAME" ...
      # ...and build-lsp for both

  # 3. Sign with the hardened runtime (notarization requires --options runtime).
  - run: |
      codesign --force --timestamp --options runtime \
        --sign "Developer ID Application: <NAME> ($APPLE_TEAM_ID)" \
        dist/sysml-darwin-arm64 dist/sysml-darwin-amd64 \
        dist/sysml-lsp-darwin-arm64 dist/sysml-lsp-darwin-amd64
      codesign --verify --strict --verbose=2 dist/sysml-darwin-arm64

  # 4. Notarize. notarytool accepts .zip/.pkg/.dmg, not .tar.gz, so zip for submission.
  - run: |
      echo "$APPLE_API_KEY_P8" | base64 -d > key.p8
      ditto -c -k --keepParent dist/sysml-darwin-arm64 notarize.zip
      xcrun notarytool submit notarize.zip --wait \
        --key key.p8 --key-id "$APPLE_API_KEY_ID" --issuer "$APPLE_API_ISSUER_ID"
      rm key.p8

  # 5. Tar the *signed* binaries and publish. Do not modify a binary after signing —
  #    any edit (including strip) invalidates the signature.
```

Two constraints that shape the above:

- **Stapling does not apply to bare executables.** Apple: "Although tickets are created for
  standalone binaries, it's not currently possible to staple tickets to them"
  (["Customizing the notarization workflow"](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow)).
  Gatekeeper then fetches the ticket online at first launch. Getting a *stapled* artifact
  would mean shipping a `.pkg` or `.dmg` instead of a tarball — a larger packaging change,
  worth doing only if offline first launch matters.
- **Signing must happen after the last modification** of the binary, and the tarball must
  be built from the signed files.

### 5.4 Recurring cost

$99/year (Apple Developer Program) plus CI time: on GitHub Actions `macos-latest` for a
public repository, $0; on CircleCI, ~200 credits/minute of macOS VM time per release.

## 6. Windows

Out of scope for this change: the Windows artifacts CircleCI publishes are unsigned too, so
SmartScreen shows "Windows protected your PC" for a downloaded `sysml-windows-amd64.exe`.
The remedy taken is not a commercial certificate but free Authenticode signing by
[SignPath Foundation](https://signpath.org): once the project's application is approved,
`.github/workflows/release-windows.yml` rebuilds the three Windows executables on GitHub
Actions for each `v*` tag, has SignPath sign them after an Approver's manual approval, and
publishes them as `*-signed*` assets beside the unsigned ones. The user-facing policy is the
README's [Code signing policy](../../README.md#code-signing-policy); the maintainer procedure
is in [releasing.md](releasing.md), "Windows Authenticode signing". SmartScreen reputation for
the new certificate still builds over download volume, so early signed downloads may warn less
rather than not at all.
