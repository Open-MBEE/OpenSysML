package route

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/imports/emscripten"
)

// libavoidWasm is libavoid as scripts/build-libavoid-wasm.sh builds it: the
// libavoid-js release the diagram panel bundles, its exceptions in exnref form.
//
//go:embed libavoid.wasm
var libavoidWasm []byte

// WasmPathEnv names the environment variable that points at a libavoid
// WebAssembly module to route with instead of the embedded one.
const WasmPathEnv = "OPENSYSML_LIBAVOID_WASM"

// engine is the compiled router module, shared by every routing pass of the
// process; each pass instantiates it afresh so no state carries over.
type engine struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
}

var (
	engineOnce sync.Once
	shared     *engine
	engineErr  error
)

func sharedEngine() (*engine, error) {
	engineOnce.Do(func() { shared, engineErr = newEngine(context.Background()) })
	return shared, engineErr
}

func newEngine(ctx context.Context) (*engine, error) {
	wasm := libavoidWasm
	if path := os.Getenv(WasmPathEnv); path != "" {
		var err error
		if wasm, err = os.ReadFile(path); err != nil { // #nosec G304 -- the operator names the module to run.
			return nil, fmt.Errorf("libavoid: %w", err)
		}
	}
	features := api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCoreFeatures(features))
	compiled, err := r.CompileModule(ctx, wasm)
	if err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("libavoid: %w", err)
	}
	if err := instantiateHost(ctx, r, compiled); err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("libavoid: %w", err)
	}
	return &engine{runtime: r, compiled: compiled}, nil
}

// avoidError is a C++ exception libavoid threw, or an assertion it failed.
type avoidError struct{ message string }

func (e avoidError) Error() string { return "libavoid: " + e.message }

// instantiateHost defines the imports Emscripten's module expects: its own
// WASI subset, whose signatures predate the standard ones, and its env glue.
func instantiateHost(ctx context.Context, r wazero.Runtime, compiled wazero.CompiledModule) error {
	wasi := r.NewHostModuleBuilder("wasi_snapshot_preview1")
	wasi.NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, fd, iovs, iovsLen, nwritten uint32) uint32 {
		var total uint32
		for i := uint32(0); i < iovsLen; i++ {
			n, _ := m.Memory().ReadUint32Le(iovs + i*8 + 4)
			total += n
		}
		m.Memory().WriteUint32Le(nwritten, total)
		return 0
	}).Export("fd_write")
	wasi.NewFunctionBuilder().WithFunc(func(fd uint32) uint32 { return 0 }).Export("fd_close")
	wasi.NewFunctionBuilder().WithFunc(func(fd, offsetLow, offsetHigh, whence, newOffset uint32) uint32 { return 70 }).Export("fd_seek")
	wasi.NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, count, size uint32) uint32 {
		m.Memory().WriteUint32Le(count, 0)
		m.Memory().WriteUint32Le(size, 0)
		return 0
	}).Export("environ_sizes_get")
	wasi.NewFunctionBuilder().WithFunc(func(environ, buf uint32) uint32 { return 0 }).Export("environ_get")
	if _, err := wasi.Instantiate(ctx); err != nil {
		return err
	}

	env := r.NewHostModuleBuilder("env")
	exporter, err := emscripten.NewFunctionExporterForModule(compiled)
	if err != nil {
		return err
	}
	exporter.ExportFunctions(env)
	env.NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, condition, file, line, function uint32) {
		panic(avoidError{fmt.Sprintf("assertion %s failed at %s:%d", cString(m, condition), cString(m, file), line)})
	}).Export("__assert_fail")
	env.NewFunctionBuilder().WithFunc(func() { panic(avoidError{"aborted"}) }).Export("abort")
	env.NewFunctionBuilder().WithFunc(func() float64 { return 0 }).Export("emscripten_date_now")
	env.NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, dst, src, n uint32) {
		if b, ok := m.Memory().Read(src, n); ok {
			m.Memory().Write(dst, b)
		}
	}).Export("emscripten_memcpy_big")
	env.NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, size uint32) uint32 {
		current := m.Memory().Size()
		if size <= current {
			return 1
		}
		if _, ok := m.Memory().Grow((size - current + 65535) / 65536); !ok {
			return 0
		}
		return 1
	}).Export("emscripten_resize_heap")
	env.NewFunctionBuilder().WithFunc(func(buf, max, format, tm, locale uint32) uint32 { return 0 }).Export("strftime_l")
	_, err = env.Instantiate(ctx)
	return err
}

// cString reads a NUL-terminated string from the module's memory.
func cString(m api.Module, at uint32) string {
	var b []byte
	for {
		c, ok := m.Memory().ReadByte(at)
		if !ok || c == 0 {
			return string(b)
		}
		b = append(b, c)
		at++
	}
}

// session is one instance of the router module; its calls go through the
// WebIDL binder's exports, and its first failure ends the session's work.
type session struct {
	ctx context.Context
	mod api.Module
	err error
}

func (e *engine) newSession(ctx context.Context) (*session, error) {
	mod, err := e.runtime.InstantiateModule(ctx, e.compiled, wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		return nil, fmt.Errorf("libavoid: %w", err)
	}
	s := &session{ctx: ctx, mod: mod}
	s.call("__wasm_call_ctors")
	return s, s.err
}

func (s *session) close() { _ = s.mod.Close(s.ctx) }

// call invokes an export; after a failure every call is a no-op returning 0.
func (s *session) call(name string, args ...uint64) uint64 {
	if s.err != nil {
		return 0
	}
	fn := s.mod.ExportedFunction(name)
	if fn == nil {
		s.err = fmt.Errorf("libavoid: no export %q", name)
		return 0
	}
	results, err := fn.Call(s.ctx, args...)
	if err != nil {
		s.err = fmt.Errorf("libavoid: %s: %w", name, err)
		return 0
	}
	if len(results) == 0 {
		return 0
	}
	return results[0]
}

// bind calls a bound method, `emscripten_bind_<Class>_<method>_<argc>`.
func (s *session) bind(class, method string, args ...uint64) uint64 {
	return s.call(fmt.Sprintf("emscripten_bind_%s_%s_%d", class, method, len(args)), args...)
}

// method calls a bound instance method on self.
func (s *session) method(class string, self uint64, method string, args ...uint64) uint64 {
	return s.call(fmt.Sprintf("emscripten_bind_%s_%s_%d", class, method, len(args)), append([]uint64{self}, args...)...)
}

// enum is the value of an Avoid enumerator.
func (s *session) enum(name string) uint64 { return s.call("emscripten_enum_Avoid_" + name) }

func (s *session) destroy(class string, self uint64) { s.method(class, self, "__destroy__") }

func f64(v float64) uint64 { return api.EncodeF64(v) }

func boolArg(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}
