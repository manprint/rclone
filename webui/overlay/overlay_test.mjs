// Runs index.js and then overlay.js in a V8 context with just enough of a
// browser, as the page does, and checks the function the overlay replaces
// and the shell commands the page copies.
//
// Run with: node --test overlay_test.mjs

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import vm from "node:vm";

const HERE = dirname(fileURLToPath(import.meta.url));
const INDEX_JS = join(HERE, "..", "assets", "index.js");
const OVERLAY_JS = join(HERE, "overlay.js");

// load returns the context after index.js, and the function the overlay
// replaces as index.js defines it
function load() {
  const memory = () => {
    const map = new Map();
    return {
      getItem: k => (map.has(k) ? map.get(k) : null),
      setItem: (k, v) => { map.set(k, String(v)); },
      removeItem: k => { map.delete(k); },
    };
  };
  const noop = () => {};
  const document = { addEventListener: noop, removeEventListener: noop };
  const sandbox = {
    document,
    location: { href: "http://127.0.0.1:8000/", origin: "http://127.0.0.1:8000", pathname: "/", search: "", reload: noop },
    navigator: { userAgent: "node-test" },
    localStorage: memory(),
    sessionStorage: memory(),
    console,
    URL, URLSearchParams, TextDecoder, TextEncoder,
    setTimeout, clearTimeout, setInterval, clearInterval,
  };
  sandbox.window = sandbox;
  sandbox.globalThis = sandbox;
  sandbox.addEventListener = noop;
  sandbox.removeEventListener = noop;
  const ctx = vm.createContext(sandbox);
  vm.runInContext(readFileSync(INDEX_JS, "utf8"), ctx, { filename: INDEX_JS });
  return { ctx, original: ctx.formatDirSizeParts };
}

// parts returns the result as JSON so that arrays created in the context
// compare with the ones of this realm
const parts = (fn, size) => JSON.stringify(fn(size));

test("index.js defines the function the overlay replaces", () => {
  const { original } = load();
  assert.equal(typeof original, "function");
  assert.notEqual(parts(original, -1), JSON.stringify(["—", ""]));
});

test("overlay shows no count for an unknown size", () => {
  const { ctx } = load();
  vm.runInContext(readFileSync(OVERLAY_JS, "utf8"), ctx, { filename: OVERLAY_JS });
  assert.equal(parts(ctx.formatDirSizeParts, -1), JSON.stringify(["—", ""]));
  assert.equal(parts(ctx.formatDirSizeParts, null), JSON.stringify(["—", ""]));
  assert.equal(parts(ctx.formatDirSizeParts, undefined), JSON.stringify(["—", ""]));
});

test("overlay keeps the result for a known size", () => {
  const { ctx, original } = load();
  const max = vm.runInContext("MAX_SUBPATHS_COUNT", ctx);
  const before = [0, 1, 5, max - 1, max, max + 1].map(size => parts(original, size));
  vm.runInContext(readFileSync(OVERLAY_JS, "utf8"), ctx, { filename: OVERLAY_JS });
  const after = [0, 1, 5, max - 1, max, max + 1].map(size => parts(ctx.formatDirSizeParts, size));
  assert.deepEqual(after, before);
});

// Anyone who can write a file names it, and a copied curl or wget command is
// pasted into a shell, where `$(...)` and backticks run inside double quotes.
// The commands are POSIX shell ones, so there is no sh to parse them on Windows.
test("copied curl and wget commands keep file names literal in a shell",
  { skip: process.platform === "win32" && "no POSIX shell" }, () => {
    const { ctx } = load();
    vm.runInContext(readFileSync(OVERLAY_JS, "utf8"), ctx, { filename: OVERLAY_JS });
    let copied = "";
    ctx.copyTextToClipboard = (btn, text) => { copied = text; };
    // argv returns the arguments sh makes of the command after its name
    const argv = command =>
      execFileSync("sh", ["-c", `set -- ${command}; shift; printf '%s\\0' "$@"`])
        .toString().split("\0").slice(0, -1);
    const names = ["$(echo pwned).mp4", "a`id`b", 'say "hi".txt', "it's $HOME!.zip",
      "back\\slash", "line\nbreak", "plain name.txt"];
    for (const name of names) {
      const url = `http://127.0.0.1:8000/${encodeURIComponent(name)}?token=t&x=1`;
      ctx.copyCurlCommand({}, url, name);
      assert.deepEqual(argv(copied), ["-o", name, "-L", url], copied);
      ctx.copyWgetCommand({}, url, name);
      assert.deepEqual(argv(copied), ["-O", name, url], copied);
    }
    ctx.copyWgetCommand({}, "http://127.0.0.1:8000/$(id)", "");
    assert.deepEqual(argv(copied), ["http://127.0.0.1:8000/$(id)"], copied);
  });
