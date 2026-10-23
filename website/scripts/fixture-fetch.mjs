// Explicit test preload, never imported by the website or its production build.
import { readFileSync } from "node:fs";
const payload = readFileSync(new URL(process.env.WEBSITE_TEST_MISSING ? "../testdata/release-v1-missing.json" : "../testdata/release-v1.json", import.meta.url), "utf8");
const original = globalThis.fetch;
globalThis.fetch = (input, options) => {
  const url = String(input);
  if (url.startsWith("https://api.github.com/repos/Licoy/xiaobai-switch/releases/")) {
    return Promise.resolve(new Response(payload, { headers: { "Content-Type": "application/json" } }));
  }
  return original(input, options);
};
