#!/usr/bin/env bash
# Provision the optional PDF toolchain for `sysml -render-document -doc-form pdf`
# into build/doc-pdf/: WeasyPrint (the default engine), pandoc (an alternative
# engine, which also drives WeasyPrint), mermaid-cli (mmdc) for diagram
# pre-rendering, and KaTeX for formula typesetting. Prince is commercial and
# is not provisioned here; install it separately and select it with -pdf-engine prince.
#
# None of these tools is needed to build, test, or render Markdown: PDF output
# alone drives them, as subprocesses. This script pins each version so a PDF
# artifact is reproducible against one toolchain.
#
# Needs: curl, python3 (with venv), node/npm. After it finishes, export the
# variables it prints so the sysml binary finds the pinned copies.
set -euo pipefail

PANDOC_VERSION="3.10.2"
PANDOC_SHA256_AMD64="c7edd535941c48be6a362081a748272837de81ae11777202d9c341d3d8261c9a"
PANDOC_SHA256_ARM64="1c4d69f2a092bd47cb180e58a4aab7b9637101ced928252458c7d41a7f7fa71d"
WEASYPRINT_VERSION="69.0"
MERMAID_CLI_VERSION="11.16.0"
KATEX_VERSION="0.16.47"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dest="$repo_root/build/doc-pdf"
mkdir -p "$dest"

case "$(uname -m)" in
x86_64) pandoc_arch="amd64" pandoc_sha256="$PANDOC_SHA256_AMD64" ;;
aarch64 | arm64) pandoc_arch="arm64" pandoc_sha256="$PANDOC_SHA256_ARM64" ;;
*)
	echo "unsupported architecture $(uname -m); install pandoc yourself and point OPENSYSML_PANDOC at it" >&2
	exit 1
	;;
esac
if [[ "$(uname -s)" != "Linux" ]]; then
	echo "this script provisions Linux binaries; install the tools yourself on $(uname -s)" >&2
	exit 1
fi

pandoc_dir="$dest/pandoc-$PANDOC_VERSION"
if [[ -x "$pandoc_dir/bin/pandoc" ]]; then
	echo "pandoc $PANDOC_VERSION already present at $pandoc_dir"
else
	tarball="$dest/pandoc-$PANDOC_VERSION-linux-$pandoc_arch.tar.gz"
	echo "Fetching pandoc $PANDOC_VERSION ($pandoc_arch) ..."
	curl -fsSL --proto '=https' --proto-redir '=https' -o "$tarball" \
		"https://github.com/jgm/pandoc/releases/download/$PANDOC_VERSION/pandoc-$PANDOC_VERSION-linux-$pandoc_arch.tar.gz"
	echo "$pandoc_sha256  $tarball" | sha256sum -c -
	tar -xzf "$tarball" -C "$dest"
	rm -f "$tarball"
fi

venv="$dest/weasyprint"
if [[ -x "$venv/bin/weasyprint" ]]; then
	echo "WeasyPrint already present at $venv"
else
	echo "Installing WeasyPrint $WEASYPRINT_VERSION into a virtual environment ..."
	python3 -m venv "$venv"
	"$venv/bin/pip" install --quiet "weasyprint==$WEASYPRINT_VERSION"
fi

mermaid="$dest/mermaid"
# puppeteer.json is written last, so it is what says the install finished: mmdc
# exists before the browser it launches has been downloaded.
if [[ -x "$mermaid/node_modules/.bin/mmdc" && -s "$mermaid/puppeteer.json" ]]; then
	echo "mermaid-cli already present at $mermaid"
else
	echo "Installing @mermaid-js/mermaid-cli $MERMAID_CLI_VERSION ..."
	mkdir -p "$mermaid"
	printf '{"dependencies":{"@mermaid-js/mermaid-cli":"%s"}}' "$MERMAID_CLI_VERSION" >"$mermaid/package.json"
	# No dependency's lifecycle script runs; the one that is needed, puppeteer's
	# browser download, is run below on its own, into build/doc-pdf.
	(cd "$mermaid" && npm install --silent --no-fund --no-audit --ignore-scripts)
	# The installer answers "chrome@<version> <path to the executable>"; the path
	# is the rest of the line, spaces and all.
	installed="$(PUPPETEER_CACHE_DIR="$mermaid/browsers" \
		"$mermaid/node_modules/.bin/puppeteer" browsers install chrome)"
	chrome="${installed#* }"
	if [[ ! -x "$chrome" ]]; then
		echo "puppeteer installed no executable browser: $installed" >&2
		exit 1
	fi
	# That browser is outside puppeteer's default cache, so name it here.
	# Sandboxing needs user namespaces, which containers often lack.
	CHROME="$chrome" python3 - >"$mermaid/puppeteer.json" <<'PY'
import json, os

print(json.dumps({"executablePath": os.environ["CHROME"],
                  "args": ["--no-sandbox", "--disable-setuid-sandbox"]}))
PY
fi

katex="$dest/katex"
if [[ -x "$katex/node_modules/.bin/katex" && -s "$katex/node_modules/katex/dist/katex.min.css" ]]; then
	echo "KaTeX already present at $katex"
else
	echo "Installing katex $KATEX_VERSION ..."
	mkdir -p "$katex"
	printf '{"dependencies":{"katex":"%s"}}' "$KATEX_VERSION" >"$katex/package.json"
	(cd "$katex" && npm install --silent --no-fund --no-audit --ignore-scripts)
fi

echo
echo "Done. Point the sysml binary at the pinned copies:"
echo "  export OPENSYSML_PANDOC=$pandoc_dir/bin/pandoc"
echo "  export OPENSYSML_WEASYPRINT=$venv/bin/weasyprint"
echo "  export OPENSYSML_MMDC=$mermaid/node_modules/.bin/mmdc"
echo "  export OPENSYSML_MMDC_PUPPETEER=$mermaid/puppeteer.json"
echo "  export OPENSYSML_KATEX=$katex/node_modules/.bin/katex"
