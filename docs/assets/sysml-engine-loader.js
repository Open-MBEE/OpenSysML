(function () {
  window.osmlLoadEngine = function (urls) {
    if (globalThis.sysmlEngine) return Promise.resolve(globalThis.sysmlEngine);
    if (!window.__osmlEngineLoad) {
      window.__osmlEngineLoad = new Promise(function (resolve, reject) {
        if (!("DecompressionStream" in window)) {
          reject(new Error("this browser cannot inflate the engine"));
          return;
        }
        function start() {
          var go = new Go();
          fetch(urls.engine).then(function (r) {
            if (!r.ok) throw new Error("the engine did not download (HTTP " + r.status + ")");
            return new Response(r.body.pipeThrough(new DecompressionStream("gzip"))).arrayBuffer();
          }).then(function (b) {
            return WebAssembly.instantiate(b, go.importObject);
          }).then(function (res) {
            if (globalThis.sysmlEngine) { resolve(globalThis.sysmlEngine); return; }
            go.run(res.instance);
            var tries = 0;
            (function wait() {
              if (globalThis.sysmlEngine) { resolve(globalThis.sysmlEngine); return; }
              if (++tries > 250) { reject(new Error("the engine did not start")); return; }
              setTimeout(wait, 40);
            })();
          }).catch(reject);
        }
        if (typeof Go === "function") { start(); return; }
        var s = document.createElement("script");
        s.src = urls.wasmExec;
        s.onload = start;
        s.onerror = function () { reject(new Error("wasm_exec.js did not load")); };
        document.head.appendChild(s);
      });
      window.__osmlEngineLoad.catch(function () { window.__osmlEngineLoad = null; });
    }
    return window.__osmlEngineLoad;
  };
})();
