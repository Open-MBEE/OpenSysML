// Runs the real sysml REPL (cmd/sysml built for js/wasm) in a page: an
// in-memory filesystem stands in for the disk and stdin, and mount() draws a
// terminal with history, completion and guided walkthroughs over it.
(function (root) {
  "use strict";

  var enc = new TextEncoder();
  var S_IFDIR = 0x4000, S_IFREG = 0x8000, S_IFCHR = 0x2000;
  var C = { O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_EXCL: 128, O_TRUNC: 512, O_APPEND: 1024, O_DIRECTORY: 65536 };

  function fsError(code) {
    var e = new Error(code);
    e.code = code;
    return e;
  }

  function normalize(path) {
    var parts = [];
    String(path).split("/").forEach(function (p) {
      if (!p || p === ".") return;
      if (p === "..") parts.pop(); else parts.push(p);
    });
    return "/" + parts.join("/");
  }

  // MemFS answers the calls Go's syscall/fs_js makes of globalThis.fs, node's
  // callback API, for one session: fd 0 is the stdin queue, 1 and 2 the output.
  function MemFS(onOutput, onWaiting, defer) {
    var self = this;
    var ino = 1;
    var nodes = { "/": { dir: true, ino: ino++, mtime: Date.now() } };
    var fds = {}, nextFd = 3;
    var stdin = [], pendingRead = null, eof = false;
    var decoders = { 1: new TextDecoder(), 2: new TextDecoder() };
    self.cwd = "/";

    function abs(p) { return p.charAt(0) === "/" ? normalize(p) : normalize(self.cwd + "/" + p); }
    function parent(p) { return normalize(p + "/.."); }
    function stat(n) {
      var size = n.dir || n.chr ? 0 : n.size;
      return {
        dev: 1, ino: n.ino, mode: (n.dir ? S_IFDIR | 493 : n.chr ? S_IFCHR | 438 : S_IFREG | 420),
        nlink: 1, uid: 0, gid: 0, rdev: 0, size: size, blksize: 4096, blocks: Math.ceil(size / 512),
        atimeMs: n.mtime, mtimeMs: n.mtime, ctimeMs: n.mtime,
        isDirectory: function () { return !!n.dir; }
      };
    }
    function mkdirs(p) {
      if (nodes[p]) return;
      mkdirs(parent(p));
      nodes[p] = { dir: true, ino: ino++, mtime: Date.now() };
    }
    function children(p) {
      var pre = p === "/" ? "/" : p + "/";
      return Object.keys(nodes).filter(function (k) {
        return k !== p && k.indexOf(pre) === 0 && k.slice(pre.length).indexOf("/") < 0;
      }).map(function (k) { return k.slice(pre.length); }).sort();
    }
    function grow(n, size) {
      if (n.data.length >= size) return;
      var d = new Uint8Array(Math.max(size, n.data.length * 2, 256));
      d.set(n.data.subarray(0, n.size));
      n.data = d;
    }
    function output(fd, bytes) {
      onOutput(decoders[fd].decode(bytes, { stream: true }), fd);
    }
    function deliver() {
      if (!pendingRead) return;
      if (!stdin.length) {
        if (!eof) return;
        var done = pendingRead;
        pendingRead = null;
        done.cb(null, 0);
        return;
      }
      var r = pendingRead, chunk = stdin[0], n = Math.min(r.length, chunk.length);
      pendingRead = null;
      r.buffer.set(chunk.subarray(0, n), r.offset);
      if (n < chunk.length) stdin[0] = chunk.subarray(n); else stdin.shift();
      r.cb(null, n);
    }

    self.addFile = function (path, content) {
      var p = abs(path);
      mkdirs(parent(p));
      var data = typeof content === "string" ? enc.encode(content) : new Uint8Array(content);
      nodes[p] = { ino: ino++, data: data, size: data.length, mtime: Date.now() };
    };
    self.readFile = function (path) {
      var n = nodes[abs(path)];
      return n && !n.dir ? new TextDecoder().decode(n.data.subarray(0, n.size)) : null;
    };
    self.list = function () {
      return Object.keys(nodes).filter(function (k) { return !nodes[k].dir; }).sort();
    };
    self.send = function (text) {
      if (text) stdin.push(enc.encode(text));
      defer(deliver);
    };
    self.eof = function () { eof = true; defer(deliver); };
    self.waiting = function () { return !!pendingRead && !stdin.length; };

    var ok = function () { var cb = arguments[arguments.length - 1]; cb(null); };
    self.api = {
      constants: C,
      writeSync: function (fd, buf) {
        if (fd === 1 || fd === 2) output(fd, buf);
        return buf.length;
      },
      write: function (fd, buf, offset, length, position, cb) {
        var bytes = buf.subarray(offset, offset + length);
        if (fd === 1 || fd === 2) { output(fd, bytes); cb(null, length); return; }
        var f = fds[fd];
        if (!f || f.node.dir) { cb(fsError("EBADF")); return; }
        var n = f.node, at = position != null ? position : f.append ? n.size : f.pos;
        grow(n, at + length);
        n.data.set(bytes, at);
        n.size = Math.max(n.size, at + length);
        n.mtime = Date.now();
        if (position == null) f.pos = at + length;
        cb(null, length);
      },
      read: function (fd, buffer, offset, length, position, cb) {
        if (fd === 0) {
          pendingRead = { buffer: buffer, offset: offset, length: length, cb: cb };
          if (stdin.length || eof) deliver(); else onWaiting();
          return;
        }
        var f = fds[fd];
        if (!f) { cb(fsError("EBADF")); return; }
        if (f.node.dir) { cb(fsError("EISDIR")); return; }
        var at = position != null ? position : f.pos, n = Math.max(0, Math.min(length, f.node.size - at));
        buffer.set(f.node.data.subarray(at, at + n), offset);
        if (position == null) f.pos = at + n;
        cb(null, n);
      },
      open: function (path, flags, mode, cb) {
        var p = abs(path), n = nodes[p];
        if (!n) {
          if (!(flags & C.O_CREAT)) { cb(fsError("ENOENT")); return; }
          var dir = nodes[parent(p)];
          if (!dir) { cb(fsError("ENOENT")); return; }
          if (!dir.dir) { cb(fsError("ENOTDIR")); return; }
          n = nodes[p] = { ino: ino++, data: new Uint8Array(0), size: 0, mtime: Date.now() };
        } else if ((flags & C.O_CREAT) && (flags & C.O_EXCL)) {
          cb(fsError("EEXIST")); return;
        }
        if (n.dir && (flags & (C.O_WRONLY | C.O_RDWR))) { cb(fsError("EISDIR")); return; }
        if (!n.dir && (flags & C.O_DIRECTORY)) { cb(fsError("ENOTDIR")); return; }
        if (!n.dir && (flags & C.O_TRUNC)) n.size = 0;
        var fd = nextFd++;
        fds[fd] = { node: n, pos: 0, append: !!(flags & C.O_APPEND) };
        cb(null, fd);
      },
      close: function (fd, cb) { delete fds[fd]; cb(null); },
      fstat: function (fd, cb) {
        if (fd <= 2) { cb(null, stat({ chr: true, ino: fd + 1000, mtime: Date.now() })); return; }
        var f = fds[fd];
        if (!f) cb(fsError("EBADF")); else cb(null, stat(f.node));
      },
      stat: function (path, cb) {
        var n = nodes[abs(path)];
        if (!n) cb(fsError("ENOENT")); else cb(null, stat(n));
      },
      lstat: function (path, cb) { self.api.stat(path, cb); },
      readdir: function (path, cb) {
        var p = abs(path), n = nodes[p];
        if (!n) cb(fsError("ENOENT")); else if (!n.dir) cb(fsError("ENOTDIR")); else cb(null, children(p));
      },
      mkdir: function (path, perm, cb) {
        var p = abs(path);
        if (nodes[p]) { cb(fsError("EEXIST")); return; }
        if (!nodes[parent(p)]) { cb(fsError("ENOENT")); return; }
        nodes[p] = { dir: true, ino: ino++, mtime: Date.now() };
        cb(null);
      },
      rename: function (from, to, cb) {
        var a = abs(from), b = abs(to);
        if (!nodes[a]) { cb(fsError("ENOENT")); return; }
        if (!nodes[parent(b)]) { cb(fsError("ENOENT")); return; }
        Object.keys(nodes).forEach(function (k) {
          if (k === a || k.indexOf(a + "/") === 0) {
            nodes[b + k.slice(a.length)] = nodes[k];
            delete nodes[k];
          }
        });
        cb(null);
      },
      unlink: function (path, cb) {
        var p = abs(path), n = nodes[p];
        if (!n) cb(fsError("ENOENT")); else if (n.dir) cb(fsError("EISDIR")); else { delete nodes[p]; cb(null); }
      },
      rmdir: function (path, cb) {
        var p = abs(path), n = nodes[p];
        if (!n) cb(fsError("ENOENT"));
        else if (!n.dir) cb(fsError("ENOTDIR"));
        else if (children(p).length) cb(fsError("ENOTEMPTY"));
        else { delete nodes[p]; cb(null); }
      },
      ftruncate: function (fd, len, cb) {
        var f = fds[fd];
        if (!f || f.node.dir) { cb(fsError("EBADF")); return; }
        grow(f.node, len);
        if (len > f.node.size) f.node.data.fill(0, f.node.size, len);
        f.node.size = len;
        cb(null);
      },
      truncate: function (path, len, cb) {
        self.api.open(path, C.O_WRONLY, 0, function (err, fd) {
          if (err) { cb(err); return; }
          self.api.ftruncate(fd, len, function (e) { delete fds[fd]; cb(e); });
        });
      },
      fsync: ok, chmod: ok, fchmod: ok, chown: ok, fchown: ok, lchown: ok, utimes: ok,
      readlink: function (p, cb) { cb(fsError("EINVAL")); },
      link: function (a, b, cb) { cb(fsError("ENOSYS")); },
      symlink: function (a, b, cb) { cb(fsError("ENOSYS")); }
    };
    self.process = {
      getuid: function () { return -1; }, getgid: function () { return -1; },
      geteuid: function () { return -1; }, getegid: function () { return -1; },
      getgroups: function () { throw fsError("ENOSYS"); },
      pid: -1, ppid: -1,
      umask: function () { return 18; },
      cwd: function () { return self.cwd; },
      chdir: function (d) {
        var p = abs(d);
        if (!nodes[p] || !nodes[p].dir) throw fsError("ENOENT");
        self.cwd = p;
      }
    };
    self.path = { resolve: function () { return abs(Array.prototype.slice.call(arguments).join("/")); } };
  }

  var modules = {};

  // compile fetches a gzipped binary once per URL, reporting bytes as they arrive.
  function compile(url, onProgress) {
    if (!modules[url]) {
      modules[url] = fetch(url).then(function (r) {
        if (!r.ok) throw new Error("the REPL did not download (HTTP " + r.status + ")");
        if (typeof DecompressionStream === "undefined") throw new Error("this browser cannot inflate the REPL");
        var total = +r.headers.get("Content-Length") || 0, got = 0;
        var counted = r.body.pipeThrough(new TransformStream({
          transform: function (chunk, ctl) {
            got += chunk.length;
            if (onProgress) onProgress(got, total);
            ctl.enqueue(chunk);
          }
        }));
        return new Response(counted.pipeThrough(new DecompressionStream("gzip"))).arrayBuffer();
      }).then(function (bytes) { return WebAssembly.compile(bytes); });
      modules[url].catch(function () { delete modules[url]; });
    }
    return modules[url];
  }

  // start runs one REPL process on module. Go reads globalThis.fs, process and
  // path once, at init, so they are swapped in for that synchronous start only.
  function start(opts) {
    var defer = opts.defer || function (fn) { setTimeout(fn, 0); };
    var vfs = new MemFS(opts.onOutput, opts.onWaiting || function () {}, defer);
    Object.keys(opts.files || {}).forEach(function (p) { vfs.addFile(p, opts.files[p]); });
    if (opts.cwd) vfs.cwd = normalize(opts.cwd);
    var go = new Go();
    var code = 0;
    go.argv = ["sysml"].concat(opts.args || []);
    go.env = Object.assign({ HOME: "/home/visitor", TERM: "dumb", OPENSYSML_RECORD_CACHE: "0" }, opts.env || {});
    var exit = go.exit;
    go.exit = function (c) { code = c; exit.call(go, c); };
    return WebAssembly.instantiate(opts.module, go.importObject).then(function (instance) {
      var saved = { fs: globalThis.fs, process: globalThis.process, path: globalThis.path };
      globalThis.fs = vfs.api;
      globalThis.process = vfs.process;
      globalThis.path = vfs.path;
      var done;
      try {
        done = go.run(instance);
      } finally {
        globalThis.fs = saved.fs;
        globalThis.process = saved.process;
        globalThis.path = saved.path;
      }
      var session = {
        fs: vfs,
        send: vfs.send,
        eof: vfs.eof,
        waiting: vfs.waiting,
        exited: false,
        done: null,
        // The REPL installs sysmlReplComplete once its session exists, which
        // can be after go.run yields, so it is looked up on each request.
        complete: function (head) {
          var complete = globalThis.sysmlReplComplete;
          if (typeof complete !== "function" || session.exited || !vfs.waiting()) return null;
          try { return JSON.parse(complete(head)); } catch (e) { return null; }
        }
      };
      session.done = Promise.resolve(done).then(function () {
        session.exited = true;
        return code;
      });
      return session;
    });
  }

  function loadScript(src) {
    if (typeof Go === "function") return Promise.resolve();
    return new Promise(function (resolve, reject) {
      var s = document.createElement("script");
      s.src = src;
      s.onload = resolve;
      s.onerror = function () { reject(new Error("wasm_exec.js did not load")); };
      document.head.appendChild(s);
    });
  }

  var HISTORY_KEY = "osml-repl-history", HISTORY_MAX = 500, OUTPUT_MAX = 4000;

  function loadHistory() {
    try {
      var h = JSON.parse(localStorage.getItem(HISTORY_KEY) || "[]");
      return Array.isArray(h) ? h.filter(function (x) { return typeof x === "string"; }) : [];
    } catch (e) { return []; }
  }
  function saveHistory(h) {
    try { localStorage.setItem(HISTORY_KEY, JSON.stringify(h.slice(-HISTORY_MAX))); } catch (e) { /* private mode */ }
  }

  function lineClass(line) {
    if (/^\s*(✓|=\s)/.test(line)) return "osml-ok";
    if (/^\s*✗|^\s*error:|^sysml: |: error: /.test(line)) return "osml-err";
    if (/: warning: |^\s*warning:/.test(line)) return "osml-hl";
    if (/^\s+(standing:|ID: |Use %)/.test(line)) return "osml-dim";
    return "";
  }

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  // mount wires the terminal markup inside rootEl (see docs/cli.md) to a REPL.
  function mount(rootEl) {
    var d = rootEl.dataset;
    var out = rootEl.querySelector("[data-repl-out]");
    var input = rootEl.querySelector("[data-repl-in]");
    var promptEl = rootEl.querySelector("[data-repl-prompt]");
    var statusEl = rootEl.querySelector("[data-repl-status]");
    var startBtn = rootEl.querySelector("[data-repl-start]");
    var toursEl = rootEl.querySelector("[data-repl-tours]");
    var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
    var session = null, starting = null, partial = "", history = loadHistory(), hIndex = history.length, draft = "";
    var queue = [], typing = false, examples = null, tours = [];

    function status(text, isError) {
      statusEl.textContent = text;
      statusEl.classList.toggle("osml-repl__status--err", !!isError);
    }
    function scroll() { out.scrollTop = out.scrollHeight; }
    function appendLine(text, cls) {
      out.appendChild(el("span", cls || lineClass(text) || null, text + "\n"));
      while (out.childNodes.length > OUTPUT_MAX) out.removeChild(out.firstChild);
    }
    function write(text) {
      var lines = (partial + text).split("\n");
      partial = lines.pop();
      lines.forEach(function (l) { appendLine(l); });
      scroll();
    }
    function onWaiting() {
      if (partial) {
        promptEl.textContent = partial.replace(/ $/, "\u00a0");
        partial = "";
      }
      rootEl.classList.remove("is-busy");
      input.disabled = false;
      pump();
    }

    function fetchExamples() {
      if (!examples) {
        var list = (d.replExamples || "").split(",").filter(Boolean);
        examples = Promise.all(list.map(function (name) {
          return fetch(d.replExampleBase + name).then(function (r) {
            if (!r.ok) throw new Error(name + " did not download (HTTP " + r.status + ")");
            return r.text();
          }).then(function (t) { return [name, t]; });
        })).then(function (pairs) {
          var files = {};
          pairs.forEach(function (p) { files["examples/runtime-showcase/" + p[0]] = p[1]; });
          return files;
        });
        examples.catch(function () { examples = null; });
      }
      return examples;
    }

    function boot() {
      if (session && !session.exited) return Promise.resolve(session);
      if (starting) return starting;
      startBtn.disabled = true;
      var fresh = !modules[d.replWasm];
      status(fresh ? "Downloading the sysml REPL\u2026" : "Starting a new session\u2026");
      starting = loadScript(d.replWasmExec).then(function () {
        return Promise.all([compile(d.replWasm, function (got, total) {
          var mb = function (n) { return (n / 1048576).toFixed(1); };
          status("Downloading the sysml REPL: " + mb(got) + (total ? " of " + mb(total) : "") + " MB\u2026");
        }), fetchExamples()]);
      }).then(function (got) {
        status("Starting the REPL\u2026");
        var t0 = performance.now();
        return start({
          module: got[0],
          files: got[1],
          onOutput: write,
          onWaiting: onWaiting,
          defer: function (fn) { requestAnimationFrame(function () { setTimeout(fn, 0); }); }
        }).then(function (s) {
          session = s;
          rootEl.classList.add("is-live");
          startBtn.hidden = true;
          input.disabled = false;
          input.placeholder = "SysML or a %command \u2014 \u2191\u2193 history, Tab completes, Shift+Enter adds a line";
          status("Running the real sysml REPL in this page (started in " +
            Math.round(performance.now() - t0) + " ms). The runtime-showcase models are in examples/runtime-showcase/.");
          s.done.then(function () {
            rootEl.classList.remove("is-live", "is-busy");
            promptEl.textContent = "$\u00a0";
            appendLine("[session ended \u2014 press Enter to start a new one]", "osml-dim");
            scroll();
            status("The session ended. Press Enter in the prompt, or run a walkthrough step, to start a new one.");
            queue = [];
          });
          return s;
        });
      }).catch(function (e) {
        status("The REPL could not start: " + e.message, true);
        throw e;
      }).then(function (s) {
        starting = null;
        startBtn.disabled = false;
        return s;
      }, function (e) {
        starting = null;
        startBtn.disabled = false;
        throw e;
      });
      return starting;
    }

    function remember(line) {
      if (line.trim() && history[history.length - 1] !== line) {
        history.push(line);
        if (history.length > HISTORY_MAX) history.splice(0, history.length - HISTORY_MAX);
        saveHistory(history);
      }
      hIndex = history.length;
      draft = "";
    }

    // submit hands one line to the REPL, echoing it after the prompt it answers.
    function submit(line) {
      appendLine(promptEl.textContent.replace(/\u00a0/g, " ") + line, "osml-repl__echo");
      scroll();
      rootEl.classList.add("is-busy");
      session.send(line + "\n");
    }

    function setInput(text) {
      input.value = text;
      input.style.height = "auto";
      input.style.height = input.scrollHeight + "px";
      input.style.overflowY = input.scrollHeight > input.clientHeight + 1 ? "auto" : "hidden";
    }
    function splitLines(text) { return text.replace(/\r\n?/g, "\n").split("\n"); }

    // pump feeds queued lines one per prompt, typing those from a walkthrough
    // where motion is allowed.
    function pump() {
      if (typing || !queue.length || !session || session.exited || !session.waiting()) return;
      var next = queue.shift(), line = next.text;
      if (!next.type || reduceMotion.matches || line.length > 160) {
        submit(line);
        return;
      }
      typing = true;
      var i = 0, step = Math.max(1, Math.ceil(line.length / 40));
      (function type() {
        i = Math.min(line.length, i + step);
        setInput(line.slice(0, i));
        if (i < line.length) { setTimeout(type, 12); return; }
        setTimeout(function () {
          typing = false;
          setInput("");
          submit(line);
        }, 120);
      })();
    }

    // run sends a block one line per prompt, as a terminal would, and keeps
    // the block as one history entry.
    function run(text, type) {
      remember(text);
      return boot().then(function () {
        queue = queue.concat(splitLines(text).map(function (l) { return { text: l, type: type }; }));
        pump();
      }, function () {});
    }

    function complete() {
      if (!session || session.exited) return;
      var pos = input.selectionStart, head = input.value.slice(0, pos), tail = input.value.slice(pos);
      var c = session.complete(head.split("\n").pop());
      if (!c || !c.candidates || !c.candidates.length) return;
      var prefix = c.prefix || "", cands = c.candidates;
      var common = cands.reduce(function (a, b) {
        var n = 0;
        while (n < a.length && n < b.length && a[n] === b[n]) n++;
        return a.slice(0, n);
      });
      var base = head.slice(0, head.length - prefix.length);
      if (cands.length === 1) {
        var word = cands[0] + (/[/]$/.test(cands[0]) ? "" : " ");
        setInput(base + word + tail);
        input.setSelectionRange((base + word).length, (base + word).length);
        return;
      }
      if (common.length > prefix.length) {
        setInput(base + common + tail);
        input.setSelectionRange((base + common).length, (base + common).length);
        return;
      }
      appendLine(promptEl.textContent.replace(/\u00a0/g, " ") + input.value, "osml-repl__echo");
      appendLine(cands.slice(0, 60).join("  ") + (cands.length > 60 ? "  \u2026 " + (cands.length - 60) + " more" : ""), "osml-dim");
      scroll();
    }

    input.addEventListener("keydown", function (e) {
      if (typing) { e.preventDefault(); return; }
      var v = input.value;
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        if (!session || session.exited) {
          boot().then(function () { input.focus(); }, function () {});
          return;
        }
        if (!session.waiting() || queue.length) return;
        setInput("");
        run(v, false);
      } else if ((e.key === "ArrowUp" && v.lastIndexOf("\n", input.selectionStart - 1) < 0) || (e.ctrlKey && e.key === "p")) {
        e.preventDefault();
        if (!hIndex) return;
        if (hIndex === history.length) draft = v;
        setInput(history[--hIndex]);
        input.setSelectionRange(input.value.length, input.value.length);
      } else if ((e.key === "ArrowDown" && v.indexOf("\n", input.selectionEnd) < 0) || (e.ctrlKey && e.key === "n")) {
        e.preventDefault();
        if (hIndex >= history.length) return;
        setInput(++hIndex === history.length ? draft : history[hIndex]);
        input.setSelectionRange(input.value.length, input.value.length);
      } else if (e.key === "Tab" && !e.shiftKey && input.value.trim()) {
        e.preventDefault();
        complete();
      } else if (e.ctrlKey && e.key === "l") {
        e.preventDefault();
        out.textContent = "";
      } else if (e.ctrlKey && e.key === "c" && input.selectionStart === input.selectionEnd) {
        e.preventDefault();
        appendLine(promptEl.textContent.replace(/\u00a0/g, " ") + input.value + "^C", "osml-dim");
        setInput("");
        queue = [];
        scroll();
      } else if (e.ctrlKey && e.key === "d" && !input.value && session && !session.exited) {
        e.preventDefault();
        session.eof();
      }
    });
    input.addEventListener("input", function () { setInput(input.value); });
    out.addEventListener("click", function () {
      if (!window.getSelection().toString() && !input.disabled) input.focus();
    });
    startBtn.addEventListener("click", function () { boot().then(function () { input.focus(); }, function () {}); });

    // Walkthroughs: each step narrates, shows its input, and types it into the REPL.
    var tourIndex = 0, stepIndex = 0;
    function renderTour() {
      var tour = tours[tourIndex], step = tour.steps[stepIndex];
      toursEl.querySelectorAll("[data-tour]").forEach(function (b) {
        b.setAttribute("aria-selected", String(+b.dataset.tour === tourIndex));
      });
      var card = toursEl.querySelector("[data-tour-card]");
      card.innerHTML = "";
      var head = el("p", "osml-repl__tourhead");
      head.appendChild(el("span", "osml-repl__tourstep", "Step " + (stepIndex + 1) + " of " + tour.steps.length));
      head.appendChild(document.createTextNode(" " + step.title));
      card.appendChild(head);
      var text = el("div", "osml-repl__tourtext");
      text.innerHTML = step.text;
      card.appendChild(text);
      if (step.input && step.input.length) {
        var pre = el("pre", "osml-repl__tourinput");
        pre.appendChild(el("code", null, step.input.join("\n")));
        card.appendChild(pre);
      }
      var nav = el("div", "osml-repl__tournav");
      var back = el("button", "osml-repl__btn", "\u25c0 Back");
      back.type = "button";
      back.disabled = stepIndex === 0;
      back.addEventListener("click", function () { stepIndex--; renderTour(); });
      nav.appendChild(back);
      if (step.input && step.input.length) {
        var runBtn = el("button", "osml-repl__btn osml-repl__btn--go", "\u25b6 Run this step");
        runBtn.type = "button";
        runBtn.addEventListener("click", function () {
          run(step.input.join("\n"), true);
          if (stepIndex < tour.steps.length - 1) { stepIndex++; renderTour(); }
          input.focus({ preventScroll: true });
        });
        nav.appendChild(runBtn);
      }
      var next = el("button", "osml-repl__btn", stepIndex < tour.steps.length - 1 ? "Skip \u25b6" : "Next walkthrough \u25b6");
      next.type = "button";
      next.disabled = stepIndex === tour.steps.length - 1 && tourIndex === tours.length - 1;
      next.addEventListener("click", function () {
        if (stepIndex < tour.steps.length - 1) stepIndex++;
        else { tourIndex++; stepIndex = 0; }
        renderTour();
      });
      nav.appendChild(next);
      card.appendChild(nav);
    }
    if (toursEl && d.replTours) {
      fetch(d.replTours).then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.json();
      }).then(function (data) {
        tours = data.tours;
        var tabs = toursEl.querySelector("[data-tour-tabs]");
        tours.forEach(function (t, i) {
          var b = el("button", "osml-repl__btn", (i + 1) + ". " + t.title);
          b.type = "button";
          b.dataset.tour = i;
          b.setAttribute("role", "tab");
          b.addEventListener("click", function () { tourIndex = i; stepIndex = 0; renderTour(); });
          tabs.appendChild(b);
        });
        renderTour();
      }).catch(function (e) {
        toursEl.querySelector("[data-tour-card]").textContent = "The walkthroughs did not load (" + e.message + ").";
      });
    }
  }

  root.osmlRepl = { start: start, compile: compile, mount: mount, MemFS: MemFS };
})(typeof window !== "undefined" ? window : globalThis);
