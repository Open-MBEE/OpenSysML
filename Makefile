.PHONY: all build build-sysml build-lsp build-grpc windows-versioninfo-check man man-check install-tree pgo-profile conformance conformance-pkg conformance-rust test coverage lint clean install help python-test python-coverage scripts-coverage node-coverage python-install proto proto-buf python-proto proto-ts proto-rust proto-lint proto-breaking vscode-grammar vscode-build vscode-package docs docs-install docs-serve docs-counts docs-check changelog-check changelog-render self-model

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION ?= $(shell go version | awk '{print $$3}')

# Build flags
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.Commit=$(COMMIT) \
           -X main.BuildTime=$(BUILD_TIME) \
           -X main.GoVersion=$(GO_VERSION)

# Static-analysis tool versions, pinned so CI and local runs agree
STATICCHECK_VERSION := 2025.1.1
GOSEC_VERSION := v2.22.5
BUF_VERSION := v1.57.2
GO_WINRES_VERSION := v0.3.3

# buf drives all protobuf codegen; override BUF to use an already-installed binary.
BUF ?= go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
# Wire-compatibility baseline: the schema as it stands on the develop branch.
BUF_BREAKING_REF ?= origin/develop

# go-winres embeds a VERSIONINFO resource into the Windows binaries (a build
# tool only; nothing of it ships). The .syso it writes carries a _windows_amd64
# suffix, so the Go toolchain ignores it on every other GOOS.
GO_WINRES ?= go run github.com/tc-hib/go-winres@$(GO_WINRES_VERSION)
TARGET_GOOS := $(or $(GOOS),$(shell go env GOOS))
TARGET_GOARCH := $(or $(GOARCH),$(shell go env GOARCH))
WINRES_DIR := packaging/windows
# $(call winres,<cmd>): emit cmd/<cmd>/rsrc_windows_<arch>.syso stamped with VERSION for Windows targets; no-op otherwise.
# go-winres runs on the host, so the cross-compile GOOS/GOARCH are cleared for it.
define winres
$(if $(filter windows,$(TARGET_GOOS)),GOOS= GOARCH= $(GO_WINRES) make --in $(WINRES_DIR)/$(1).winres.json --arch $(TARGET_GOARCH) --out cmd/$(1)/rsrc --product-version "$(VERSION)" --file-version "$(VERSION)")
endef

# Build output directory
BIN_DIR := bin
PYTHON_DIR := clients/python
NODE_DIR := clients/node
# The TypeScript protobuf plugin, installed by `npm ci` from the client's lockfile.
PROTOC_GEN_ES := $(NODE_DIR)/node_modules/.bin/protoc-gen-es
VSCODE_DIR := editors/vscode
PYTHON ?= python3
# buf.gen.python.yaml starts the interpreter this names.
export PYTHON
SITE_DIR := site
# Where make self-model writes the architecture self-model's rendered views.
SELF_MODEL_DIR := examples/self-model
SELF_MODEL_OUT ?= build/self-model
# Where the commands the Go tests build and run write their coverage counters.
GO_COUNTER_DIR := $(CURDIR)/build/gocoverdir
LIBS_DIR := internal/core/libs

# The commands whose manual pages are generated and shipped, in section 1.
COMMANDS := sysml sysml-lsp sysml-grpc
MAN_DIR := man/man1
MAN_PAGES := $(addprefix $(MAN_DIR)/,$(addsuffix .1,$(COMMANDS)))

# Installation paths, as a distribution's packaging expects to set them.
DESTDIR ?=
prefix ?= /usr/local
exec_prefix ?= $(prefix)
bindir ?= $(exec_prefix)/bin
datarootdir ?= $(prefix)/share
mandir ?= $(datarootdir)/man
man1dir ?= $(mandir)/man1
INSTALL ?= install

all: build test python-test ## Build and test everything

build: build-sysml build-lsp build-grpc ## Build all binaries

build-sysml: ## Build sysml binary
	@echo "Building sysml..."
	@mkdir -p $(BIN_DIR)
	$(call winres,sysml)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sysml ./cmd/sysml
	@echo "✓ Built $(BIN_DIR)/sysml ($(VERSION))"

build-lsp: ## Build sysml-lsp binary
	@echo "Building sysml-lsp..."
	@mkdir -p $(BIN_DIR)
	$(call winres,sysml-lsp)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sysml-lsp ./cmd/sysml-lsp
	@echo "✓ Built $(BIN_DIR)/sysml-lsp ($(VERSION))"

build-grpc: ## Build sysml-grpc binary
	@echo "Building sysml-grpc..."
	@mkdir -p $(BIN_DIR)
	$(call winres,sysml-grpc)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sysml-grpc ./cmd/sysml-grpc
	@echo "✓ Built $(BIN_DIR)/sysml-grpc ($(VERSION))"

windows-versioninfo-check: ## Check a Windows binary's VERSIONINFO carries VERSION (EXE=path/to/file.exe)
	@test -n "$(EXE)" || { echo "Error: set EXE=path/to/file.exe"; exit 1; }
	GOOS= GOARCH= GO_WINRES="$(GO_WINRES)" scripts/check-windows-versioninfo.sh "$(EXE)" "$(VERSION)"

man: ## Regenerate the shipped manual pages from each command's description
	@echo "Writing the manual pages..."
	@mkdir -p $(MAN_DIR)
	@for cmd in $(COMMANDS); do \
		go run ./cmd/$$cmd -man > $(MAN_DIR)/$$cmd.1 || exit 1; \
	done
	@echo "✓ Wrote $(MAN_PAGES)"

man-check: ## Verify the shipped pages are current and formatter-clean
	@echo "Checking the manual pages..."
	go test -count=1 -run 'TestTheShippedManualPage|TestTheManualPage' ./cmd/sysml ./cmd/sysml-lsp ./cmd/sysml-grpc
	@# mandoc is the strictest reader; groff is the one always at hand.
	@if command -v mandoc >/dev/null 2>&1; then \
		mandoc -T lint -W warning $(MAN_PAGES) || exit 1; \
	elif command -v groff >/dev/null 2>&1; then \
		groff -man -Tutf8 -ww -z $(MAN_PAGES) || exit 1; \
	else \
		echo "note: neither mandoc nor groff is installed; the pages were not formatted"; \
	fi
	@echo "✓ Manual pages are current"

pgo-profile: ## Regenerate cmd/*/default.pgo, the CPU profile go build optimizes the binaries against
	@echo "Collecting the PGO profile..."
	scripts/pgo-profile.sh
	@echo "✓ Regenerated cmd/*/default.pgo"

conformance: ## Run the language-independent conformance suite against sysml-grpc
	@echo "Running the conformance suite..."
	@mkdir -p $(BIN_DIR)
	go run ./cmd/conformance -withhold-capabilities strict_conformance,oslc_query -report $(BIN_DIR)/conformance-report.json -junit $(BIN_DIR)/conformance-report.xml
	@echo "✓ Conformance suite passed ($(BIN_DIR)/conformance-report.json, $(BIN_DIR)/conformance-report.xml)"

conformance-rust: ## Run the conformance suite with the blocking Rust client
	$(MAKE) build
	@mkdir -p $(BIN_DIR)
	OPENSYSML_GRPC_BINARY="$(CURDIR)/$(BIN_DIR)/sysml-grpc" cargo run --manifest-path clients/rust/Cargo.toml -p opensysml-conformance -- -binary "$(CURDIR)/$(BIN_DIR)/sysml-grpc" -report "$(CURDIR)/$(BIN_DIR)/conformance-report-rust.json"

conformance-pkg: ## Run the conformance suite through the public Go API (client/opensysml)
	@echo "Running the conformance suite through client/opensysml..."
	@mkdir -p $(BIN_DIR)
	go run ./cmd/conformance -protocols pkg,pkg-connect -allow-skips -report $(BIN_DIR)/conformance-pkg-report.json
	@echo "✓ Conformance suite passed through client/opensysml ($(BIN_DIR)/conformance-pkg-report.json)"

test: ## Run Go tests with race detection and coverage
	@echo "Running Go race tests..."
	@# Per-package timeout: under -race, passes and model run within 1% of go's 10m default.
	@# -pgo=off: coverage plus cmd/*/default.pgo trips golang/go#80891 (link: fingerprint mismatch).
	go test -v -race -pgo=off -timeout 30m -coverprofile=coverage.txt -covermode=atomic ./...

coverage: ## Write the coverage profile the SonarCloud scan reads
	@echo "Writing coverage.txt..."
	@# -coverpkg credits a package for the code it exercises elsewhere: without it
	@# ast/dump.go measures 21% though the parser's golden tests run 90% of it.
	@# Instrumenting every package is too slow to combine with -race, which
	@# make test above runs instead. -pgo=off as in make test.
	@# -count=1: a replayed result carries zero blocks for the -coverpkg packages it does
	@# not link, keyed to the sources of its own run, so they go stale as those change.
	@# Tests that run a built command (internal/testutil/gobuild) instrument it and point
	@# it at this directory; go test folds in only its own binary's counters.
	rm -rf $(GO_COUNTER_DIR)
	mkdir -p $(GO_COUNTER_DIR)
	OPENSYSML_GOCOVERDIR=$(GO_COUNTER_DIR) go test -count=1 -pgo=off -timeout 30m -coverpkg=./... -coverprofile=coverage.txt -covermode=atomic ./...
	go tool covdata textfmt -i=$(GO_COUNTER_DIR) -o $(GO_COUNTER_DIR)/profile.txt
	tail -n +2 $(GO_COUNTER_DIR)/profile.txt >> coverage.txt
	@# -coverpkg repeats every block once per test binary; see the script's header.
	python3 scripts/dedupe-coverage.py coverage.txt
	@go tool cover -func=coverage.txt | tail -n 1

lint: ## Run static analysis (staticcheck + gosec), as CI does
	@echo "Running staticcheck..."
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...
	@echo "Running gosec..."
	@# Generated protobuf code is excluded: its unsafe.Pointer use (G103) comes
	@# from protoc-gen-go and is not ours to change.
	go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) -quiet -exclude-generated ./...
	@echo "✓ Lint passed"

test-short: ## Run Go tests without race detection
	@echo "Running Go tests without race detection..."
	go test -v ./...

stdlib-snapshot: ## Regenerate the embedded snapshot of the bundled library after editing $(LIBS_DIR)/stdlib
	go generate ./$(LIBS_DIR)

stdlib-snapshot-check: ## Verify the committed library snapshot matches the bundled library, as CI does
	go run ./$(LIBS_DIR)/gensnapshot -check -out $(LIBS_DIR)/stdlib.snapshot
	@echo "✓ stdlib.snapshot is current"

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.txt coverage-python.xml coverage-scripts.xml .coverage-scripts coverage-node.lcov
	rm -rf $(GO_COUNTER_DIR)
	rm -f sysml sysml-lsp sysml-grpc
	rm -f cmd/*/rsrc_windows_*.syso
	rm -rf $(SITE_DIR)
	@# Only the default destination; an overridden SELF_MODEL_OUT is the caller's.
	rm -rf build/self-model
	@echo "✓ Cleaned"

install: build ## Install binaries to $GOPATH/bin
	@echo "Installing to $(shell go env GOPATH)/bin..."
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml-lsp
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml-grpc
	@echo "✓ Installed"

# What a distribution's package build calls: staged under DESTDIR, into the
# GNU-conventional paths, binaries and manual pages together.
install-tree: build ## Install binaries and manual pages under DESTDIR/prefix
	@echo "Installing to $(DESTDIR)$(prefix)..."
	$(INSTALL) -d "$(DESTDIR)$(bindir)" "$(DESTDIR)$(man1dir)"
	@for cmd in $(COMMANDS); do \
		$(INSTALL) -m 0755 "$(BIN_DIR)/$$cmd" "$(DESTDIR)$(bindir)/$$cmd" || exit 1; \
		$(INSTALL) -m 0644 "$(MAN_DIR)/$$cmd.1" "$(DESTDIR)$(man1dir)/$$cmd.1" || exit 1; \
	done
	@echo "✓ Installed into $(DESTDIR)$(bindir) and $(DESTDIR)$(man1dir)"

version: ## Show version information
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Go version: $(GO_VERSION)"

proto: proto-buf python-proto proto-ts proto-rust ## Regenerate all protobuf stubs

# One template, so the Go stubs and the Java client's message classes cannot drift apart.
# The Java plugin is a remote one, so this needs the Buf Schema Registry.
proto-buf: ## Regenerate the Go and Java protobuf stubs
	@echo "Regenerating Go and Java protobuf stubs..."
	$(BUF) generate
	@echo "✓ Regenerated Go and Java stubs"

python-proto: ## Regenerate Python protobuf stubs
	@echo "Regenerating Python protobuf stubs..."
	@$(PYTHON) -c "import grpc_tools.protoc" >/dev/null 2>&1 || { echo "Error: grpcio-tools not installed. Run: $(PYTHON) -m pip install grpcio-tools"; exit 1; }
	$(BUF) generate --template buf.gen.python.yaml
	@echo "✓ Regenerated Python stubs"

proto-ts: $(PROTOC_GEN_ES) ## Regenerate the TypeScript stubs the npm client in clients/node ships
	@echo "Regenerating TypeScript protobuf stubs..."
	$(BUF) generate --template buf.gen.ts.yaml
	@echo "✓ Regenerated TypeScript stubs"

$(PROTOC_GEN_ES): $(NODE_DIR)/package-lock.json
	cd $(NODE_DIR) && npm ci --ignore-scripts

proto-rust: ## Generate Rust stubs and the descriptor for the Rust clients
	$(BUF) generate --template buf.gen.rust.yaml
	$(BUF) build -o clients/rust/conformance/sysml.descriptor.binpb

proto-lint: ## Lint the protobuf schema
	$(BUF) lint
	@echo "✓ Proto lint passed"

proto-breaking: ## Check the protobuf schema for wire-breaking changes against develop
	@# An archive, not the .git directory: buf would clone that, which a blobless (CI) checkout cannot serve.
	baseline=$$(mktemp -t proto-baseline.XXXXXX) && trap 'rm -f "$$baseline"' EXIT && \
	git archive --format=tar -o "$$baseline" '$(BUF_BREAKING_REF)' api/proto && \
	$(BUF) breaking --against "$$baseline#format=tar,subdir=api/proto"
	@echo "✓ No breaking schema changes"

python-install: ## Install the Python client in editable mode
	@echo "Installing opensysml..."
	cd $(PYTHON_DIR) && pip install -e .
	@echo "✓ Installed opensysml"

python-test: ## Run Python client tests
	@echo "Running Python client tests..."
	cd $(PYTHON_DIR) && pytest tests/ -v
	@echo "✓ Python client tests passed"

# Run from the repo root so the report records repo-relative paths, which is
# what the SonarCloud scan resolves against.
python-coverage: ## Run Python client tests and write coverage-python.xml
	@echo "Running Python client tests with coverage..."
	pytest $(PYTHON_DIR)/tests --cov=opensysml --cov-report=xml:coverage-python.xml --cov-report=term
	@echo "✓ Wrote coverage-python.xml"

# The repository scripts the checks run, measured the same way. Each script runs
# the way CI runs it, so the report credits what the checks execute. The release
# scripts under clients/python/scripts are loaded by path, so their tests run here too.
SCRIPTS_COVERAGE := $(PYTHON) -m coverage run --append --rcfile=scripts/coverage-scripts.ini

scripts-coverage: ## Run the repository scripts and their tests under coverage and write coverage-scripts.xml
	@echo "Running the repository scripts with coverage..."
	$(PYTHON) -m coverage erase --rcfile=scripts/coverage-scripts.ini
	$(SCRIPTS_COVERAGE) scripts/changelog-test.py
	$(SCRIPTS_COVERAGE) scripts/changelog.py check
	$(SCRIPTS_COVERAGE) scripts/mkdocs_census-test.py
	$(SCRIPTS_COVERAGE) scripts/dedupe-coverage-test.py
	$(SCRIPTS_COVERAGE) scripts/check-doc-links.py
	$(SCRIPTS_COVERAGE) scripts/check-doc-ids.py
	$(SCRIPTS_COVERAGE) scripts/check-doc-figures.py
	$(SCRIPTS_COVERAGE) scripts/sync-release-digests.py --check
	$(SCRIPTS_COVERAGE) -m pytest -q $(PYTHON_DIR)/tests/test_check_version.py $(PYTHON_DIR)/tests/test_pin_release_checksums.py
	$(PYTHON) -m coverage xml --rcfile=scripts/coverage-scripts.ini
	$(PYTHON) -m coverage report --rcfile=scripts/coverage-scripts.ini
	@echo "✓ Wrote coverage-scripts.xml"

# c8 records paths relative to the client directory, so rewrite them to
# repo-relative before the scan reads the report.
node-coverage: ## Run Node client tests and write coverage-node.lcov
	@echo "Running Node client tests with coverage..."
	cd $(NODE_DIR) && npm run test:coverage
	sed -e 's|^SF:|SF:$(NODE_DIR)/|' $(NODE_DIR)/coverage/lcov.info > coverage-node.lcov
	@echo "✓ Wrote coverage-node.lcov"

vscode-grammar: ## Regenerate the VS Code TextMate grammars from the lexer keywords
	@echo "Generating TextMate grammars..."
	go run ./$(VSCODE_DIR)/tools/gengrammar -out $(VSCODE_DIR)/syntaxes
	@echo "✓ Grammars generated"

vscode-build: ## Type-check and bundle the VS Code extension
	@echo "Building the VS Code extension..."
	cd $(VSCODE_DIR) && npm ci && npm run typecheck && npm run build
	@echo "✓ Built $(VSCODE_DIR)/dist/extension.js"

vscode-package: ## Package the VS Code extension as a .vsix for side-loading
	@echo "Packaging the VS Code extension..."
	cd $(VSCODE_DIR) && npm ci && npm run package
	@echo "✓ Packaged $(VSCODE_DIR)/opensysml-sysml.vsix"

self-model: build-sysml ## Render the architecture self-model's views (see examples/self-model/README.md)
	@echo "Rendering the architecture self-model..."
	@mkdir -p "$(SELF_MODEL_OUT)"
	@# A renamed or deleted view or document must not leave its old rendering behind.
	@rm -f "$(SELF_MODEL_OUT)"/OpenSysMLViews.*.mmd "$(SELF_MODEL_OUT)"/OpenSysMLViews.*.md \
		"$(SELF_MODEL_OUT)"/OpenSysMLDocument-*.md
	$(BIN_DIR)/sysml $(SELF_MODEL_DIR)/*.sysml -render-all "$(SELF_MODEL_OUT)"
	@# The architecture document the model declares, rendered by the same model.
	$(BIN_DIR)/sysml $(SELF_MODEL_DIR)/*.sysml -render-documents "$(SELF_MODEL_OUT)"
	@echo "✓ Rendered the self-model's views and document into $(SELF_MODEL_OUT)/"

docs-counts: ## Regenerate and verify all derived documentation counts
	@echo "Regenerating the documentation count lines and refereed figures..."
	go run ./cmd/doc-counts
	go run ./cmd/doc-counts -check
	go run ./cmd/validation-census -check
	go test -count=1 ./cmd/pilot-diff ./cmd/pilot-reject ./cmd/doc-counts ./cmd/validation-census
	@echo "✓ Documentation counts and refereed figures are current"

docs-check: ## Verify documentation links, internal-label hygiene, quoted oracle figures, changelog fragments and the build-time compliance census
	$(PYTHON) scripts/check-doc-links.py
	$(PYTHON) scripts/check-doc-ids.py
	$(PYTHON) scripts/check-doc-figures.py
	$(PYTHON) scripts/changelog.py check
	$(PYTHON) scripts/mkdocs_census-test.py

changelog-check: ## Verify every changelog fragment under changes/unreleased/ and the folding script
	$(PYTHON) scripts/changelog-test.py
	$(PYTHON) scripts/changelog.py check

changelog-render: ## Fold changes/unreleased/ fragments into the Unreleased section of CHANGELOG.md
	$(PYTHON) scripts/changelog.py render

docs-install: ## Install the documentation site toolchain
	$(PYTHON) -m pip install -r docs-requirements.txt

docs: ## Build the documentation site, failing on a broken link
	@echo "Building the documentation site..."
	$(PYTHON) -m mkdocs build --strict --site-dir $(SITE_DIR)
	@echo "✓ Built $(SITE_DIR)/"

docs-serve: ## Serve the documentation site with live reload
	$(PYTHON) -m mkdocs serve --strict

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
