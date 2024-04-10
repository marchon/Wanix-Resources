// import is a keyword not a global.
// this makes it usable from go.
globalThis.import = (url) => import(url);

addEventListener("message", async (e) => {
  if (e.data.duplex) return;
  if (e.data.init) {
    
    globalThis.hostURL = e.data.init.hostURL;
    globalThis.bootfs = e.data.init.fs;
    globalThis.process.pid = e.data.init.pid;
    globalThis.process.ppid = e.data.init.ppid;
    globalThis.process.dir = e.data.init.dir;

    globalThis.duplex = await import(URL.createObjectURL(bootfs["duplex.js"].blob));
    globalThis.task = await import(URL.createObjectURL(bootfs["task.js"].blob)); // only for kernel
    
    globalThis.sys = duplex.open(new duplex.WorkerConn(globalThis), new duplex.CBORCodec());
    
    sys.handle("exec", duplex.HandlerFunc(async (resp, call) => {
      const params = await call.receive();
      const go = new Go();
      go.argv = [params[0], ...(params[1]||[])];
      if (params[2].env) {
        go.env = params[2].env;
      }
      let mod;
      if (bootfs[params[0]]) {
        mod = await blobToArrayBuffer(bootfs[params[0]].blob);
      } else {
        mod = await globalThis.fs.readFile(params[0]);
      }
      // TODO: handle params[2].stdin
      const res = await WebAssembly.instantiate(mod, go.importObject);
      await go.run(res.instance);
      globalThis.stdin(null, () => null);
      globalThis.stdout(null);
      globalThis.stderr(null);
      resp.return(go.exitcode);
    }))
    sys.handle("stdout", duplex.HandlerFunc(async (resp, call) => {
      const ch = await resp.continue();
      globalThis.stdout = (buf) => {
        if (buf === null) {
          ch.close();
          return;
        }
        ch.write(buf);
      }
    }));
    sys.handle("stderr", duplex.HandlerFunc(async (resp, call) => {
      const ch = await resp.continue();
      globalThis.stderr = (buf) => {
        if (buf === null) {
          ch.close();
          return;
        }
        ch.write(buf);
      }
    }));
    sys.handle("output", duplex.HandlerFunc(async (resp, call) => {
      const ch = await resp.continue();
      globalThis.stdout = (buf) => {
        if (buf === null) {
          ch.close();
          return;
        }
        ch.write(buf);
      }
      globalThis.stderr = (buf) => {
        if (buf === null) {
          ch.close();
          return;
        }
        ch.write(buf);
      }
    }));
    sys.handle("stdin", duplex.HandlerFunc(async (resp, call) => {
      await call.receive();
      const ch = await resp.continue();
      globalThis.stdin = (buf, cb) => {
        if (buf === null) {
          ch.close();
          return;
        }
        ch.read(buf).then(n => cb(null, n));
      }
    }));
    sys.respond();
    
    postMessage({ready: true});
  }
});

function blobToArrayBuffer(blob) {
  return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = event => resolve(event.target.result);
      reader.onerror = reject;
      reader.readAsArrayBuffer(blob);
  });
}

//# sourceURL=worker.js
