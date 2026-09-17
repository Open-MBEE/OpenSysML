#!/usr/bin/env bash
# Provision the optional PDF toolchain for `sysml -render-document -doc-form pdf`
# into build/doc-pdf/: WeasyPrint (the default engine), pandoc (an alternative
# engine, which also drives WeasyPrint), mermaid-cli (mmdc) for Mermaid diagram
# pre-rendering, Graphviz for DOT diagrams, the PlantUML jar for PlantUML
# diagrams, and KaTeX for formula typesetting. Prince is commercial and is not
# provisioned here; install it separately and select it with -pdf-engine prince.
# Java, which runs the PlantUML jar, is not provisioned either: it is taken from
# PATH, or from where OPENSYSML_JAVA points.
#
# None of these tools is needed to build, test, or render Markdown: PDF output
# alone drives them, as subprocesses. This script pins each version so a PDF
# artifact is reproducible against one toolchain.
#
# Needs: curl, python3 (with venv), node/npm, and for Graphviz dpkg-deb (or ar
# and tar). After it finishes, export the variables it prints so the sysml
# binary finds the pinned copies.
set -euo pipefail

PANDOC_VERSION="3.10.2"
PANDOC_SHA256_AMD64="c7edd535941c48be6a362081a748272837de81ae11777202d9c341d3d8261c9a"
PANDOC_SHA256_ARM64="1c4d69f2a092bd47cb180e58a4aab7b9637101ced928252458c7d41a7f7fa71d"
WEASYPRINT_VERSION="69.0"
MERMAID_CLI_VERSION="11.16.0"
KATEX_VERSION="0.16.47"
# Graphviz publishes x86_64 Debian packages per Ubuntu release; the tarball for
# the host's release is unpacked (not installed) under build/doc-pdf/graphviz.
GRAPHVIZ_VERSION="16.1.0"
GRAPHVIZ_SHA256_UBUNTU_22_04="35e749e7f87882a4ea2d99c260e7e5cefa77bc5c69a04d9a4873295784b6d14e"
GRAPHVIZ_SHA256_UBUNTU_24_04="1ac34dac4dde843ec0069d51c4a5e06f036dbf9a4303f4aaf157e0c8e03e4def"
GRAPHVIZ_SHA256_UBUNTU_26_04="f183e9e351576ab323ded649074cfef7945b7de223c7e43c42fb6ec2d3e32a7f"
PLANTUML_VERSION="1.2026.8"
PLANTUML_SHA256="5e1ecfa8ecd32c90b03bbf3b1eb6f020943f98ab0fcf4032be31a0002ee2c462"

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

graphviz="$dest/graphviz"
# Graphviz is x86_64 only here; elsewhere the package manager's dot serves,
# pointed at by OPENSYSML_DOT.
graphviz_note=""
if [[ -x "$graphviz/bin/dot" ]]; then
	echo "Graphviz $GRAPHVIZ_VERSION already present at $graphviz"
elif [[ "$pandoc_arch" != "amd64" ]]; then
	graphviz_note="Graphviz is not published for $(uname -m); install it with the package manager and point OPENSYSML_DOT at its dot."
else
	ubuntu_version="$(. /etc/os-release 2>/dev/null && [[ "${ID:-}" == "ubuntu" ]] && echo "${VERSION_ID:-}" || true)"
	case "$ubuntu_version" in
	22.04) graphviz_sha256="$GRAPHVIZ_SHA256_UBUNTU_22_04" ;;
	24.04) graphviz_sha256="$GRAPHVIZ_SHA256_UBUNTU_24_04" ;;
	26.04) graphviz_sha256="$GRAPHVIZ_SHA256_UBUNTU_26_04" ;;
	*) graphviz_sha256="" ;;
	esac
	if [[ -z "$graphviz_sha256" ]]; then
		graphviz_note="Graphviz is published for Ubuntu 22.04, 24.04 and 26.04 only; install it with the package manager and point OPENSYSML_DOT at its dot."
	else
		echo "Fetching Graphviz $GRAPHVIZ_VERSION (Ubuntu $ubuntu_version) ..."
		tarball="$dest/graphviz-$GRAPHVIZ_VERSION-debs.tar.xz"
		curl -fsSL --proto '=https' --proto-redir '=https' -o "$tarball" \
			"https://gitlab.com/api/v4/projects/4207231/packages/generic/graphviz-releases/$GRAPHVIZ_VERSION/ubuntu_${ubuntu_version}_graphviz-$GRAPHVIZ_VERSION-debs.tar.xz"
		echo "$graphviz_sha256  $tarball" | sha256sum -c -
		debs="$dest/graphviz-debs"
		rm -rf "$debs" "$graphviz"
		mkdir -p "$debs" "$graphviz/root" "$graphviz/bin"
		# Only the executables and their libraries; the language bindings,
		# headers and documentation stay packed.
		tar -xJf "$tarball" -C "$debs" "graphviz_${GRAPHVIZ_VERSION}-1_amd64.deb" "libgraphviz4_${GRAPHVIZ_VERSION}-1_amd64.deb"
		for deb in "$debs/graphviz_${GRAPHVIZ_VERSION}-1_amd64.deb" "$debs/libgraphviz4_${GRAPHVIZ_VERSION}-1_amd64.deb"; do
			if command -v dpkg-deb >/dev/null 2>&1; then
				dpkg-deb -x "$deb" "$graphviz/root"
			else
				(cd "$debs" && ar x "$deb" data.tar.xz && tar -xJf data.tar.xz -C "$graphviz/root" && rm -f data.tar.xz)
			fi
		done
		rm -rf "$debs" "$tarball"
		# The packages expect /usr; a wrapper points the unpacked copy at its
		# own libraries and plugins instead.
		cat >"$graphviz/bin/dot" <<'SH'
#!/usr/bin/env bash
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../root" && pwd)"
export LD_LIBRARY_PATH="$root/usr/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export GVBINDIR="$root/usr/lib/graphviz"
exec "$root/usr/bin/dot" "$@"
SH
		chmod 0755 "$graphviz/bin/dot"
		# The renderers that need libraries the package does not carry (DevIL,
		# Ghostscript, LASi, Poppler) are dropped, then the plugin table the
		# package's install step would have written is written here.
		rm -f "$graphviz"/root/usr/lib/graphviz/libgvplugin_{devil,gs,lasi,poppler}.so*
		"$graphviz/bin/dot" -c
		if ! "$graphviz/bin/dot" -V 2>&1 | grep -q "graphviz version $GRAPHVIZ_VERSION"; then
			echo "the unpacked Graphviz does not run: $("$graphviz/bin/dot" -V 2>&1)" >&2
			exit 1
		fi
	fi
fi

plantuml="$dest/plantuml/plantuml-$PLANTUML_VERSION.jar"
if [[ -s "$plantuml" ]]; then
	echo "PlantUML $PLANTUML_VERSION already present at $plantuml"
else
	echo "Fetching PlantUML $PLANTUML_VERSION ..."
	mkdir -p "$(dirname "$plantuml")"
	curl -fsSL --proto '=https' --proto-redir '=https' -o "$plantuml.part" \
		"https://github.com/plantuml/plantuml/releases/download/v$PLANTUML_VERSION/plantuml-$PLANTUML_VERSION.jar"
	echo "$PLANTUML_SHA256  $plantuml.part" | sha256sum -c -
	mv "$plantuml.part" "$plantuml"
fi

echo
echo "Done. Point the sysml binary at the pinned copies:"
echo "  export OPENSYSML_PANDOC=$pandoc_dir/bin/pandoc"
echo "  export OPENSYSML_WEASYPRINT=$venv/bin/weasyprint"
echo "  export OPENSYSML_MMDC=$mermaid/node_modules/.bin/mmdc"
echo "  export OPENSYSML_MMDC_PUPPETEER=$mermaid/puppeteer.json"
echo "  export OPENSYSML_KATEX=$katex/node_modules/.bin/katex"
if [[ -n "$graphviz_note" ]]; then
	echo "  # $graphviz_note"
else
	echo "  export OPENSYSML_DOT=$graphviz/bin/dot"
fi
echo "  export OPENSYSML_PLANTUML_JAR=$plantuml"
if command -v java >/dev/null 2>&1; then
	echo "  # java is on PATH ($(command -v java)); set OPENSYSML_JAVA to use another."
else
	echo "  # java is not on PATH: install a JRE and point OPENSYSML_JAVA at it to draw PlantUML diagrams."
fi
