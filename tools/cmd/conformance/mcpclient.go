package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// mcpSettings is what the mcp protocol starts sysml-mcp with.
type mcpSettings struct {
	binary      string
	root        string
	logPath     string
	cacheSize   int
	unavailable []string
}

// mcpClient drives sysml-mcp's runtime tools over stdio: each RPC's tool takes
// the request as its arguments and answers the response as its structured
// result, so the scenarios judge the tools exactly as they judge the service.
type mcpClient struct {
	root    string
	session *mcp.ClientSession
	log     *os.File
}

// startMCP starts sysml-mcp serving the runtime tools over the fixtures as its
// root, and completes the MCP handshake, after which the server is ready.
func startMCP(ctx context.Context, settings *mcpSettings) (*mcpClient, error) {
	root, err := filepath.Abs(settings.root)
	if err != nil {
		return nil, err
	}
	log, err := os.Create(settings.logPath) // #nosec G304 -- logPath is the runner's own temporary directory.
	if err != nil {
		return nil, err
	}
	// #nosec G204 -- binary is what the runner just built, or the one named on the command line.
	process := exec.CommandContext(ctx, settings.binary,
		"-tools", "runtime",
		"-root", root,
		"-cache-size", fmt.Sprint(settings.cacheSize),
		"-log-level", "error",
	)
	process.Stderr = log
	process.Env = withoutEnvironment(os.Environ(), testWithholdCapabilitiesEnv)
	if len(settings.unavailable) > 0 {
		process.Env = append(process.Env,
			testWithholdCapabilitiesEnv+"="+strings.Join(settings.unavailable, ","))
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "conformance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: process}, nil)
	if err != nil {
		_ = log.Close()
		return nil, fmt.Errorf("starting %s: %w; its log is %s", settings.binary, err, settings.logPath)
	}
	return &mcpClient{root: root, session: session, log: log}, nil
}

func (c *mcpClient) protocol() string { return "mcp" }

// close ends the session, which closes the server's stdin and waits for it to exit.
func (c *mcpClient) close() {
	_ = c.session.Close()
	_ = c.log.Close()
}

func (c *mcpClient) call(ctx context.Context, method string, request protoreflect.Message) (protoreflect.Message, error) {
	descriptor, err := methodByName(method)
	if err != nil {
		return nil, err
	}
	if request == nil {
		request = dynamicpb.NewMessage(descriptor.Input())
	}
	tool, err := runtimeTool(method)
	if err != nil {
		return nil, err
	}
	arguments, err := toolArguments(method, request, c.root)
	if err != nil {
		return nil, err
	}
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: arguments})
	if err != nil {
		return nil, fmt.Errorf("calling the %s tool: %w", tool, err)
	}
	if result.IsError {
		return nil, toolStatus(result)
	}
	return toolResponse(method, descriptor.Output(), result.StructuredContent)
}

// runtimeTool names the runtime tool that stands for an RPC. An RPC without
// one is an error, which the runner reports on every scenario naming the RPC.
func runtimeTool(method string) (string, error) {
	entry, ok := catalog.ToolFor(method)
	if !ok || entry.Group != catalog.GroupRuntime {
		return "", fmt.Errorf("sysml-mcp serves no runtime tool for the %s RPC", method)
	}
	return entry.Name, nil
}

// toolArguments spells a request as its tool's arguments. The tools take the
// request under its protojson field names, so the translation is protojson's
// except for parse, whose documents the tool takes as paths and sources.
func toolArguments(method string, request protoreflect.Message, root string) (map[string]any, error) {
	switch method {
	case "GetServerInfo":
		return map[string]any{}, nil
	case "ParseFile":
		return parseFileArguments(request, root)
	case "ParseSources":
		return parseSourcesArguments(request, root)
	}
	raw, err := protojson.Marshal(request.Interface())
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", method, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	fields := map[string]any{}
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", method, err)
	}
	numberIntegers(request.Descriptor(), fields)
	return fields, nil
}

// numberIntegers rewrites the request's own 64-bit integer fields, which
// protojson spells as strings, into the JSON numbers the tool's typed
// arguments (seed, runs, samples) take. Nested messages keep protojson's
// spelling: the tools hand them to protojson, which reads it exactly.
func numberIntegers(descriptor protoreflect.MessageDescriptor, fields map[string]any) {
	all := descriptor.Fields()
	for i := 0; i < all.Len(); i++ {
		field := all.Get(i)
		if !wideInteger(field.Kind()) || field.IsMap() {
			continue
		}
		value, ok := fields[field.JSONName()]
		if !ok {
			continue
		}
		if items, ok := value.([]any); ok {
			for j, item := range items {
				items[j] = numbered(item)
			}
			continue
		}
		fields[field.JSONName()] = numbered(value)
	}
}

func wideInteger(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

func numbered(value any) any {
	if text, ok := value.(string); ok {
		return json.Number(text)
	}
	return value
}

// parseFileArguments spells a ParseFileRequest as the parse tool takes it: the
// file as the one path, or the content as the one inline source.
func parseFileArguments(request protoreflect.Message, root string) (map[string]any, error) {
	fields := request.Descriptor().Fields()
	arguments := map[string]any{}
	if request.Get(fields.ByName("strict_conformance")).Bool() {
		arguments["strictConformance"] = true
	}
	if request.Get(fields.ByName("content_hash")).String() != "" {
		return nil, &uncoveredError{reason: "the parse tool has no content_hash; the field is deprecated"}
	}
	language := request.Get(fields.ByName("language")).String()
	source := request.WhichOneof(fields.ByName("file_path").ContainingOneof())
	switch {
	case source == nil:
		// The tool refuses a parse naming no document, as the service does.
	case source.Name() == "file_path":
		path := request.Get(source).String()
		if err := underRoot(root, path); err != nil {
			return nil, err
		}
		if language != "" {
			return nil, &uncoveredError{reason: "the parse tool reads a path's language from its extension, not from language"}
		}
		arguments["paths"] = []any{path}
	default:
		arguments["sources"] = []any{inlineSource(contentDocumentName, request.Get(source).String(), language)}
	}
	return arguments, nil
}

// contentDocumentName is what the service names the document a ParseFile
// request carries inline; the parse tool is told the name so its parse of the
// same content is the same document.
const contentDocumentName = "<content>"

// parseSourcesArguments spells a ParseSourcesRequest as the parse tool takes
// it: the documents given by path as paths, the inline ones as sources. The
// tool lists paths before sources, so documents interleaving the two kinds
// would come back in another order; such a request is reported uncovered.
func parseSourcesArguments(request protoreflect.Message, root string) (map[string]any, error) {
	fields := request.Descriptor().Fields()
	arguments := map[string]any{}
	if request.Get(fields.ByName("strict_conformance")).Bool() {
		arguments["strictConformance"] = true
	}
	if base := request.Get(fields.ByName("base_model_hash")).String(); base != "" {
		arguments["baseModelHash"] = base
	}
	var paths, sources []any
	documents := request.Get(fields.ByName("documents")).List()
	for i := 0; i < documents.Len(); i++ {
		document := documents.Get(i).Message()
		documentFields := document.Descriptor().Fields()
		language := document.Get(documentFields.ByName("language")).String()
		name := document.Get(documentFields.ByName("name")).String()
		source := document.WhichOneof(documentFields.ByName("file_path").ContainingOneof())
		switch {
		case source == nil:
			return nil, &uncoveredError{reason: "the parse tool takes no document without a path or content"}
		case source.Name() == "file_path":
			path := document.Get(source).String()
			if err := underRoot(root, path); err != nil {
				return nil, err
			}
			if language != "" || name != "" {
				return nil, &uncoveredError{reason: "the parse tool takes a path alone, without a language or a name"}
			}
			if len(sources) > 0 {
				return nil, &uncoveredError{reason: "the parse tool lists paths before inline sources; this request interleaves them"}
			}
			paths = append(paths, path)
		default:
			if language == "" && strings.EqualFold(filepath.Ext(name), ".kerml") {
				// The service reads an inline document without a language as SysML
				// whatever its name; the tool reads a .kerml name as KerML, and
				// naming the language instead would ask for inline_language.
				return nil, &uncoveredError{reason: "the parse tool reads an inline document named " + name + " as KerML, where the service reads one without a language as SysML"}
			}
			sources = append(sources, inlineSource(name, document.Get(source).String(), language))
		}
	}
	if len(paths) > 0 {
		arguments["paths"] = paths
	}
	if len(sources) > 0 {
		arguments["sources"] = sources
	}
	return arguments, nil
}

func inlineSource(name, text, language string) map[string]any {
	source := map[string]any{"path": name, "text": text}
	if language != "" {
		source["language"] = language
	}
	return source
}

// underRoot reports an absolute path outside the server's root as uncovered:
// the runtime tools read only under -root, so such a path cannot reach the
// service through them, whatever the service would answer for it.
func underRoot(root, path string) error {
	if !filepath.IsAbs(path) {
		return nil
	}
	separator := string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(path)+separator, filepath.Clean(root)+separator) {
		return &uncoveredError{reason: fmt.Sprintf("sysml-mcp reads only under its root %s; %s is outside it", root, path)}
	}
	return nil
}

// schemaRefusal opens the tool error the MCP SDK answers for arguments the
// tool's input schema does not admit, before the tool runs.
const schemaRefusal = `validating "arguments": `

// toolStatus reads the status of a refused call: the runtime tools spell it
// "CODE: message", the code being the Connect code name in upper case, and
// arguments the tool's schema refuses are its INVALID_ARGUMENT. A tool error
// in any other form is reported as UNKNOWN with its whole text.
func toolStatus(result *mcp.CallToolResult) error {
	text := toolText(result)
	if message, ok := strings.CutPrefix(text, schemaRefusal); ok {
		return status.Error(codes.InvalidArgument, message)
	}
	if code, message, ok := strings.Cut(text, ": "); ok {
		if mapped := connectCode(code); mapped != codes.Unknown || strings.EqualFold(code, "unknown") {
			return status.Error(mapped, message)
		}
	}
	return status.Error(codes.Unknown, text)
}

func toolText(result *mcp.CallToolResult) string {
	var texts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			texts = append(texts, text.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// toolResponse reads a tool's structured result as the RPC's response message.
// The parse tool answers ParseFile with a ParseSourcesResponse, whose one root
// is the ParseFileResponse's root.
func toolResponse(method string, output protoreflect.MessageDescriptor, structured any) (protoreflect.Message, error) {
	fields, ok := structured.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the tool for %s answered no structured object (%T)", method, structured)
	}
	if method == "ParseFile" {
		roots, _ := fields["roots"].([]any)
		if len(roots) > 1 {
			return nil, fmt.Errorf("the parse tool answered %d roots for one file", len(roots))
		}
		delete(fields, "roots")
		delete(fields, "affected")
		if len(roots) == 1 {
			fields["root"] = roots[0]
		}
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	response := dynamicpb.NewMessage(output)
	if err := protojson.Unmarshal(raw, response); err != nil {
		return nil, fmt.Errorf("the tool for %s answered no %s: %w", method, output.Name(), err)
	}
	return response, nil
}
