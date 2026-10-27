.PHONY: all build docker vulncheck sca-tools audit audit-npm audit-python audit-rust audit-java extension-libraries-check extension-libraries-sync build-sysml build-re build-wasm-re build-lsp build-grpc build-mcp build-engine build-core build-syntax build-sysml-wasm build-release-wasm build-wasm build-wasm-wasip1 build-wasm-js wasm-check static-check windows-versioninfo-check man man-check install-tree pgo-profile conformance conformance-pkg conformance-mcp conformance-rust conformance-julia conformance-matlab test test-shard coverage coverage-shard coverage-merge lint clean install help ontology-table ontology-table-check metaclass-graph metaclass-graph-check python-metamodel python-metamodel-check fuml-expected python-test python-coverage scripts-coverage node-coverage vscode-coverage jupyterlab-coverage mcp-npm-coverage python-install jupyter-kernel-install jupyter-kernel-test proto proto-buf python-proto proto-ts proto-rust proto-lint proto-breaking jupyterlab-build jupyterlab-test jupyterlab-install vscode-grammar vscode-build vscode-package docs docs-install docs-serve docs-engine-assets docs-landing-assets docs-counts diagnostics-catalogue diagnostics-check docs-check changelog-check changelog-render self-model mcp-registry-check

# Version information
# Only release tags describe a build; the moving `nightly` tag is not a version.
VERSION ?= $(shell git describe --tags --always --dirty --match 'v[0-9]*' 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION ?= $(shell go version | awk '{print $$3}')

# Build flags
# -s -w drop the symbol table and DWARF; version stamps, build info and stack traces stay.
LDFLAGS := -s -w \
           -X main.Version=$(VERSION) \
           -X main.Commit=$(COMMIT) \
           -X main.BuildTime=$(BUILD_TIME) \
           -X main.GoVersion=$(GO_VERSION)
# Static binaries: a host-linked libc would pin a build to the builder's glibc.
# Only Go's net resolver used cgo, so the pure-Go one serves instead.
GO_BUILD := CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)"
GO_INSTALL := CGO_ENABLED=0 go install -ldflags "$(LDFLAGS)"

# Static-analysis tool versions, pinned so CI and local runs agree
STATICCHECK_VERSION := 2025.1.1
# Unbounded, staticcheck's heap on this module peaks near 4 GiB, which the 4 GiB CI
# executor kills it for; the soft limit holds the peak near 2 GiB for ~10% more GC time.
STATICCHECK_ENV := GOMEMLIMIT=2GiB
GOSEC_VERSION := v2.22.5
# Dependency-audit tool versions: govulncheck runs through `go run`; the others come
# from scripts/download-sca-tools.sh, pinned and SHA-256 verified there.
GOVULNCHECK_VERSION := v1.9.0
CYCLONEDX_MAVEN_VERSION := 2.9.1
SCA_BIN := build/sca/bin
PIP_AUDIT := build/sca/venv/bin/pip-audit
# `npm audit` fails on production advisories at this severity and above, in every lockfile directory.
NPM_AUDIT_LEVEL := moderate
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
# WebAssembly output, one directory per Go wasm target.
WASM_DIR := $(BIN_DIR)/wasm
PYTHON_DIR := client/python
JUPYTER_KERNEL_DIR := client/jupyter-kernel
NODE_DIR := client/node
# The TypeScript protobuf plugin, installed from the client's lockfile.
PROTOC_GEN_ES := $(NODE_DIR)/node_modules/.bin/protoc-gen-es
VSCODE_DIR := editors/vscode
JUPYTERLAB_DIR := editors/jupyterlab
MCP_NPM_DIR := packaging/npm/sysml-mcp
NPM_LOCK_DIRS := $(NODE_DIR) $(MCP_NPM_DIR) $(VSCODE_DIR) $(JUPYTERLAB_DIR) editors/syson/frontend scripts/asciidoc
PYTHON_PROJECT_DIRS := $(PYTHON_DIR) $(JUPYTER_KERNEL_DIR) packaging/pypi-pysysml
PYTHON_REQUIREMENTS := docs-requirements.txt
JAVA_PROJECT_DIRS := client/java editors/syson editors/mdk
PYTHON ?= python3
# api/proto/buf.gen.python.yaml starts the interpreter this names.
export PYTHON
JULIA ?= julia
OCTAVE ?= octave
SITE_DIR := site
# Where make self-model writes the architecture self-model's rendered views.
SELF_MODEL_DIR := examples/self-model
SELF_MODEL_OUT ?= build/self-model
# Where the commands the Go tests build and run write their coverage counters.
GO_COUNTER_DIR := $(CURDIR)/build/gocoverdir
LIBS_DIR := internal/workspace/libs
# The development tools are a nested module; go's ./... at the root stops at
# its go.mod, so every whole-tree target runs go a second time in it.
TOOLS_DIR := tools
# make coverage's scope; coverage-shard narrows it to one CI shard.
COVERAGE_PACKAGES ?= ./...
COVERAGE_TOOLS ?= ./...
COVERAGE_PROFILE ?= coverage.txt
# The coverage job's nodes: the race shards except runtime-corpus, since without -race
# internal/exec/runtime runs whole in about 19 minutes under -coverpkg (see .circleci/config.yml).
COVERAGE_SHARDS := runtime model export graph codegen tools rest migrate cli

# The commands whose manual pages are generated and shipped, in section 1.
COMMANDS := sysml sysml-lsp sysml-grpc sysml-jupyter-kernel sysml-mcp
# sysml-engine, sysml-syntax, sysml-core and sysml-wasm serve JSON RPC surfaces
# without protobuf. Natively they are built only on request and stay out of
# `build`, `install`, the release and the manual pages.
WASM_COMMANDS := $(COMMANDS) sysml-engine sysml-syntax sysml-core sysml-wasm
MAN_DIR := packaging/man/man1
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

build: build-sysml build-lsp build-grpc build-jupyter-kernel build-mcp ## Build all binaries

build-sysml: ## Build bin/sysml, the DK build with every feature group
	@echo "Building sysml (DK)..."
	@mkdir -p $(BIN_DIR)
	$(call winres,sysml)
	$(GO_BUILD) -o $(BIN_DIR)/sysml ./cmd/sysml
	@echo "✓ Built $(BIN_DIR)/sysml ($(VERSION))"

# The RE (Runtime Environment) sysml leaves out the optional and developer-only
# feature groups (sysml_re; see cmd/sysml/features.go); the DK (Development Kit)
# build is the default one. Opt-in: nothing builds the RE by default.
build-re: ## Build bin/re/sysml, the RE build without the optional features (-tags sysml_re)
	@echo "Building sysml (RE)..."
	@mkdir -p $(BIN_DIR)/re
	$(call winres,sysml)
	$(GO_BUILD) -tags sysml_re -o $(BIN_DIR)/re/sysml ./cmd/sysml
	@echo "✓ Built $(BIN_DIR)/re/sysml ($(VERSION))"

build-lsp: ## Build sysml-lsp binary
	@echo "Building sysml-lsp..."
	@mkdir -p $(BIN_DIR)
	$(call winres,sysml-lsp)
	$(GO_BUILD) -o $(BIN_DIR)/sysml-lsp ./cmd/sysml-lsp
	@echo "✓ Built $(BIN_DIR)/sysml-lsp ($(VERSION))"

build-grpc: ## Build sysml-grpc binary
	@echo "Building sysml-grpc..."
	@mkdir -p $(BIN_DIR)
	$(call winres,sysml-grpc)
	$(GO_BUILD) -o $(BIN_DIR)/sysml-grpc ./cmd/sysml-grpc
	@echo "✓ Built $(BIN_DIR)/sysml-grpc ($(VERSION))"

# Container image: sysml-grpc as the entrypoint with sysml and sysml-lsp beside
# it, built by the Dockerfile with the same -ldflags metadata as the binaries.
IMAGE ?= openmbee/opensysml
IMAGE_TAG ?= dev

docker: ## Build the $(IMAGE):$(IMAGE_TAG) container image for the local platform
	@echo "Building $(IMAGE):$(IMAGE_TAG)..."
	DOCKER_BUILDKIT=1 docker build \
	    --build-arg VERSION="$(VERSION)" \
	    --build-arg COMMIT="$(COMMIT)" \
	    --build-arg BUILD_TIME="$(BUILD_TIME)" \
	    -t $(IMAGE):$(IMAGE_TAG) .
	@echo "✓ Built $(IMAGE):$(IMAGE_TAG) ($(VERSION))"

build-jupyter-kernel: ## Build sysml-jupyter-kernel binary
	@echo "Building sysml-jupyter-kernel..."
	@mkdir -p $(BIN_DIR)
	$(call winres,sysml-jupyter-kernel)
	$(GO_BUILD) -o $(BIN_DIR)/sysml-jupyter-kernel ./cmd/sysml-jupyter-kernel
	@echo "✓ Built $(BIN_DIR)/sysml-jupyter-kernel ($(VERSION))"

build-mcp: ## Build sysml-mcp binary
	@echo "Building sysml-mcp..."
	@mkdir -p $(BIN_DIR)
	$(call winres,sysml-mcp)
	$(GO_BUILD) -o $(BIN_DIR)/sysml-mcp ./cmd/sysml-mcp
	@echo "✓ Built $(BIN_DIR)/sysml-mcp ($(VERSION))"

# The JSON commands, natively: each serves its JSON-RPC over stdio. Opt-in:
# nothing builds, installs or releases them.
build-engine: ## Build bin/sysml-engine natively (opt-in; not released)
	@echo "Building sysml-engine..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR)/sysml-engine ./cmd/sysml-engine
	@echo "✓ Built $(BIN_DIR)/sysml-engine ($(VERSION))"

build-core: ## Build bin/sysml-core natively (opt-in; not released)
	@echo "Building sysml-core..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR)/sysml-core ./cmd/sysml-core
	@echo "✓ Built $(BIN_DIR)/sysml-core ($(VERSION))"

build-sysml-wasm: ## Build bin/sysml-wasm natively (opt-in; not released)
	@echo "Building sysml-wasm..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR)/sysml-wasm ./cmd/sysml-wasm
	@echo "✓ Built $(BIN_DIR)/sysml-wasm ($(VERSION))"

build-release-wasm: ## Build release bin/wasm/release/{sysml-wasm.wasm,sysml-prod.wasm.gz,wasm_exec.js}
	@echo "Building release WebAssembly assets..."
	@mkdir -p $(WASM_DIR)/release
	GOOS=js GOARCH=wasm $(GO_BUILD) -trimpath -o $(WASM_DIR)/release/sysml-wasm.wasm ./cmd/sysml-wasm
	GOOS=js GOARCH=wasm $(GO_BUILD) -trimpath -tags sysml_prod -o $(WASM_DIR)/release/sysml-prod.wasm ./cmd/sysml
	gzip -9nf $(WASM_DIR)/release/sysml-prod.wasm
	@cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_DIR)/release/wasm_exec.js
	@echo "✓ Built $(WASM_DIR)/release/sysml-wasm.wasm, sysml-prod.wasm.gz and wasm_exec.js ($(VERSION))"

build-syntax: ## Build bin/sysml-syntax natively (opt-in; not released)
	@echo "Building sysml-syntax..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR)/sysml-syntax ./cmd/sysml-syntax
	@echo "✓ Built $(BIN_DIR)/sysml-syntax ($(VERSION))"

# WebAssembly: GOOS=wasip1 runs under a WASI preview 1 runtime (wasmtime, a Node WASI
# host), GOOS=js under Node or a browser through the toolchain's wasm_exec.js. The
# version stamps are the -X flags every other build passes; there is no Windows
# resource to embed and no libc to link. `build` stays native: these are opt-in.
build-wasm: build-wasm-wasip1 build-wasm-js ## Build all seven commands for both WebAssembly targets

build-wasm-wasip1: ## Build bin/wasm/wasip1/*.wasm, runnable under a WASI preview 1 runtime
	@echo "Building WebAssembly (wasip1)..."
	@mkdir -p $(WASM_DIR)/wasip1
	@for cmd in $(WASM_COMMANDS); do \
		GOOS=wasip1 GOARCH=wasm $(GO_BUILD) -o $(WASM_DIR)/wasip1/$$cmd.wasm ./cmd/$$cmd || exit 1; \
	done
	@echo "✓ Built $(WASM_DIR)/wasip1 ($(VERSION))"

build-wasm-js: ## Build bin/wasm/js/*.wasm plus the wasm_exec.js that runs them
	@echo "Building WebAssembly (js)..."
	@mkdir -p $(WASM_DIR)/js
	@for cmd in $(WASM_COMMANDS); do \
		GOOS=js GOARCH=wasm $(GO_BUILD) -o $(WASM_DIR)/js/$$cmd.wasm ./cmd/$$cmd || exit 1; \
	done
	@cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_DIR)/js/wasm_exec.js
	@echo "✓ Built $(WASM_DIR)/js ($(VERSION))"

build-wasm-re: ## Build bin/wasm/{js,wasip1}/re/sysml.wasm, the RE build, with -tags sysml_re
	@echo "Building WebAssembly sysml (RE)..."
	@mkdir -p $(WASM_DIR)/js/re $(WASM_DIR)/wasip1/re
	GOOS=wasip1 GOARCH=wasm $(GO_BUILD) -tags sysml_re -o $(WASM_DIR)/wasip1/re/sysml.wasm ./cmd/sysml
	GOOS=js GOARCH=wasm $(GO_BUILD) -tags sysml_re -o $(WASM_DIR)/js/re/sysml.wasm ./cmd/sysml
	@cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_DIR)/js/wasm_exec.js
	@echo "✓ Built $(WASM_DIR)/{js,wasip1}/re/sysml.wasm ($(VERSION))"

# Cold, the gate compiles and vets the whole tree for both targets: 7-11 minutes on a hosted CI
# runner, past go test's default 10m. 25m is about twice that, so a slow run fails on an assertion.
WASM_GATE_TIMEOUT ?= 25m

wasm-check: ## Run the WebAssembly build-and-run gate (needs Node; fails rather than skipping)
	OPENSYSML_REQUIRE_WASM=1 go test -count=1 -v -timeout $(WASM_GATE_TIMEOUT) ./tests/wasm

static-check: ## Check the built Linux binaries are statically linked (BINARIES=path...)
	scripts/check-static-binaries.sh $(or $(BINARIES),$(addprefix $(BIN_DIR)/,$(COMMANDS)))

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
	go test -count=1 -run 'TestTheShippedManualPage|TestTheManualPage' ./cmd/sysml ./cmd/sysml-lsp ./cmd/sysml-grpc ./cmd/sysml-jupyter-kernel ./cmd/sysml-mcp
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
	go run -C $(TOOLS_DIR) ./cmd/conformance -withhold-capabilities strict_conformance,oslc_query -report $(CURDIR)/$(BIN_DIR)/conformance-report.json -junit $(CURDIR)/$(BIN_DIR)/conformance-report.xml
	@echo "✓ Conformance suite passed ($(BIN_DIR)/conformance-report.json, $(BIN_DIR)/conformance-report.xml)"

conformance-rust: ## Run the conformance suite with the blocking Rust client
	$(MAKE) build
	@mkdir -p $(BIN_DIR)
	OPENSYSML_GRPC_BINARY="$(CURDIR)/$(BIN_DIR)/sysml-grpc" cargo run --manifest-path client/rust/Cargo.toml -p opensysml-conformance -- -binary "$(CURDIR)/$(BIN_DIR)/sysml-grpc" -report "$(CURDIR)/$(BIN_DIR)/conformance-report-rust.json"

conformance-julia: ## Run the conformance suite with the Julia client (JULIA=path overrides the binary)
	$(MAKE) build-grpc
	@mkdir -p $(BIN_DIR)
	$(JULIA) --project=client/julia/OpenSysML client/julia/OpenSysML/conformance/run.jl \
		--binary "$(CURDIR)/$(BIN_DIR)/sysml-grpc" \
		--report "$(CURDIR)/$(BIN_DIR)/conformance-report-julia.json"

conformance-matlab: ## Run the conformance suite with the MATLAB/Octave client (OCTAVE=path overrides the binary)
	$(MAKE) build-grpc
	@mkdir -p $(BIN_DIR)
	@addr_file="$(CURDIR)/$(BIN_DIR)/.conformance-matlab-addr"; rm -f "$$addr_file"; \
	"$(CURDIR)/$(BIN_DIR)/sysml-grpc" -port 0 -health-port 0 -report-address > "$$addr_file" & \
	svc_pid=$$!; \
	rc=0; \
	for i in $$(seq 1 100); do [ -s "$$addr_file" ] && break; sleep 0.1; done; \
	addr=$$(cat "$$addr_file" 2>/dev/null); \
	if [ -z "$$addr" ]; then rc=1; echo "sysml-grpc did not report an address"; \
	else \
		$(OCTAVE) --no-gui --eval "addpath('client/matlab'); addpath('client/matlab/conformance'); addpath('client/matlab/conformance/private'); run_conformance('--address','$$addr','--report','$(CURDIR)/$(BIN_DIR)/conformance-report-matlab.json')" || rc=$$?; \
	fi; \
	kill $$svc_pid 2>/dev/null; wait $$svc_pid 2>/dev/null; rm -f "$$addr_file"; exit $$rc

conformance-pkg: ## Run the conformance suite through the public Go API (client/opensysml)
	@echo "Running the conformance suite through client/opensysml..."
	@mkdir -p $(BIN_DIR)
	go run -C $(TOOLS_DIR) ./cmd/conformance -protocols pkg,pkg-connect -allow-skips -report $(CURDIR)/$(BIN_DIR)/conformance-pkg-report.json
	@echo "✓ Conformance suite passed through client/opensysml ($(BIN_DIR)/conformance-pkg-report.json)"

conformance-mcp: ## Run the conformance suite through sysml-mcp's runtime tools
	@echo "Running the conformance suite through sysml-mcp..."
	@mkdir -p $(BIN_DIR)
	go run -C $(TOOLS_DIR) ./cmd/conformance -protocols mcp -allow-skips -withhold-capabilities strict_conformance,oslc_query -report $(CURDIR)/$(BIN_DIR)/conformance-mcp-report.json -junit $(CURDIR)/$(BIN_DIR)/conformance-mcp-report.xml
	@echo "✓ Conformance suite passed through sysml-mcp ($(BIN_DIR)/conformance-mcp-report.json, $(BIN_DIR)/conformance-mcp-report.xml)"

test: ## Run Go tests with race detection and coverage
	@echo "Running Go race tests..."
	@# Per-package timeout: under -race the runtime package runs 31-47 minutes on CI runners.
	@# -pgo=off: coverage plus cmd/*/default.pgo trips golang/go#80891 (link: fingerprint mismatch).
	go test -v -race -pgo=off -timeout 45m -coverprofile=coverage.txt -covermode=atomic ./...
	go test -C $(TOOLS_DIR) -v -race -pgo=off -timeout 45m ./...

# Run race-free by their own gate steps in the PR workflow's static-and-integrity job.
RACE_SHARD_SKIP := ^(TestTrainingExamplesSemanticErrors|TestCorpusGatesCacheStateIndependent|TestPilotCorporaDiagnostics|TestPilotLibraryXMI|TestPSSMSuiteMigration|TestDifferentialRandomizedAssignments|TestDifferentialConformanceCorpus|TestDifferentialStandardLibrary|TestDifferentialTrainingCorpus|TestPortability|TestPortabilityGateIsRequired)$$
# The runtime-corpus shard: the suites that run every model of the conformance corpus,
# about half the package's time under -race.
RACE_RUNTIME_EXPLORATION := ^TestExploreWithIsExploreOverTheConformanceCorpus$$
RACE_RUNTIME_CORPUS := ^Test(ExploreWithIsExploreOverTheConformanceCorpus|CheckAgreesWithExploreOverTheConformanceCorpus|CheckWitnessesReplayOverTheConformanceCorpus|ExecutionConformance|ExecutionConformanceUnderPolicies)$$
RACE_SHARD_TOOLS_SKIP := ^(TestSuiteRead|TestSuiteClassification|TestEmitSuite|TestSuiteClassificationReasons|TestSuiteLibraryCallsAreClassified|TestSuiteReadsControlAndObjectFlow|TestSuiteReadsClassifiers|TestSuiteReadsExceptionHandlers)$$

test-shard: ## Run one CI shard of the race suite (SHARD=runtime|runtime-corpus|model|export|graph|codegen|tools|rest|migrate|cli)
	@echo "Running Go race tests, shard $(SHARD)..."
	@# 55m per package; its job's ceiling is 60. Whole, internal/exec/runtime took 31-57 minutes
	@# under -race on the CI runners, so it runs as two shards of about half that each.
	pkgs=$$(scripts/race-shard.sh $(SHARD)) && case "$(SHARD)" in \
	  runtime-corpus) go test -run '$(RACE_RUNTIME_EXPLORATION)' -skip '$(RACE_SHARD_SKIP)' -v -race -pgo=off -timeout 55m -coverprofile=coverage-exploration.txt -covermode=atomic $$pkgs && \
	    go test -run '$(RACE_RUNTIME_CORPUS)' -skip '$(RACE_SHARD_SKIP)|$(RACE_RUNTIME_EXPLORATION)' -v -race -pgo=off -timeout 55m -coverprofile=coverage.txt -covermode=atomic $$pkgs && \
	    tail -n +2 coverage-exploration.txt >> coverage.txt && python3 scripts/dedupe-coverage.py coverage.txt ;; \
	  runtime) go test -skip '$(RACE_SHARD_SKIP)|$(RACE_RUNTIME_CORPUS)' -v -race -pgo=off -timeout 55m -coverprofile=coverage.txt -covermode=atomic $$pkgs ;; \
	  *) go test -skip '$(RACE_SHARD_SKIP)' -v -race -pgo=off -timeout 55m -coverprofile=coverage.txt -covermode=atomic $$pkgs ;; \
	esac
	@# The tools module runs with tools (tests/wasm) on CircleCI and, as the PR workflow has no
	@# tools shard (tests/wasm is its WebAssembly gate job there), with rest on GitHub Actions.
	case "$(SHARD)" in rest|tools) go test -C $(TOOLS_DIR) -skip '$(RACE_SHARD_TOOLS_SKIP)' -v -race -pgo=off -timeout 55m ./... ;; esac

test-quick: ## Run the race-free quick tier over one or more race shards (SHARDS="runtime" | "model export graph" | "rest migrate cli codegen")
	@echo "Running Go tests without race detection, shards $(SHARDS)..."
	@# Draft pull requests run this instead of test-shard: the same packages and skips, no -race, no coverage.
	@test -n "$(SHARDS)" || { echo "Error: set SHARDS to one or more of runtime model export graph codegen tools rest migrate cli"; exit 1; }
	pkgs=$$(for shard in $(SHARDS); do scripts/race-shard.sh $$shard || exit 1; done) && \
	go test -skip '$(RACE_SHARD_SKIP)' -v -timeout 45m $$pkgs
	case " $(SHARDS) " in *" rest "*|*" tools "*) go test -C $(TOOLS_DIR) -skip '$(RACE_SHARD_TOOLS_SKIP)' -v -timeout 45m ./... ;; esac

coverage: ## Write the coverage profile the SonarCloud scan reads
	@echo "Writing coverage.txt..."
	@# -coverpkg credits a package for the code it exercises elsewhere: without it
	@# ast/dump.go measures 21% though the parser's golden tests run 90% of it.
	@# Instrumenting every package is too slow to combine with -race, which
	@# make test above runs instead. -pgo=off as in make test.
	@# -count=1: a replayed result carries zero blocks for the -coverpkg packages it does
	@# not link, keyed to the sources of its own run, so they go stale as those change.
	@# Tests that run a built command (tests/testutil/gobuild) instrument it and point
	@# it at this directory; go test folds in only its own binary's counters.
	rm -rf $(GO_COUNTER_DIR)
	mkdir -p $(GO_COUNTER_DIR)
	OPENSYSML_GOCOVERDIR=$(GO_COUNTER_DIR) go test -count=1 -pgo=off -timeout 30m -coverpkg=./... -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic $(COVERAGE_PACKAGES)
	go tool covdata textfmt -i=$(GO_COUNTER_DIR) -o $(GO_COUNTER_DIR)/profile.txt
	tail -n +2 $(GO_COUNTER_DIR)/profile.txt >> $(COVERAGE_PROFILE)
	@# The tools' tests exercise product packages too; their profile credits those. It is
	@# written under GO_COUNTER_DIR, where no shard's COVERAGE_PROFILE (coverage-tools.txt) can be.
	if [ -n "$(COVERAGE_TOOLS)" ]; then \
		go test -C $(TOOLS_DIR) -count=1 -pgo=off -timeout 30m -coverpkg=github.com/Open-MBEE/OpenSysML/... -coverprofile=$(GO_COUNTER_DIR)/tools.txt -covermode=atomic $(COVERAGE_TOOLS) && \
		tail -n +2 $(GO_COUNTER_DIR)/tools.txt >> $(COVERAGE_PROFILE); \
	fi
	@# -coverpkg repeats every block once per test binary; see the script's header.
	python3 scripts/dedupe-coverage.py $(COVERAGE_PROFILE)
	@go tool cover -func=$(COVERAGE_PROFILE) | tail -n 1

coverage-shard: ## Write one CI shard's coverage profile, coverage-$(SHARD).txt (SHARD=runtime|model|export|graph|codegen|tools|rest|migrate|cli)
	@# The race shards' packages; the tools module goes with tools (tests/wasm), as in test-shard.
	pkgs=$$(scripts/race-shard.sh $(SHARD)) && \
	$(MAKE) --no-print-directory coverage COVERAGE_PROFILE=coverage-$(SHARD).txt \
		COVERAGE_PACKAGES="$$(echo $$pkgs)" \
		COVERAGE_TOOLS="$(if $(filter tools,$(SHARD)),./...)"

coverage-merge: ## Merge the shard profiles coverage-<shard>.txt of every COVERAGE_SHARDS shard into coverage.txt
	{ echo "mode: atomic"; for shard in $(COVERAGE_SHARDS); do tail -n +2 coverage-$$shard.txt || exit 1; done; } > coverage.txt
	python3 scripts/dedupe-coverage.py coverage.txt
	@go tool cover -func=coverage.txt | tail -n 1

lint: ## Run static analysis (staticcheck + gosec), as CI does
	@echo "Running staticcheck..."
	$(STATICCHECK_ENV) go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...
	$(STATICCHECK_ENV) go run -C $(TOOLS_DIR) honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...
	@echo "Running gosec..."
	@# Generated protobuf code is excluded: its unsafe.Pointer use (G103) comes
	@# from protoc-gen-go and is not ours to change.
	module_go_version=$$(go list -m -f '{{.GoVersion}}') && GOSECGOVERSION=$$module_go_version go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) -quiet -exclude-generated ./...
	module_go_version=$$(go list -C $(TOOLS_DIR) -m -f '{{.GoVersion}}') && GOSECGOVERSION=$$module_go_version go run -C $(TOOLS_DIR) github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) -quiet -exclude-generated ./...
	@echo "✓ Lint passed"

vulncheck: ## Fail on known vulnerabilities reachable from the Go modules (govulncheck), as CI does
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
	go run -C $(TOOLS_DIR) golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

sca-tools: ## Provision the pinned cargo-audit, osv-scanner and pip-audit under build/sca
	./scripts/download-sca-tools.sh

audit-npm: ## Fail on $(NPM_AUDIT_LEVEL)+ advisories against the production dependencies of every npm lockfile (NPM_LOCK_DIRS=dir... narrows)
	@for dir in $(NPM_LOCK_DIRS); do echo "npm audit $$dir" && npm --prefix "$$dir" audit --omit=dev --audit-level=$(NPM_AUDIT_LEVEL) || exit 1; done

audit-python: ## Fail on known vulnerabilities in the Python packages' dependencies and the pinned requirements files (pip-audit --strict)
	@for dir in $(PYTHON_PROJECT_DIRS); do echo "pip-audit $$dir" && $(PIP_AUDIT) --strict --progress-spinner off "$$dir" || exit 1; done
	@for file in $(PYTHON_REQUIREMENTS); do echo "pip-audit -r $$file" && $(PIP_AUDIT) --strict --progress-spinner off -r "$$file" || exit 1; done

audit-rust: ## Fail on RustSec advisories against client/rust/Cargo.lock (cargo audit)
	$(SCA_BIN)/cargo-audit audit --file client/rust/Cargo.lock

audit-java: ## Fail on OSV advisories against the Maven trees' compile and runtime dependencies (JAVA_PROJECT_DIRS=dir... narrows)
	@# A CycloneDX SBOM carries the versions Maven resolved, reactor modules included, which osv-scanner's own pom resolver cannot follow.
	@for dir in $(JAVA_PROJECT_DIRS); do echo "osv-scanner $$dir" && mvn -B -q -f "$$dir/pom.xml" org.cyclonedx:cyclonedx-maven-plugin:$(CYCLONEDX_MAVEN_VERSION):makeAggregateBom -DoutputFormat=json -DoutputName=bom -DincludeTestScope=false && $(SCA_BIN)/osv-scanner scan source -L "$$dir/target/bom.json" || exit 1; done

audit: vulncheck audit-npm audit-python audit-rust audit-java ## Run every dependency audit, as CI and the release procedure do
	@echo "✓ Dependency audits passed"

test-short: ## Run Go tests without race detection
	@echo "Running Go tests without race detection..."
	go test -v ./...
	go test -C $(TOOLS_DIR) -v ./...

extension-libraries-check: ## Check the vendored extension libraries match their upstream pin
	./scripts/sync-extension-libraries.sh --check

extension-libraries-sync: ## Re-vendor the extension libraries from the pinned upstream commit
	./scripts/sync-extension-libraries.sh

stdlib-snapshot: ## Regenerate the embedded snapshot of the bundled library after editing $(LIBS_DIR)/stdlib
	go generate ./$(LIBS_DIR)

stdlib-snapshot-check: ## Verify the committed library snapshot matches the bundled library, as CI does
	go run -C $(TOOLS_DIR) ./gen/snapshot -check
	@echo "✓ stdlib.snapshot is current"

ontology-table: ## Regenerate the metamodel table from the pilot SysML.ecore that scripts/pilot-pin.sh pins
	./scripts/download-pilot-metamodel.sh
	go run -C $(TOOLS_DIR) ./gen/ontology
	@echo "✓ internal/translate/rdf/ontology/table.go regenerated"

ontology-table-check: ## Verify the committed metamodel table matches the pinned pilot SysML.ecore, as CI does
	./scripts/download-pilot-metamodel.sh
	go run -C $(TOOLS_DIR) ./gen/ontology -check
	@echo "✓ internal/translate/rdf/ontology/table.go is current"

metaclass-graph: build-sysml ## Regenerate the metaclass hierarchy Turtle from the committed metamodel table
	./bin/sysml -metaclass-graph -o internal/translate/rdf/ontology/sysml-metaclasses.ttl

metaclass-graph-check: ## Verify the committed metaclass hierarchy Turtle matches the metamodel table, as CI does
	go test -count=1 -run TestMetaclassGraphIsCurrent ./internal/translate/rdf/ontology
	@echo "✓ internal/translate/rdf/ontology/sysml-metaclasses.ttl is current"

python-metamodel: ## Regenerate Python metaclass classes from the ontology table
	go run -C $(TOOLS_DIR) ./gen/pymetamodel
	@echo "✓ Python metamodel generated"

python-metamodel-check: ## Verify generated Python metaclass classes are current
	go run -C $(TOOLS_DIR) ./gen/pymetamodel -check
	@echo "✓ Python metamodel is current"

fuml-expected: ## Regenerate docs/project/fuml-referee-expected.json from the pinned fUML reference implementation (needs a JDK)
	./scripts/fuml-expected.sh
	@echo "✓ fuml-referee-expected.json regenerated"

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.txt coverage-python.xml coverage-scripts.xml .coverage-scripts coverage-node.lcov
	rm -rf $(GO_COUNTER_DIR)
	rm -f sysml sysml-lsp sysml-grpc sysml-jupyter-kernel sysml-mcp
	rm -f cmd/*/rsrc_windows_*.syso
	rm -rf $(SITE_DIR)
	@# Only the default destination; an overridden SELF_MODEL_OUT is the caller's.
	rm -rf build/self-model
	@echo "✓ Cleaned"

install: build ## Install binaries to $GOPATH/bin
	@echo "Installing to $(shell go env GOPATH)/bin..."
	$(GO_INSTALL) ./cmd/sysml
	$(GO_INSTALL) ./cmd/sysml-lsp
	$(GO_INSTALL) ./cmd/sysml-grpc
	$(GO_INSTALL) ./cmd/sysml-jupyter-kernel
	$(GO_INSTALL) ./cmd/sysml-mcp
	@echo "✓ Installed"

# What a distribution's package build calls: staged under DESTDIR, into the
# GNU-conventional paths, binaries and manual pages together.

mcp-registry-check: ## Validate the MCP Registry listing (packaging/mcp/server.json) without credentials
	@echo "Checking the MCP Registry listing..."
	go test -count=1 -run 'TestMCPRegistry' ./tests/hygiene
	scripts/mcp-registry-publish.sh --check

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

# The Java client's message classes are not committed: its Maven build generates them from
# api/proto/sysml.proto (client/java/opensysml-client/pom.xml).
proto-buf: ## Regenerate the Go protobuf stubs
	@echo "Regenerating Go protobuf stubs..."
	$(BUF) generate api/proto --template api/proto/buf.gen.yaml
	@echo "✓ Regenerated Go stubs"

python-proto: ## Regenerate Python protobuf stubs
	@echo "Regenerating Python protobuf stubs..."
	@$(PYTHON) -c "import grpc_tools.protoc" >/dev/null 2>&1 || { echo "Error: grpcio-tools not installed. Run: $(PYTHON) -m pip install grpcio-tools"; exit 1; }
	$(BUF) generate api/proto --template api/proto/buf.gen.python.yaml
	@echo "✓ Regenerated Python stubs"

proto-ts: $(PROTOC_GEN_ES) ## Regenerate the TypeScript stubs the npm client in client/node ships
	@echo "Regenerating TypeScript protobuf stubs..."
	$(BUF) generate api/proto --template api/proto/buf.gen.ts.yaml
	@echo "✓ Regenerated TypeScript stubs"

$(PROTOC_GEN_ES): $(NODE_DIR)/package-lock.json
	cd $(NODE_DIR) && npm install --ignore-scripts --no-audit --no-fund

proto-rust: ## Generate Rust stubs and the descriptor for the Rust clients
	$(BUF) generate api/proto --template api/proto/buf.gen.rust.yaml
	$(BUF) build api/proto -o client/rust/conformance/sysml.descriptor.binpb

proto-lint: ## Lint the protobuf schema
	$(BUF) lint api/proto
	@echo "✓ Proto lint passed"

proto-breaking: ## Check the protobuf schema for wire-breaking changes against develop
	@# An archive, not the .git directory: buf would clone that, which a blobless (CI) checkout cannot serve.
	@# The subtree as the tree-ish, not a pathspec: a pathspec walks the whole tree and lazily fetches its blobs.
	baseline=$$(mktemp -t proto-baseline.XXXXXX) && trap 'rm -f "$$baseline"' EXIT && \
	git archive --format=tar -o "$$baseline" '$(BUF_BREAKING_REF):api/proto' && \
	$(BUF) breaking api/proto --against "$$baseline#format=tar"
	@echo "✓ No breaking schema changes"

python-install: ## Install the Python client in editable mode
	@echo "Installing opensysml..."
	cd $(PYTHON_DIR) && pip install -e .
	@echo "✓ Installed opensysml"

python-test: ## Run Python client tests
	@echo "Running Python client tests..."
	cd $(PYTHON_DIR) && pytest tests/ -v
	@echo "✓ Python client tests passed"

jupyter-kernel-install: ## Install the jupyter-opensysml-kernel package in editable mode
	@echo "Installing jupyter-opensysml-kernel..."
	cd $(JUPYTER_KERNEL_DIR) && pip install -e .
	@echo "✓ Installed jupyter-opensysml-kernel"

# The wheel tests build with python -m build, so it must be installed too.
jupyter-kernel-test: ## Run the jupyter-opensysml-kernel package tests and type check
	@echo "Running jupyter-opensysml-kernel tests..."
	cd $(JUPYTER_KERNEL_DIR) && pytest tests/ -v && mypy jupyter_opensysml_kernel
	@echo "✓ jupyter-opensysml-kernel tests passed"

# Run from the repo root so the report records repo-relative paths, which is
# what the SonarCloud scan resolves against.
python-coverage: ## Run Python client tests and write coverage-python.xml
	@echo "Running Python client tests with coverage..."
	pytest $(PYTHON_DIR)/tests --cov=opensysml --cov-report=xml:coverage-python.xml --cov-report=term
	@echo "✓ Wrote coverage-python.xml"

# The repository scripts the checks run, measured the same way. Each script runs
# the way CI runs it, so the report credits what the checks execute. The release
# scripts under client/python/scripts are loaded by path, so their tests run here too.
SCRIPTS_COVERAGE := $(PYTHON) -m coverage run --append --rcfile=scripts/coverage-scripts.ini

scripts-coverage: ## Run the repository scripts and their tests under coverage and write coverage-scripts.xml
	@echo "Running the repository scripts with coverage..."
	$(PYTHON) -m coverage erase --rcfile=scripts/coverage-scripts.ini
	$(SCRIPTS_COVERAGE) scripts/changelog-test.py
	$(SCRIPTS_COVERAGE) scripts/changelog.py check
	$(SCRIPTS_COVERAGE) scripts/release-announce-test.py
	$(SCRIPTS_COVERAGE) scripts/mkdocs_census-test.py
	$(SCRIPTS_COVERAGE) scripts/mkdocs_suite_figures-test.py
	$(SCRIPTS_COVERAGE) scripts/mkdocs_install_scripts-test.py
	$(SCRIPTS_COVERAGE) scripts/dedupe-coverage-test.py
	$(SCRIPTS_COVERAGE) scripts/go-test-stream-test.py
	$(SCRIPTS_COVERAGE) scripts/check-doc-links.py
	$(SCRIPTS_COVERAGE) scripts/check-doc-ids.py
	$(SCRIPTS_COVERAGE) scripts/check-doc-figures.py
	$(SCRIPTS_COVERAGE) scripts/sync-release-digests.py --check
	$(SCRIPTS_COVERAGE) -m pytest -q $(PYTHON_DIR)/tests/test_check_version.py $(PYTHON_DIR)/tests/test_pin_release_checksums.py $(PYTHON_DIR)/tests/test_snapshot_version.py
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

vscode-coverage: ## Run the VS Code extension's tests and write coverage-vscode.lcov
	@echo "Running the VS Code extension tests with coverage..."
	cd $(VSCODE_DIR) && npm ci && npm run test:coverage
	sed -e 's|^SF:|SF:$(VSCODE_DIR)/|' $(VSCODE_DIR)/coverage/lcov.info > coverage-vscode.lcov
	@echo "✓ Wrote coverage-vscode.lcov"

jupyterlab-coverage: ## Run the JupyterLab extension's tests and write coverage-jupyterlab.lcov
	@echo "Running the JupyterLab extension tests with coverage..."
	cd $(JUPYTERLAB_DIR) && npm ci && npm run test:coverage
	sed -e 's|^SF:|SF:$(JUPYTERLAB_DIR)/|' $(JUPYTERLAB_DIR)/coverage/lcov.info > coverage-jupyterlab.lcov
	@echo "✓ Wrote coverage-jupyterlab.lcov"

mcp-npm-coverage: ## Run the sysml-mcp npm package's tests and write coverage-mcp-npm.lcov
	@echo "Running the sysml-mcp npm package tests with coverage..."
	cd $(MCP_NPM_DIR) && npm run test:coverage
	sed -e 's|^SF:|SF:$(MCP_NPM_DIR)/|' $(MCP_NPM_DIR)/coverage/lcov.info > coverage-mcp-npm.lcov
	@echo "✓ Wrote coverage-mcp-npm.lcov"

vscode-grammar: ## Regenerate the VS Code TextMate grammars and the JupyterLab CodeMirror syntax table from the keyword lists
	@echo "Generating TextMate grammars and the CodeMirror syntax table..."
	go run ./$(VSCODE_DIR)/tools/gengrammar -out $(VSCODE_DIR)/syntaxes -codemirror-out $(JUPYTERLAB_DIR)/src/syntax.json
	@echo "✓ Grammars generated"

# `jupyter labextension build` needs the jupyterlab Python package on PATH
# beside Node; the bundle lands in the kernel package, which ships it.
jupyterlab-build: ## Build the JupyterLab extension into client/jupyter-kernel (needs Node and the jupyterlab package)
	@echo "Building the JupyterLab extension..."
	cd $(JUPYTERLAB_DIR) && npm ci && npm run build
	cp $(JUPYTERLAB_DIR)/install.json $(JUPYTER_KERNEL_DIR)/labextension/
	@echo "✓ Built $(JUPYTER_KERNEL_DIR)/labextension"

jupyterlab-test: ## Run the JupyterLab extension's tokenizer tests
	@echo "Running the JupyterLab extension tests..."
	cd $(JUPYTERLAB_DIR) && npm ci && npm test
	@echo "✓ JupyterLab extension tests passed"

jupyterlab-install: jupyterlab-build ## Link the built JupyterLab extension into the current Jupyter environment for development
	jupyter labextension develop --overwrite $(JUPYTER_KERNEL_DIR)
	@echo "✓ Linked jupyterlab-opensysml; restart JupyterLab to load it"

vscode-build: ## Type-check and bundle the VS Code extension
	@echo "Building the VS Code extension..."
	cd $(VSCODE_DIR) && npm ci && npm run typecheck && npm run build
	@echo "✓ Built $(VSCODE_DIR)/dist/extension.js"

vscode-package: ## Package the VS Code extension as a .vsix for side-loading (VSIX_VERSION= stamps a version other than the manifest's)
	@echo "Packaging the VS Code extension..."
	@# `npm run package -- <args>` appends the arguments to the script's last command, `vsce package`.
	cd $(VSCODE_DIR) && npm ci && npm run package $(if $(VSIX_VERSION),-- $(VSIX_VERSION) --no-update-package-json)
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

docs-counts: ## Regenerate and verify the committed documentation counts; the test-suite figures are counted when the site is built
	@echo "Regenerating the documentation count lines and refereed figures..."
	go run -C $(TOOLS_DIR) ./cmd/doc-counts
	go run -C $(TOOLS_DIR) ./cmd/doc-counts -check
	go run -C $(TOOLS_DIR) ./cmd/validation-census -check
	go run -C $(TOOLS_DIR) ./cmd/transformation-census -check
	go test -C $(TOOLS_DIR) -count=1 ./census/doccounts ./census/validation ./census/transformation ./referee/diff ./referee/reject ./referee/apijson
	@echo "✓ Documentation counts and refereed figures are current"

diagnostics-catalogue: ## Regenerate the diagnostic code catalogue in docs/reference/diagnostics.md from the registry
	@echo "Writing the diagnostic code catalogue..."
	go test -count=1 -run TestCatalogueIsCurrent ./internal/check/codes -update

diagnostics-check: ## Verify the diagnostic code catalogue is current
	@echo "Checking the diagnostic code catalogue..."
	go test -count=1 -run TestCatalogueIsCurrent ./internal/check/codes

docs-check: ## Verify documentation links, internal-label hygiene, quoted oracle figures, changelog fragments and the build-time census and test-suite figures
	$(PYTHON) scripts/check-doc-links.py
	$(PYTHON) scripts/check-doc-ids.py
	$(PYTHON) scripts/check-doc-figures.py
	$(PYTHON) scripts/changelog.py check
	$(MAKE) diagnostics-check
	$(PYTHON) scripts/mkdocs_census-test.py
	$(PYTHON) scripts/mkdocs_suite_figures-test.py
	$(PYTHON) scripts/griffe_sphinx_roles-test.py
	$(PYTHON) scripts/mkdocs_install_scripts-test.py

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

# The home page's diagram client and the cli page's in-browser REPL fetch these
# assets; they are built, never vendored. The Pages job runs this target before
# `docs`; a local `make docs` or `docs-serve` preview wants the same target first
# or neither page can load its module.
docs-engine-assets: ## Build the in-browser WebAssembly assets into docs/assets
	@mkdir -p docs/assets
	GOOS=js GOARCH=wasm $(GO_BUILD) -trimpath -o docs/assets/sysml-wasm.wasm ./cmd/sysml-wasm
	gzip -9nf docs/assets/sysml-wasm.wasm
	GOOS=js GOARCH=wasm $(GO_BUILD) -tags sysml_re -o docs/assets/sysml-repl.wasm ./cmd/sysml
	gzip -9f docs/assets/sysml-repl.wasm
	@mkdir -p docs/assets/repl-examples
	@cp examples/runtime-showcase/*.sysml docs/assets/repl-examples/
	@cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" docs/assets/
	@echo "✓ Built docs/assets/sysml-wasm.wasm.gz, sysml-repl.wasm.gz, repl-examples/ + wasm_exec.js"

# The landing page's diagram bundles the browser client from client/node's source, so that
# package is installed first; `npm install`, not `npm ci`, see node_install_test.go in tests/hygiene.
docs-landing-assets: ## Build the landing-page diagram assets into docs/assets
	cd $(NODE_DIR) && npm install --ignore-scripts --no-audit --no-fund
	cd $(VSCODE_DIR) && npm ci && npm run build:landing

docs-serve: docs-install ## Serve docs with live reload (build docs-engine-assets and docs-landing-assets first)
	$(PYTHON) -m mkdocs serve --strict

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
