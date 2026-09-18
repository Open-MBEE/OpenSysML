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

	"github.com/Open-MBEE/OpenSysML/internal/junit"

	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	// serviceName is the service every scenario addresses.
	serviceName                 = "sysml.SysMLService"
	transportConnect            = "connect"
	transportGRPC               = "grpc"
	testWithholdCapabilitiesEnv = "OPENSYSML_TEST_WITHHOLD_CAPABILITIES"
)

var knownProtocols = map[string]struct{}{"grpc": {}, "connect": {}, "connect-json": {}, "pkg": {}, "pkg-connect": {}}

func main() {
	var (
		dir       = flag.String("dir", "conformance", "conformance suite directory (scenarios/ and fixtures/)")
		binary    = flag.String("binary", "", "sysml-grpc binary to test; built from ./cmd/sysml-grpc when empty")
		repoRoot  = flag.String("repo", ".", "repository root to build the service from")
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

	opts := options{
		dir:       *dir,
		binary:    *binary,
		repoRoot:  *repoRoot,
		report:    *report,
		junit:     *junitOut,
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
	protocols, err := parseProtocols(opts.protocols)
	if err != nil {
		return err
	}
	if opts.transport != transportConnect && opts.transport != transportGRPC {
		return fmt.Errorf("unknown -transport %q; want connect or grpc", opts.transport)
	}
	if err := validateProtocols(protocols, opts.transport); err != nil {
		return err
	}
	unavailable, err := parseCapabilities(opts.withhold)
	if err != nil {
		return err
	}
	if len(unavailable) > 0 && slices.Contains(protocols, "pkg") {
		return errors.New("-withhold-capabilities cannot test the in-process pkg protocol")
	}
	var filter *regexp.Regexp
	if opts.run != "" {
		compiled, err := regexp.Compile(opts.run)
		if err != nil {
			return fmt.Errorf("bad -run pattern: %w", err)
		}
		filter = compiled
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

	if opts.binary == "" {
		built, err := buildService(opts.repoRoot, workDir)
		if err != nil {
			return err
		}
		opts.binary = built
	}
	configurations := []configuration{{name: "default"}}
	if len(unavailable) > 0 {
		configurations = append(configurations, configuration{
			name:        "without-" + strings.Join(unavailable, "-"),
			unavailable: unavailable,
		})
	}

	reportData := &Report{Service: opts.binary}
	defaultCapabilities := map[string][]string{}
	for _, config := range configurations {
		fmt.Fprintf(os.Stdout, "\nConfiguration %s\n", config.name)
		configReport := &ConfigurationSummary{
			Name:                    config.name,
			UnavailableCapabilities: config.unavailable,
		}
		err := func() error {
			logName := strings.ReplaceAll(config.name, string(os.PathSeparator), "-") + ".log"
			svc, err := startServiceWithCapabilities(ctx, opts.binary, filepath.Join(workDir, logName),
				len(scenarios)+16, opts.transport, config.unavailable)
			if err != nil {
				return err
			}
			defer svc.stop()

			for _, protocol := range protocols {
				c, err := svc.client(protocol)
				if err != nil {
					return err
				}
				runner := &runner{
					service: svc, client: c,
					fixtures: filepath.Join(opts.dir, "fixtures"),
					models:   map[string]string{}, verbose: opts.verbose,
					omitHandshakeScenarios: len(config.unavailable) > 0,
					out:                    os.Stdout, scenarioLog: os.Stdout,
				}
				if err := runner.readCapabilities(ctx); err != nil {
					c.close()
					return err
				}
				if len(config.unavailable) == 0 {
					defaultCapabilities[protocol] = append([]string(nil), runner.capabilities...)
				} else if err := validateWithheldCapabilities(
					defaultCapabilities[protocol], runner.capabilities, config.unavailable); err != nil {
					c.close()
					return fmt.Errorf("%s: %w", protocol, err)
				}
				summary := runner.runAll(ctx, scenarios, filter)
				c.close()
				configReport.Protocols = append(configReport.Protocols, summary)
				configReport.add(summary)
				if len(config.unavailable) == 0 && summary.Skipped > 0 && !opts.allowSkip {
					configReport.unacceptedSkip = true
				}
				if err := validateFallbackExecution(summary, config.unavailable, opts.allowSkip); err != nil {
					return fmt.Errorf("%s: %w", protocol, err)
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
		reportData.Configurations = append(reportData.Configurations, configReport)
		reportData.add(configReport)
	}
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
			if protocol != "grpc" && protocol != "pkg" {
				return fmt.Errorf("protocol %q requires -transport connect; grpc transport only supports grpc and pkg", protocol)
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
			return nil, fmt.Errorf("unknown protocol %q; want grpc, connect, connect-json, pkg or pkg-connect", item)
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
	Service           string                  `json:"service"`
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
