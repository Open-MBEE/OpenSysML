;; Emscripten's libavoid build raises C++ exceptions through the host import
;; __throw_exception_with_stack_trace, which the JavaScript glue turns back into
;; a WebAssembly exception the module's own catch handlers see. In Go there is
;; no glue, so this module takes the import's place: it throws the module's
;; exported C++ exception tag directly, and wasm-merge links the two.
(module
  (import "libavoid" "__cpp_exception" (tag $cpp (param i32)))
  (func (export "__throw_exception_with_stack_trace") (param i32)
    (throw $cpp (local.get 0)))
)
