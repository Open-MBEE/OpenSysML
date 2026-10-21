// Command conformance runs the language-independent conformance suite in
// conformance/ against sysml-grpc, which it builds and starts itself.
//
// It is both the CI gate and the executable specification a client in another
// language ports: the scenarios are data, and this program is one reading of
// them. See conformance/README.md for the comparison rules it implements.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"

	_ "github.com/Open-MBEE/OpenSysML/api/proto" // registers the schema this runner reflects over
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/report/junit"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"

	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	// serviceName is the service every scenario addresses.
	serviceName                 = "sysml.SysMLService"
	transportConnect            = "connect"
	transportGRPC               = "grpc"
	testWithholdCapabilitiesEnv = "OPENSYSML_TEST_WITHHOLD_CAPABILITIES"
)

var knownProtocols = map[string]struct{}{"grpc": {}, "connect": {}, "connect-json": {}, "pkg": {}, "pkg-connect": {}, "mcp": {}}

func main() {
	var (
		dir       = flag.String("dir", "", "conformance suite directory (scenarios/ and fixtures/; default: <repo>/conformance)")
		binary    = flag.String("binary", "", "sysml-grpc binary to test; built from <repo>/cmd/sysml-grpc when empty")
		mcpBinary = flag.String("mcp-binary", "", "sysml-mcp binary the mcp protocol tests; built from <repo>/cmd/sysml-mcp when empty")
		repoRoot  = flag.String("repo", "", "repository root to build the service from (default: the product module root above the working directory)")
		report    = flag.String("report", "", "write the machine-readable summary to this file (- for stdout)")
		junitOut  = flag.String("junit", "", "also write the results as JUnit XML to this file")
		run       = flag.String("run", "", "run only the scenarios whose id matches this regular expression")
		verbose   = flag.Bool("v", false, "print each scenario's normalized response")
		allowSkip = flag.Bool("allow-skips", false, "treat a scenario skipped for a missing capability as a pass")
		protocols = flag.String("protocols", "grpc,connect,connect-json", "Comma-separated protocols to test")
		transport = flag.String("transport", "connect", "server transport (connect or grpc)")
		withhold  = flag.String("withhold-capabilities", "", "also test the service with these comma-separated capabilities unavailable")
	)
	flag.Parse()

	root, err := repo.Choose(*repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "conformance: %v\n", err)
		os.Exit(1)
	}
	*dir = repo.Resolve(root, *dir)
	if *dir == "" {
		*dir = filepath.Join(root, "conformance")
	}
	if *report != "-" {
		*report = repo.Resolve(root, *report)
	}
	opts := options{
		dir:       *dir,
		binary:    repo.Resolve(root, *binary),
		mcpBinary: repo.Resolve(root, *mcpBinary),
		repoRoot:  root,
		report:    *report,
		junit:     repo.Resolve(root, *junitOut),
		run:       *run,
		verbose:   *verbose,
		allowSkip: *allowSkip,
		protocols: *protocols,
		transport: *transport,
		withhold:  *withhold,
	}
	if err := runSuite(opts); err != nil {
		fmt.Fprintf(os.Stderr, "conformance: %v\n", err)
		os.Exit(1)
	}
}

// options is one suite run's command line.
type options struct {
	dir       string
	binary    string
	mcpBinary string
	repoRoot  string
	report    string
	junit     string
	run       string
	verbose   bool
	allowSkip bool
	protocols string
	transport string
	withhold  string
}

func runSuite(opts options) error {
	plan, err := planSuite(opts)
	if err != nil {
		return err
	}
	scenarios, err := loadScenarios(filepath.Join(opts.dir, "scenarios"))
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workDir, err := os.MkdirTemp("", "conformance-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	if err := buildMissingBinaries(&opts, plan, workDir); err != nil {
		return err
	}
	configurations := []configuration{{name: "default"}}
	if len(plan.unavailable) > 0 {
		configurations = append(configurations, configuration{
			name:        "without-" + strings.Join(plan.unavailable, "-"),
			unavailable: plan.unavailable,
		})
	}

	reportData := &Report{}
	if plan.needsService {
		reportData.Service = opts.binary
	}
	if plan.testsMCP {
		reportData.MCPServer = opts.mcpBinary
	}
	defaultCapabilities := map[string][]string{}
	for _, config := range configurations {
		fmt.Fprintf(os.Stdout, "\nConfiguration %s\n", config.name)
		configReport, err := runConfiguration(ctx, opts, plan, config, scenarios, workDir, defaultCapabilities)
		if err != nil {
			return err
		}
		reportData.Configurations = append(reportData.Configurations, configReport)
		reportData.add(configReport)
	}
	return finishSuite(opts, reportData)
}

// suitePlan is what the options ask the suite to run: the protocols, the
// capabilities to withhold in a second configuration, and the scenario filter.
type suitePlan struct {
	protocols   []string
	unavailable []string
	filter      *regexp.Regexp
	// The mcp protocol runs its own server; the service is started only for
	// the protocols that talk to it.
	needsService bool
	testsMCP     bool
}

// planSuite validates the options against each other and reads the plan from them.
func planSuite(opts options) (suitePlan, error) {
	var plan suitePlan
	protocols, err := parseProtocols(opts.protocols)
	if err != nil {
		return plan, err
	}
	if opts.transport != transportConnect && opts.transport != transportGRPC {
		return plan, fmt.Errorf("unknown -transport %q; want connect or grpc", opts.transport)
	}
	if err := validateProtocols(protocols, opts.transport); err != nil {
		return plan, err
	}
	unavailable, err := parseCapabilities(opts.withhold)
	if err != nil {
		return plan, err
	}
	if len(unavailable) > 0 && slices.Contains(protocols, "pkg") {
		return plan, errors.New("-withhold-capabilities cannot test the in-process pkg protocol")
	}
	if opts.run != "" {
		compiled, err := regexp.Compile(opts.run)
		if err != nil {
			return plan, fmt.Errorf("bad -run pattern: %w", err)
		}
		plan.filter = compiled
	}
	plan.protocols, plan.unavailable = protocols, unavailable
	plan.needsService = slices.ContainsFunc(protocols, func(protocol string) bool { return protocol != "mcp" })
	plan.testsMCP = slices.Contains(protocols, "mcp")
	return plan, nil
}

// buildMissingBinaries builds the service and MCP server the plan needs when the
// options name no binary for them.
func buildMissingBinaries(opts *options, plan suitePlan, workDir string) error {
	if plan.needsService && opts.binary == "" {
		built, err := buildCommand(opts.repoRoot, workDir, "sysml-grpc")
		if err != nil {
			return err
		}
		opts.binary = built
	}
	if plan.testsMCP && opts.mcpBinary == "" {
		built, err := buildCommand(opts.repoRoot, workDir, "sysml-mcp")
		if err != nil {
			return err
		}
		opts.mcpBinary = built
	}
	return nil
}

// runConfiguration runs every protocol against one capability configuration,
// starting the service it needs; the default configuration records the
// capabilities a withholding configuration is checked against.
func runConfiguration(ctx context.Context, opts options, plan suitePlan, config configuration, scenarios []*Scenario,
	workDir string, defaultCapabilities map[string][]string) (*ConfigurationSummary, error) {
	configReport := &ConfigurationSummary{
		Name:                    config.name,
		UnavailableCapabilities: config.unavailable,
	}
	logName := strings.ReplaceAll(config.name, string(os.PathSeparator), "-")
	var svc *service
	if plan.needsService {
		started, err := startServiceWithCapabilities(ctx, opts.binary, filepath.Join(workDir, logName+".log"),
			len(scenarios)+16, opts.transport, config.unavailable)
		if err != nil {
			return nil, err
		}
		defer started.stop()
		svc = started
	}

	run := &configurationRun{opts: opts, plan: plan, config: config, svc: svc, scenarios: scenarios,
		mcpLog: filepath.Join(workDir, logName+"-mcp.log"), defaultCapabilities: defaultCapabilities}
	for _, protocol := range plan.protocols {
		summary, err := run.protocol(ctx, protocol)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", protocol, err)
		}
		configReport.Protocols = append(configReport.Protocols, summary)
		configReport.add(summary)
		if len(config.unavailable) == 0 && summary.Skipped > 0 && !opts.allowSkip {
			configReport.unacceptedSkip = true
		}
		if err := validateFallbackExecution(summary, config.unavailable, opts.allowSkip); err != nil {
			return nil, fmt.Errorf("%s: %w", protocol, err)
		}
	}
	return configReport, nil
}

// configurationRun is one capability configuration under test: the service
// started for it and what every protocol's run shares.
type configurationRun struct {
	opts                options
	plan                suitePlan
	config              configuration
	svc                 *service
	scenarios           []*Scenario
	mcpLog              string
	defaultCapabilities map[string][]string
}

// protocol opens one protocol's client, checks the capabilities it advertises
// and runs the scenarios over it.
func (r *configurationRun) protocol(ctx context.Context, protocol string) (*Summary, error) {
	var c client
	var err error
	served := r.opts.binary
	if protocol == "mcp" {
		served = r.opts.mcpBinary
		c, err = startMCP(ctx, &mcpSettings{
			binary:      r.opts.mcpBinary,
			root:        filepath.Join(r.opts.dir, "fixtures"),
			logPath:     r.mcpLog,
			cacheSize:   len(r.scenarios) + 16,
			unavailable: r.config.unavailable,
		})
	} else {
		c, err = r.svc.client(protocol)
	}
	if err != nil {
		return nil, err
	}
	defer c.close()
	run := &runner{
		service: served, client: c,
		fixtures: filepath.Join(r.opts.dir, "fixtures"),
		models:   map[string]string{}, verbose: r.opts.verbose,
		omitHandshakeScenarios: len(r.config.unavailable) > 0,
		out:                    os.Stdout, scenarioLog: os.Stdout,
	}
	if err := run.readCapabilities(ctx); err != nil {
		return nil, err
	}
	if len(r.config.unavailable) == 0 {
		r.defaultCapabilities[protocol] = append([]string(nil), run.capabilities...)
	} else if err := validateWithheldCapabilities(
		r.defaultCapabilities[protocol], run.capabilities, r.config.unavailable); err != nil {
		return nil, err
	}
	return run.runAll(ctx, r.scenarios, r.plan.filter), nil
}

// finishSuite writes the reports and turns the suite's failures into the error
// the command exits with.
func finishSuite(opts options, reportData *Report) error {
	if err := writeReport(opts.report, reportData); err != nil {
		return err
	}
	if opts.junit != "" {
		if err := junit.WriteFile(opts.junit, junitReport(reportData)); err != nil {
			return err
		}
	}
	if reportData.failure {
		return fmt.Errorf("%d of %d scenarios failed", reportData.Failed+reportData.Errored, reportData.Total)
	}
	if reportData.unacceptedSkip {
		return fmt.Errorf("%d scenarios were skipped for missing capabilities; pass -allow-skips to accept that", reportData.Skipped)
	}
	return nil
}

type configuration struct {
	name        string
	unavailable []string
}

func parseCapabilities(value string) ([]string, error) {
	var capabilities []string
	seen := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			return nil, fmt.Errorf("capability %q is listed more than once", item)
		}
		seen[item] = struct{}{}
		capabilities = append(capabilities, item)
	}
	return capabilities, nil
}

func validateWithheldCapabilities(defaultCapabilities, actual, unavailable []string) error {
	for _, capability := range unavailable {
		if !slices.Contains(defaultCapabilities, capability) {
			return fmt.Errorf("the default service does not advertise %q", capability)
		}
	}
	want := make([]string, 0, len(defaultCapabilities))
	for _, capability := range defaultCapabilities {
		if !slices.Contains(unavailable, capability) {
			want = append(want, capability)
		}
	}
	if !slices.Equal(actual, want) {
		return fmt.Errorf("capabilities = %v, want the default set without %v: %v",
			actual, unavailable, want)
	}
	return nil
}

func validateFallbackExecution(summary *Summary, unavailable []string, allowSkip bool) error {
	if len(unavailable) == 0 {
		return nil
	}
	executed := map[string]bool{}
	for _, result := range summary.Results {
		if result.WithoutCapability {
			for _, capability := range result.MissingCapabilities {
				executed[capability] = true
			}
		}
		if result.Outcome == "skip" && !allowSkip {
			if len(result.MissingCapabilities) == 0 {
				return fmt.Errorf("%s skipped for a reason other than an unavailable capability", result.ID)
			}
			for _, capability := range result.MissingCapabilities {
				if !slices.Contains(unavailable, capability) {
					return fmt.Errorf("%s skipped for unexpected missing capability %q", result.ID, capability)
				}
			}
		}
	}
	for _, capability := range unavailable {
		if !executed[capability] {
			return fmt.Errorf("no without-capability expectation executed for %q", capability)
		}
	}
	return nil
}

func validateProtocols(protocols []string, transport string) error {
	if transport == transportGRPC {
		for _, protocol := range protocols {
			if protocol != "grpc" && protocol != "pkg" && protocol != "mcp" {
				return fmt.Errorf("protocol %q requires -transport connect; grpc transport only supports grpc, pkg and mcp", protocol)
			}
		}
	}
	return nil
}

func parseProtocols(value string) ([]string, error) {
	var protocols []string
	seen := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := knownProtocols[item]; !ok {
			return nil, fmt.Errorf("unknown protocol %q; want grpc, connect, connect-json, pkg, pkg-connect or mcp", item)
		}
		if _, ok := seen[item]; ok {
			return nil, fmt.Errorf("protocol %q is listed more than once", item)
		}
		seen[item] = struct{}{}
		protocols = append(protocols, item)
	}
	if len(protocols) == 0 {
		return nil, errors.New("-protocols must name at least one protocol")
	}
	return protocols, nil
}

// Report contains aggregate and per-configuration conformance results.
type Report struct {
	Service string `json:"service"`
	// MCPServer is the sysml-mcp binary the mcp protocol tested, when selected.
	MCPServer         string                  `json:"mcp_server,omitempty"`
	Total             int                     `json:"total"`
	Passed            int                     `json:"passed"`
	Failed            int                     `json:"failed"`
	Skipped           int                     `json:"skipped"`
	Errored           int                     `json:"errored"`
	WithoutCapability int                     `json:"without_capability"`
	Configurations    []*ConfigurationSummary `json:"configurations"`
	failure           bool
	unacceptedSkip    bool
}

// ConfigurationSummary contains the results for one capability configuration.
type ConfigurationSummary struct {
	Name                    string     `json:"name"`
	UnavailableCapabilities []string   `json:"unavailable_capabilities,omitempty"`
	Total                   int        `json:"total"`
	Passed                  int        `json:"passed"`
	Failed                  int        `json:"failed"`
	Skipped                 int        `json:"skipped"`
	Errored                 int        `json:"errored"`
	WithoutCapability       int        `json:"without_capability"`
	Protocols               []*Summary `json:"protocols"`
	failure                 bool
	unacceptedSkip          bool
}

func (summary *ConfigurationSummary) add(protocol *Summary) {
	summary.Total += protocol.Total
	summary.Passed += protocol.Passed
	summary.Failed += protocol.Failed
	summary.Skipped += protocol.Skipped
	summary.Errored += protocol.Errored
	summary.WithoutCapability += protocol.WithoutCapability
	summary.failure = summary.failure || protocol.Failed > 0 || protocol.Errored > 0
}

func (report *Report) add(configuration *ConfigurationSummary) {
	report.Total += configuration.Total
	report.Passed += configuration.Passed
	report.Failed += configuration.Failed
	report.Skipped += configuration.Skipped
	report.Errored += configuration.Errored
	report.WithoutCapability += configuration.WithoutCapability
	report.failure = report.failure || configuration.failure
	report.unacceptedSkip = report.unacceptedSkip || configuration.unacceptedSkip
}

// writeReport writes the summary as JSON, to a file or to stdout.
func writeReport(path string, summary *Report) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644) // #nosec G306 -- a CI artifact is meant to be readable.
}

// methodByName resolves a method of the service from the compiled schema, so a
// scenario naming an RPC that does not exist is an error rather than a pass.
func methodByName(name string) (protoreflect.MethodDescriptor, error) {
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, fmt.Errorf("the schema for %s is not registered: %w", serviceName, err)
	}
	svc, ok := descriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, errors.New(serviceName + " is not a service")
	}
	method := svc.Methods().ByName(protoreflect.Name(name))
	if method == nil {
		return nil, fmt.Errorf("%s has no RPC %q", serviceName, name)
	}
	return method, nil
}
