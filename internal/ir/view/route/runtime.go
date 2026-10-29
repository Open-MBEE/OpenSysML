package route

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// dateNow is what libavoid sees as the clock, so a route never depends on when it was computed.
const dateNow = 0.0

// Router runs one compiled libavoid module; every Route call instantiates it afresh.
type Router struct {
	source   string
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
}

var (
	defaultOnce   sync.Once
	defaultRouter *Router
	defaultErr    error
)

// Default compiles the module named by OPENSYSML_LIBAVOID, else the embedded
// one, once per process and returns it for every caller.
func Default() (*Router, error) {
	defaultOnce.Do(func() {
		data, source, err := moduleBytes()
		if err != nil {
			defaultErr = err
			return
		}
		defaultRouter, defaultErr = NewRouter(data, source)
	})
	return defaultRouter, defaultErr
}

// NewRouter compiles a libavoid module; source names it in errors.
func NewRouter(module []byte, source string) (*Router, error) {
	ctx := context.Background()
	config := wazero.NewRuntimeConfig().
		WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling)
	runtime := wazero.NewRuntimeWithConfig(ctx, config)
	router := &Router{source: source, runtime: runtime}
	if err := router.defineImports(ctx); err != nil {
		_ = runtime.Close(ctx)
		return nil, &ModuleError{Source: source, Err: err}
	}
	compiled, err := runtime.CompileModule(ctx, module)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, &ModuleError{Source: source, Err: err}
	}
	for _, name := range requiredExports {
		if compiled.ExportedFunctions()[name] == nil {
			_ = runtime.Close(ctx)
			return nil, &ModuleError{Source: source, Err: fmt.Errorf("export %q missing", name)}
		}
	}
	router.compiled = compiled
	return router, nil
}

// Close releases the compiled module.
func (r *Router) Close() error {
	return r.runtime.Close(context.Background())
}

// requiredExports are the libavoid bindings the router calls.
var requiredExports = []string{
	"__wasm_call_ctors",
	"emscripten_bind_Router_Router_1",
	"emscripten_bind_Router_setRoutingParameter_2",
	"emscripten_bind_Router_setRoutingOption_2",
	"emscripten_bind_Router_processTransaction_0",
	"emscripten_bind_Router___destroy___0",
	"emscripten_bind_Point_Point_2",
	"emscripten_bind_Point___destroy___0",
	"emscripten_bind_Rectangle_Rectangle_3",
	"emscripten_bind_Rectangle___destroy___0",
	"emscripten_bind_ShapeRef_ShapeRef_2",
	"emscripten_bind_ShapeConnectionPin_ShapeConnectionPin_7",
	"emscripten_bind_ShapeConnectionPin_setExclusive_1",
	"emscripten_bind_ConnEnd_ConnEnd_2",
	"emscripten_bind_ConnEnd___destroy___0",
	"emscripten_bind_ConnRef_ConnRef_3",
	"emscripten_bind_ConnRef_setRoutingType_1",
	"emscripten_bind_ConnRef_displayRoute_0",
	"emscripten_bind_Polygon_size_0",
	"emscripten_bind_Polygon_get_ps_1",
	"emscripten_bind_Point_get_x_0",
	"emscripten_bind_Point_get_y_0",
	"emscripten_enum_Avoid_RouterFlag_OrthogonalRouting",
	"emscripten_enum_Avoid_ConnType_ConnType_Orthogonal",
	"emscripten_enum_Avoid_ConnDirFlag_ConnDirUp",
	"emscripten_enum_Avoid_ConnDirFlag_ConnDirDown",
	"emscripten_enum_Avoid_ConnDirFlag_ConnDirLeft",
	"emscripten_enum_Avoid_ConnDirFlag_ConnDirRight",
	"emscripten_enum_Avoid_RoutingParameter_segmentPenalty",
	"emscripten_enum_Avoid_RoutingParameter_idealNudgingDistance",
	"emscripten_enum_Avoid_RoutingParameter_shapeBufferDistance",
	"emscripten_enum_Avoid_RoutingOption_nudgeOrthogonalSegmentsConnectedToShapes",
	"emscripten_enum_Avoid_RoutingOption_performUnifyingNudgingPreprocessingStep",
	"emscripten_enum_Avoid_RoutingOption_nudgeSharedPathsWithCommonEndPoint",
}

// session is the host-side state of one module instance: the first failure the
// module reported and whatever it wrote to its standard streams.
type session struct {
	failure error
	output  bytes.Buffer
}

type sessionKey struct{}

func sessionOf(ctx context.Context) *session {
	s, _ := ctx.Value(sessionKey{}).(*session)
	if s == nil {
		s = &session{}
	}
	return s
}

// trap is the panic value a host import raises to unwind the module; wazero
// returns it from the exported call as an error.
type trap struct{ err error }

func (t trap) Error() string { return t.err.Error() }

func fail(ctx context.Context, err error) {
	s := sessionOf(ctx)
	if s.failure == nil {
		s.failure = err
	}
	panic(trap{err: err})
}

// defineImports instantiates the two host modules the Emscripten build imports.
func (r *Router) defineImports(ctx context.Context) error {
	env := r.runtime.NewHostModuleBuilder("env")
	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, cond, file, line, fn uint32) {
		fail(ctx, fmt.Errorf("assertion %q failed at %s:%d in %s",
			cString(m, cond), cString(m, file), line, cString(m, fn)))
	}).Export("__assert_fail")
	env.NewFunctionBuilder().WithFunc(func(ctx context.Context) {
		fail(ctx, errors.New("native code called abort()"))
	}).Export("abort")
	env.NewFunctionBuilder().WithFunc(func() float64 { return dateNow }).Export("emscripten_date_now")
	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, dst, src, n uint32) {
		data, ok := m.Memory().Read(src, n)
		if !ok || !m.Memory().Write(dst, data) {
			fail(ctx, fmt.Errorf("memcpy of %d bytes from %#x to %#x leaves memory", n, src, dst))
		}
	}).Export("emscripten_memcpy_big")
	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, size uint32) uint32 {
		return growHeap(m.Memory(), size)
	}).Export("emscripten_resize_heap")
	// libavoid formats no dates; a module that does is not one this router supports.
	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, buf, max, format, tm, locale uint32) uint32 {
		fail(ctx, errors.New("module called strftime_l, which the router does not provide"))
		return 0
	}).Export("strftime_l")
	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, exception uint32) {
		fail(ctx, fmt.Errorf("libavoid threw a C++ exception (object %#x)", exception))
	}).Export("__throw_exception_with_stack_trace")
	if _, err := env.Instantiate(ctx); err != nil {
		return err
	}

	wasi := r.runtime.NewHostModuleBuilder("wasi_snapshot_preview1")
	wasi.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, fd, iovs, count, written uint32) uint32 {
		return fdWrite(ctx, m.Memory(), iovs, count, written)
	}).Export("fd_write")
	wasi.NewFunctionBuilder().WithFunc(func(fd uint32) uint32 { return 0 }).Export("fd_close")
	wasi.NewFunctionBuilder().WithFunc(func(fd, low, high, whence, result uint32) uint32 {
		return 70 // ENOTSUP: nothing seeks on the module's streams
	}).Export("fd_seek")
	wasi.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, count, size uint32) uint32 {
		m.Memory().WriteUint32Le(count, 0)
		m.Memory().WriteUint32Le(size, 0)
		return 0
	}).Export("environ_sizes_get")
	wasi.NewFunctionBuilder().WithFunc(func(environ, buf uint32) uint32 { return 0 }).Export("environ_get")
	_, err := wasi.Instantiate(ctx)
	return err
}

const wasmPageSize = 65536

// growHeap grows linear memory to hold size bytes; 1 reports success to the module.
func growHeap(memory api.Memory, size uint32) uint32 {
	current := memory.Size()
	if size <= current {
		return 1
	}
	pages := (uint64(size) - uint64(current) + wasmPageSize - 1) / wasmPageSize
	if _, ok := memory.Grow(uint32(pages)); !ok {
		return 0
	}
	return 1
}

// fdWrite gathers the module's stdout/stderr into the session for error reports.
func fdWrite(ctx context.Context, memory api.Memory, iovs, count, written uint32) uint32 {
	s := sessionOf(ctx)
	var total uint32
	for i := uint32(0); i < count; i++ {
		ptr, ok1 := memory.ReadUint32Le(iovs + i*8)
		length, ok2 := memory.ReadUint32Le(iovs + i*8 + 4)
		if !ok1 || !ok2 {
			return 28 // EINVAL
		}
		data, ok := memory.Read(ptr, length)
		if !ok {
			return 28
		}
		s.output.Write(data)
		total += length
	}
	memory.WriteUint32Le(written, total)
	return 0
}

func cString(m api.Module, ptr uint32) string {
	if ptr == 0 {
		return ""
	}
	var out []byte
	for {
		b, ok := m.Memory().ReadByte(ptr)
		if !ok || b == 0 {
			return string(out)
		}
		out = append(out, b)
		ptr++
	}
}

// instance is one running copy of the module with its exports bound.
type instance struct {
	ctx     context.Context
	session *session
	module  api.Module
	funcs   map[string]api.Function
	err     error
}

func (r *Router) instantiate(ctx context.Context) (*instance, error) {
	s := &session{}
	ctx = context.WithValue(ctx, sessionKey{}, s)
	config := wazero.NewModuleConfig().WithName("").WithStartFunctions()
	module, err := r.runtime.InstantiateModule(ctx, r.compiled, config)
	if err != nil {
		return nil, &ModuleError{Source: r.source, Err: err}
	}
	in := &instance{ctx: ctx, session: s, module: module, funcs: map[string]api.Function{}}
	for _, name := range requiredExports {
		in.funcs[name] = module.ExportedFunction(name)
	}
	in.call("__wasm_call_ctors")
	if in.err != nil {
		_ = module.Close(ctx)
		return nil, &ModuleError{Source: r.source, Err: in.err}
	}
	return in, nil
}

func (in *instance) close() {
	_ = in.module.Close(in.ctx)
}

// call invokes an export; after the first failure every call is a no-op returning 0.
func (in *instance) call(name string, params ...uint64) uint64 {
	if in.err != nil {
		return 0
	}
	results, err := in.funcs[name].Call(in.ctx, params...)
	if err != nil {
		in.err = in.describe(name, err)
		return 0
	}
	if len(results) == 0 {
		return 0
	}
	return results[0]
}

func (in *instance) describe(name string, err error) error {
	cause := err
	if in.session.failure != nil {
		cause = in.session.failure
	}
	if out := strings.TrimSpace(in.session.output.String()); out != "" {
		return fmt.Errorf("%s: %w (module wrote: %s)", name, cause, out)
	}
	return fmt.Errorf("%s: %w", name, cause)
}

func (in *instance) callF64(name string, params ...uint64) float64 {
	return math.Float64frombits(in.call(name, params...))
}

func f64(v float64) uint64 { return math.Float64bits(v) }

func i32(v uint32) uint64 { return uint64(v) }

func boolean(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}
