//go:build !js && !wasip1

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
// process, with the instances passes have finished with and left clean.
type engine struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule

	mu   sync.Mutex
	idle []*session
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
	ctx   context.Context
	mod   api.Module
	err   error
	fns   map[string]api.Function
	stack []uint64
}

func (e *engine) newSession(ctx context.Context) (*session, error) {
	mod, err := e.runtime.InstantiateModule(ctx, e.compiled, wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		return nil, fmt.Errorf("libavoid: %w", err)
	}
	s := &session{ctx: ctx, mod: mod, fns: map[string]api.Function{}}
	s.call("__wasm_call_ctors")
	return s, s.err
}

// acquire hands out an idle instance, or a fresh one.
func (e *engine) acquire(ctx context.Context) (*session, error) {
	e.mu.Lock()
	if n := len(e.idle); n > 0 {
		s := e.idle[n-1]
		e.idle = e.idle[:n-1]
		e.mu.Unlock()
		return s, nil
	}
	e.mu.Unlock()
	return e.newSession(ctx)
}

// release keeps an instance whose objects were all destroyed for the next
// pass, so its code stack and memory stay grown; a failed one is dropped.
func (e *engine) release(s *session) {
	if s.err != nil {
		_ = s.mod.Close(s.ctx)
		return
	}
	e.mu.Lock()
	e.idle = append(e.idle, s)
	e.mu.Unlock()
}

// call invokes an export; after a failure every call is a no-op returning 0.
func (s *session) call(name string, args ...uint64) uint64 {
	if s.err != nil {
		return 0
	}
	fn, ok := s.fns[name]
	if !ok {
		if fn = s.mod.ExportedFunction(name); fn == nil {
			s.err = fmt.Errorf("libavoid: no export %q", name)
			return 0
		}
		s.fns[name] = fn
	}
	// The stack carries the arguments in and the result (at most one) out.
	s.stack = append(append(s.stack[:0], args...), 0)
	if err := fn.CallWithStack(s.ctx, s.stack); err != nil {
		s.err = fmt.Errorf("libavoid: %s: %w", name, err)
		return 0
	}
	return s.stack[0]
}

// callF64 is call for an export returning an f64.
func (s *session) callF64(name string, args ...uint64) float64 {
	return api.DecodeF64(s.call(name, args...))
}

func f64(v float64) uint64 { return api.EncodeF64(v) }

func i32(v uint32) uint64 { return uint64(v) }

func boolean(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}
