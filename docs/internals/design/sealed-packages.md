# Sealed packages: encrypted at rest, compiled when the model itself is the secret

A proposal: nothing below §1 is implemented yet. Companion to
[package management over git](package-management.md), which settles how a package is identified,
fetched, locked and installed; this note settles how a package whose content its publisher will
not disclose — a vendor's performance model, an agency's classified library — can still be
shared, installed, imported and executed.

## 0. Goals and non-goals

Goals

- A publisher ships a project nobody but the intended recipients can read: not the git host, not
  the index, not a mirror, not a laptop disk, not a package manager that installs it.
- A recipient holding the key adds the usage to `.project.json`, installs it with the same
  package manager as any other package, and `sysml`, `sysml-lsp`, the REPL, the kernel and the
  MCP server import it as they import a plain `.kpar`.
- When the *content* is the secret rather than the *access* to it, a publisher ships what a
  consumer needs to compose and run against the library without shipping the model text at all.
- Everything OMG-shaped stays OMG-shaped: the thing that is encrypted is a conformant `.kpar`;
  the identity, version and usages a resolver needs stay readable without the key.
- No cryptography of our own. The envelope, the key formats and the signatures are existing,
  audited formats with command-line tools a publisher can use without OpenSysML.

Non-goals

- Hiding a model from the process that executes it. A model `sysml` parses is a syntax tree in
  memory, and that tree prints (§2.3). This note says so rather than pretending otherwise.
- Digital-rights management: a "may load but may not print" rule enforced against the user of an
  open-source binary. The sealed bit of §4.4 is a courtesy to the user, not a boundary.
- A key server, accounts or licence checks. Keys are files; the recipients are a list the
  publisher writes.

## 1. What exists today

| Capability | Where | Relevance |
|---|---|---|
| `.kpar` read into memory, every `.meta.json` checksum verified before a member is read | `internal/workspace/project/archive.go` `OpenArchive`, `ParseArchive`, `verify` | the loader a sealed archive decrypts into; nothing is written to disk on the way |
| `.kpar` written deterministically, `-bundle`/`-unbundle` | `bundle.go`, `cmd/sysml/bundle.go` | the artifact that gets encrypted; `-unbundle` is the one place a plaintext is written |
| Installed-project discovery under `lib/`, `.sysand/lib`, `-library-root` | `discovery.go` `installedProjects` | finds every `.kpar` file and project directory under the roots; a sealed archive is one more file kind to notice |
| Usage → installed project matching by slug and version | `interchange.go` `Uses`, `pickInstalled` | works from `.project.json` alone, which the container keeps in the clear |
| Signed release manifest, cosign keyless, sigstore bundle verified against a pinned CI identity | `client/python/opensysml/signing.py`; `.circleci/config.yml` | the signature pattern a sealed package can reuse |
| Ahead-of-time compilation of a calc to C or Go; differential test against the interpreter | `internal/translate/codegen`; `sysml -compile`; [native compilation](../../project/native-compilation.md) | the "ship the implementation, not the model" route of §4.5 |
| FMU reading: `modelDescription.xml` plus `binaries/<platform>/` from the `.fmu` zip; the `tool:fmi` engine | `internal/exec/fmi`; `tests/fmi` | the industry precedent for a black-box behaviour shipped as interface + binary |
| External engines as processes or WebAssembly modules under a manifest | [bring your own engine](bring-your-own-engines.md) | the host a compiled sealed library plugs into; the remote route of §4.6 |
| Package manager | SysPI (`Open-MBEE/SysPI`), beside sysand | installs the ciphertext as-is; never holds a key |

## 2. What the formats give and do not give

### 2.1 The project interchange format has no encryption

KerML 1.1 Beta 2 §10.3: a project interchange file is "a single project serialized as a set of
model interchange files, archived using the ZIP format"; it *shall* contain a model interchange
file for each root namespace, in one of the §10.2 formats (textual notation, XMI, JSON), and
exactly one `.project.json` and one `.meta.json` at the top level. Table 13's `checksum` is an
integrity record (SHA, BLAKE, MD, ADLER32 — the algorithm list is in note 6) and nothing more:
there is no encryption, signature or recipient field anywhere in Tables 12–13. SysML v2.1
Beta 2's model interchange conformance clause adds only that a SysML project uses the same
`.kpar` extension, `.sysml` members and the SysML metamodel URI.

So an archive whose members are ciphertext is not a `.kpar`: it has no §10.2 interchange file
per root namespace. The sealed container therefore gets its own extension and *contains* a
conformant `.kpar` rather than being a variant of one.

### 2.2 ZIP's own encryption is not an option

ZIP has two encryption schemes, the original ZipCrypto (broken since the 1990s) and the WinZip AES
extension (a vendor extension outside the format's standardized subset). Go's `archive/zip` implements
neither and does not look at the encryption flag: fed a `zip -P` archive, `OpenArchive` reports
`member "P.kerml": zip: not a valid zip file`, the decompressor's reading of the ciphertext.
Java's `java.util.zip`, which the pilot implementation and every Eclipse-based tool read archives
with, refuses encrypted entries as well. A password-protected `.kpar` is unreadable by every
SysML v2 tool, and would not be conformant if it were readable.

### 2.3 A loaded model prints

The AST is syntax-complete and immutable, and the formatter (`internal/syntax/format`) renders it
back to notation: `sysml -convert` to `sysml`/`kerml`, `%save`, LSP formatting and hover, the RDF
export's `sysx:sourceText`, the api-json export and the MCP file tools all re-emit what was loaded.
Encrypting the file protects it everywhere except inside the process of someone who holds the
key — which is exactly what encryption is for. "Stays encrypted to the end user" is achievable in
one of two senses only:

- *access*: the end user cannot load it without a key the publisher gave them (§4.1–4.4), or
- *content*: the end user never receives the model, only something derived from it that they can
  run against (§4.5) or call (§4.6).

The two compose: a compiled library can itself be sealed.

### 2.4 The envelope: age

[age](https://age-encryption.org) (`filippo.io/age`) is the envelope proposed: a small,
specified, audited format with a reference implementation in pure Go, a command-line tool on
every platform, encryption to many recipients at once (X25519 keys, or SSH `ed25519`/`rsa` public
keys as they sit in `~/.ssh` and on a git host's `/<user>.keys` page), hardware recipients through
`age-plugin-yubikey`, authenticated encryption (ChaCha20-Poly1305 with a header MAC), and no key
server. A publisher who never installs OpenSysML can `age -R recipients.txt -o project.skpar.age project.kpar`
and ship the result.

Why not the alternatives:

- **CMS / PKCS#7 enveloped data** (`openssl cms -encrypt`) is the format PKI-issued X.509
  certificates speak, which matters where recipients already hold agency certificates. Go's
  standard library has no CMS; the maintained implementations are third-party and large. Kept as
  an open decision (§6): the container's envelope member is named by format, so a CMS envelope is
  a second allowed value, not a redesign.
- **PGP** carries a web of trust and a key format nobody wants to ship in a modelling tool.
- **A format of our own** — a key in a header and AES-GCM over the archive — is the thing every
  guide to applied cryptography says not to do, and would have no `age`/`openssl` command for a
  publisher to use independently of us.

## 3. Threat model

| Adversary | Protected by | Not protected |
|---|---|---|
| Git host, index, mirror, CDN, package manager, backup, lost laptop | the envelope: the project is ciphertext everywhere it is stored or moved; SysPI and sysand never hold a key | — |
| A recipient who was given the key | nothing — they are the intended reader (§2.3) | the model text, once loaded |
| A recipient whose access should end | per-recipient encryption: the next release is encrypted to a list without them; earlier releases stay readable to them | what they already have |
| Anyone with the publisher's *public* key | the signature: encryption to a public key proves nothing about who wrote the package, so a sealed package is signed as well (§4.2) | — |
| A recipient of a compiled sealed library (§4.5) | compilation: the model text is not in the artifact; the interface stub is | the algorithm, to a determined reverse engineer with a decompiler — the bar every closed-source library accepts |
| A recipient of a remotely served library (§4.6) | distance: the model never leaves the publisher's service | the service's answers, which are the product |

## 4. Proposed design

### 4.1 The container: `.skpar`

A sealed project archive is a ZIP, extension `.skpar`, with exactly these top-level members:

| Member | Clear or sealed | Content |
|---|---|---|
| `.project.json` | clear | the project's own Table 12 record, byte-identical to the one inside the sealed `.kpar`: `name`, `version`, `publisher`, `usage[]`, `license` |
| `.meta.json` | clear | Table 13 metadata for *this* archive: `index` empty, `created`, `metamodel`, and a `checksum` entry for the envelope member |
| `project.kpar.age` | sealed | the conformant `.kpar` of the project, encrypted with age to the recipients |
| `project.kpar.age.sigstore.json` or `project.kpar.age.sig` | clear | the signature over the envelope bytes (§4.2) |

What the split buys:

- Resolution, locking, index records and dependency closure read `.project.json` alone, so SysPI,
  sysand's static index layout and `.project.lock.json` all work on a sealed package unchanged
  and keyless. The usage list is visible; the model is not. A publisher who considers even the
  usage list sensitive publishes a wrapper project whose `.project.json` names only the sealed one.
- The outer `.meta.json` checksum lets the installer verify the download before anyone decrypts
  anything, with the reader that already refuses on a checksum mismatch.
- The inner `.kpar` is an ordinary archive, built by `sysml -bundle`, so the inner `.meta.json`
  index and member checksums are verified after decryption exactly as for a plain archive, and a
  recipient who wants the plaintext for a tool of their own runs `age -d` and has a `.kpar`.
- Nothing in the container is OpenSysML-specific: `unzip`, `age`, `cosign`/`ssh-keygen -Y` and the
  OMG format are the whole toolchain.

The names are fixed so that a reader need not scan: one envelope member, named `project.kpar.age`;
a future CMS envelope would be `project.kpar.p7m`, and a container holds exactly one.

`sysml -bundle -seal -recipient <pubkey|file> [-recipient …] [-sign <identity>]` writes a `.skpar`
from a project directory; `-unbundle` on a `.skpar` requires `-identity` and an explicit
`-unseal` and writes the plaintext project directory. Both live with the existing `-bundle`
code in `cmd/sysml/bundle.go` and `internal/workspace/project/bundle.go`.

### 4.2 Keys and trust

- **Recipients are public keys, one per consumer** (or per consuming organisation), as PEM/OpenSSH
  text. A publisher keeps a `recipients.txt` per package, re-encrypts each release to the current
  list, and removes a recipient by leaving them off the next release. One pre-shared symmetric
  key would leak with the weakest recipient and could not be withdrawn from one without rotating
  all; age's scrypt passphrase recipient exists and is accepted, but the documentation steers to
  keys.
- **Identities** (private keys) are found, in order: `-identity <file>` (repeatable), the
  `OPENSYSML_IDENTITIES` environment variable (path list), `$XDG_CONFIG_HOME/sysml/identities/*`,
  `~/.ssh/id_ed25519`/`id_rsa` when age's SSH recipient support applies, and the LSP
  `initializationOptions.identities`. A sealed archive for which no identity decrypts is an
  `*ArchiveError` naming the archive and saying the identity is missing — never a silent skip, so
  an unresolved import points at the real cause.
- **Signatures.** Encryption proves nothing about origin, so a sealed archive carries a signature
  over the envelope bytes and the loader refuses an unsigned or badly signed one unless
  `-allow-unsigned` is given. Two forms, both already in use around the project: a sigstore bundle
  (`cosign sign-blob`, keyless from the publisher's CI identity — the pattern
  `client/python/opensysml/signing.py` verifies for our own releases) or an SSH signature
  (`ssh-keygen -Y sign`, verified against an allowed-signers file). Trust roots are configured the
  same way identities are. Verification happens before decryption, so an attacker cannot make the
  loader decrypt arbitrary bytes.
- **Hardware and agencies.** `age-plugin-yubikey` (and the PIV/HSM plugins behind the same
  plugin protocol) keep an identity off the disk; a publisher that must use agency-issued X.509
  certificates is the CMS case of §6.

### 4.3 The loader

`installedProjects` learns a second file kind: a `.skpar` under a library root or `lib/` is an
installed project whose `.project.json` is read from the container, so matching, version
selection and the lock work as for a `.kpar`. Decryption is deferred until the project's members
are actually needed — a usage that resolves to a sealed project nobody imports costs nothing and
needs no key. When they are needed:

1. verify the outer `.meta.json` checksum of the envelope member;
2. verify the signature over the envelope bytes against the configured trust roots;
3. decrypt in memory with the first identity that fits, into the existing in-memory `Archive`
   (`ParseArchive` over the plaintext bytes), which verifies the inner `.meta.json` checksums and
   refuses a nested or malformed project as it does today;
4. mark every document of the archive *sealed* (§4.4).

No plaintext touches the disk: `OpenArchive` already reads archives into memory, and the on-disk
per-document analysis cache under `$XDG_CACHE_HOME/sysml-ls/libs` must either skip sealed documents or
store their records encrypted to the same identity — skipping is the first implementation, since
a sealed library is small compared with the standard library the cache exists for.

Where the code lives matters for two gates. `internal/workspace/project` is linked by
`internal/frontend/repl`, which `tests/hygiene` keeps free of transports and which the `sysml_re`
WebAssembly build keeps under its size budget. `filippo.io/age` needs neither `net/http` nor
`crypto/tls`, but it is a new dependency with `golang.org/x/crypto` behind it, so it goes in a
sub-package (`project/seal`) that `cmd/sysml` and the LSP link and the RE build excludes
behind the feature mechanism of `cmd/sysml/features.go` — the shape the package installer
work used for its network code. In the RE build a `.skpar` is reported as sealed with the message
that this build cannot open it.

### 4.4 The sealed bit

Each document loaded from a sealed archive carries a flag, and our own surfaces that re-emit a
document's text — `-convert`, `%save`, `-unbundle`, LSP `textDocument/formatting`, the RDF export's
`sysx:sourceText`, the api-json export, the MCP file-reading tools — refuse a sealed document with
a typed error naming the archive, unless the operation is given `-unseal`. Hover, go-to-definition,
diagnostics, completion, execution, documents and queries work: those are what the recipient was
given the key for. The flag survives the archive's documents only; a model the recipient writes
that imports the sealed one is theirs.

This prevents the accident — `%save` of a session into a public repository, an export that
inlines the library's text — and nothing else. The user holds the key and the binary is open
source; the note says so wherever the flag is documented.

### 4.5 Compiled sealed libraries: ship the implementation, not the model

When the publisher's secret is the content — the sizing algorithm, the degradation model, the
calibration — the only sharing that keeps it is to not share the model. The route is the one
every engineering tool has taken (Modelica tools and FMUs, Simulink protected models, compiled
language libraries): the consumer gets an **interface** they can model against and an
**implementation** they can run but not read.

- **The interface** is a SysML stub, in the clear, generated from the model: every public
  `calc def`, `action def`, `part def` and the features they expose — parameter names, types,
  multiplicities, ports, `doc` comments — with the bodies removed and the definitions marked as
  implemented by the library through metadata (`@SealedLibrary::Implemented` naming the artifact
  and the entry point). The stub is what completion, hover, type checking and diagrams see; a
  consumer connects to and calls exactly what the stub declares.
- **The implementation** is the output of `sysml -compile -lib` over the real model: the native
  compilation record's Phase 7 — a library with the stable C ABI of its item N2.4 (typed
  arguments in, a typed result or a typed failure out, the host's budget honoured) — or, for one
  artifact on every platform, the same source compiled to WebAssembly (`GOOS=wasip1` for the Go
  backend, `wasi-sdk` for the C one) and run by the pure-Go WebAssembly host that
  [bring your own engine](bring-your-own-engines.md) specifies for strategies and engines. The
  FMU shape — `modelDescription.xml` beside `binaries/<platform>/` — is the precedent, and its
  per-platform binary directory is the pain the WebAssembly form avoids.
- **Dispatch.** The interpreter, meeting a body-less definition the stub marks as implemented,
  calls the library through the C ABI or the module's exported function with the arguments it
  would have bound, and treats the answer as it treats a compiled calc's today — the differential
  test between compiled and interpreted code is what makes that substitution sound. A library
  that fails to load, or whose interface digest does not match the stub's, is a typed error at
  the first call, never a silent fallback to nothing.
- **Packaging.** The stub project is an ordinary package (a `.kpar` at a git tag, or sealed under
  §4.1 as well); the library is one more member beside it, `lib/<name>.wasm` or
  `lib/<platform>/<name>.so`, checksummed in `.meta.json` like any member and signed with the
  archive.

What this covers and what it cannot:

- It hides **behaviour**: calcs, constraints, actions and state machines — as far as the compiler
  accepts them. Today that is scalar and sequence calcs (Phase 1, half done); records,
  instances, constraints, actions and state machines are Phases 1–6 of the native compilation
  record and the embedded track's closed IR, in that order. A sealed library is one more customer
  for the C ABI and for those phases, not a reason to compile approximately: a model outside the
  subset is refused with the construct named, and the publisher seals the text (§4.1) instead.
- It cannot hide **structure** a consumer connects to. A `part def` whose ports a consumer wires
  is public by nature, and a requirement a consumer must satisfy must be readable to be satisfied.
  The stub generator exposes exactly the interface and the publisher reviews it.
- It is reversible with effort, like every compiled library. The claim is "not disclosed", not
  "undiscoverable"; combined with §4.1 the artifact is also encrypted at rest.

### 4.6 Served libraries: the model never leaves

For the publisher who will not ship even a compiled form, the library stays on their own
`sysml-grpc` (or a Flexo MMS tenant) and the consumer's model calls it: the stub of §4.5 marks the
definitions as implemented by an **engine** rather than a library, and the external-engine
protocol — a process today, a remote endpoint under the same JSON once the transport note's
session is implemented — carries the call. Nothing new is needed beyond what the engine protocol
already specifies; this section exists so the three routes are stated together and a publisher
can pick by what they are protecting:

| Protecting against | Route |
|---|---|
| storage, transport, mirrors, the wrong recipient | sealed archive (§4.1) |
| the recipient reading the model | compiled library (§4.5), sealed too |
| the recipient having the model in any form | served library (§4.6) |

## 5. Phasing

Phase 1 — sealed archives
1. `project/seal`: age envelope, identity discovery, signature verification (sigstore bundle and
   SSH signature), container read and write; `-bundle -seal`, `-unbundle -unseal`.
2. Discovery and loading of `.skpar` under library roots and `lib/`; deferred decryption; the
   sealed bit and the refusals of §4.4 on every re-emitting surface, each with a test that the
   surface refuses and that `-unseal` lifts it; the cache skips sealed documents.
3. SysPI: install a `.skpar` as-is (it is a file under `lib/`, verified by the outer checksum);
   `.project.lock.json` records its SHA-256 like any archive's.
Gate: a fixture package sealed to two recipients and signed; loaded by one recipient through the
CLI, the LSP, the REPL and the gRPC service; refused for a third identity, for a stripped
signature and for a tampered envelope, each with the named error; `go test ./tests/kpar` passes
unchanged for plain archives; the `sysml_re` size gate unchanged.

Phase 2 — compiled sealed libraries, as the compilation phases allow
4. The interface-stub generator and the `SealedLibrary` metadata; the library member and its
   checksum in the archive.
5. Dispatch from the interpreter into a `-compile -lib` artifact through the C ABI (native
   compilation N2.4), then the WebAssembly form under the engine host.
6. The compiled/interpreted differential run over a sealed fixture: same values, same output,
   same typed failures.
Gate: a calc library compiled, sealed, installed and called from a consumer model with the
interpreter's results; the stub shows no body; the artifact contains no notation.

Phase 3 — served libraries: documentation and one worked example over the engine protocol, no
new code unless the example finds a gap.

## 6. Decisions

Settled by this note: the sealed thing is a conformant `.kpar`; the container keeps `.project.json`
in the clear so package management is keyless; the envelope is an existing format, never ours;
per-recipient public keys, not a shared secret; signed as well as encrypted; the sealed bit is a
courtesy and documented as one; "the model itself is the secret" is answered by compilation or
service, never by obscuring text.

Open:
1. Extension name: `.skpar` is proposed (sealed KerML project archive); the alternative `.kpars`
   reads as a plural of `.kpar`.
2. Envelope: age alone, or age and CMS from the start. CMS matters only where recipients must
   use PKI-issued X.509 certificates; the member-name convention leaves room for it.
3. Signature form to require by default: sigstore bundle (keyless, CI identity, what our releases
   already use) or SSH signatures (no infrastructure, a key per publisher). Both are verified;
   which one is *demanded* when a package carries neither is a policy knob.
4. Whether the analysis cache stores sealed records encrypted rather than skipping them, once a
   sealed library large enough to need it exists.
5. Whether the WebAssembly library form waits for the engine host or justifies it; the native
   compilation record's N2.4 and the engine note's host are the two items a Phase 2 start would
   pull forward.
