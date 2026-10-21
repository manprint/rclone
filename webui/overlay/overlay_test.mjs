// Runs index.js and then overlay.js in a V8 context with just enough of a
// browser, as the page does, and checks the function the overlay replaces.
//
// Run with: node --test overlay_test.mjs

import assert from "node:assert/strict";
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
