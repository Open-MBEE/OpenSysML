# Homebrew formula for OpenSysML, maintained here and copied to the tap repo
# Open-MBEE/homebrew-tap as Formula/opensysml.rb.
#
# Per release, exactly eight fields change: the `sha256` values of the bundle
# and of the sysml-mcp archive per platform. Get them from the release's
# SHA256SUMS.txt, e.g. for v0.0.4:
#
#   curl -fL https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.4/SHA256SUMS.txt
#
# and take the line for each opensysml-<os>-<arch>.tar.gz and
# sysml-mcp-<os>-<arch>.tar.gz. The URLs change only in the tag path component; Homebrew scans `version` from that tag, and
# `brew audit --strict` rejects a redundant explicit `version` line.
#
# Or render the whole file (recommended, no manual copying):
#
#   scripts/render-homebrew-formula.sh v0.0.4 > Formula/opensysml.rb
#
# The PLACEHOLDER checksums below are intentionally invalid: this file as
# committed here is the source template. Do not commit it to the tap unrendered.
class Opensysml < Formula
  desc "SysML v2 toolchain: interactive REPL, language server and MCP server"
  homepage "https://github.com/Open-MBEE/OpenSysML"
  license "Apache-2.0"

  # z3 makes the experimental %check/%explain solver path work out of the box;
  # the solver stays optional at runtime, discovered on PATH or via OPENSYSML_SMT.
  depends_on "z3"

  on_macos do
    on_arm do
      url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/opensysml-darwin-arm64.tar.gz"
      sha256 "__SHA256_DARWIN_ARM64__"
      resource "sysml-mcp" do
        url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/sysml-mcp-darwin-arm64.tar.gz"
        sha256 "__SHA256_MCP_DARWIN_ARM64__"
      end
    end
    on_intel do
      url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/opensysml-darwin-amd64.tar.gz"
      sha256 "__SHA256_DARWIN_AMD64__"
      resource "sysml-mcp" do
        url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/sysml-mcp-darwin-amd64.tar.gz"
        sha256 "__SHA256_MCP_DARWIN_AMD64__"
      end
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/opensysml-linux-arm64.tar.gz"
      sha256 "__SHA256_LINUX_ARM64__"
      resource "sysml-mcp" do
        url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/sysml-mcp-linux-arm64.tar.gz"
        sha256 "__SHA256_MCP_LINUX_ARM64__"
      end
    end
    on_intel do
      url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/opensysml-linux-amd64.tar.gz"
      sha256 "__SHA256_LINUX_AMD64__"
      resource "sysml-mcp" do
        url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/sysml-mcp-linux-amd64.tar.gz"
        sha256 "__SHA256_MCP_LINUX_AMD64__"
      end
    end
  end

  def install
    bin.install "sysml", "sysml-lsp"
    man1.install Dir["share/man/man1/*.1"]
    # sysml-mcp is released as its own archive, with its manual page, apart
    # from the bundle.
    resource("sysml-mcp").stage do
      bin.install "sysml-mcp"
      man1.install "share/man/man1/sysml-mcp.1"
    end
  end

  test do
    # Release binaries embed the tag (e.g. "sysml v0.0.4") via ldflags; `version`
    # is that tag without the leading "v", scanned from the URL.
    assert_match version.to_s, shell_output("#{bin}/sysml --version")
    assert_match version.to_s, shell_output("#{bin}/sysml-lsp --version")
    assert_match version.to_s, shell_output("#{bin}/sysml-mcp --version")

    # The MCP server writes a host's configuration naming its own installed path.
    assert_match "#{bin}/sysml-mcp", shell_output("#{bin}/sysml-mcp -print-config opencode")
    assert_path_exists man1/"sysml-mcp.1"

    # The manual pages ship in the bundle archive, so `man sysml` works.
    assert_path_exists man1/"sysml.1"

    # Evaluate an expression non-interactively: exercises lexer, parser, and runtime.
    assert_match "= 8", shell_output("#{bin}/sysml -e '5 + 3'")

    # The z3 dependency is the solver %check/%explain discover on PATH: it must
    # be there and answer SMT-LIB2 on standard input.
    assert_match "sat", pipe_output("z3 -smt2 -in", "(declare-const x Int)\n(assert (> x 5))\n(check-sat)\n", 0)
  end
end
