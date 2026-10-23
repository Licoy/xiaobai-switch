import { generateKeyPairSync, createHash, sign } from "node:crypto";
import { afterEach, describe, expect, it, vi } from "vitest";

const manifestModule =
  // @ts-expect-error The workflow helper is native ESM without TypeScript declarations.
  await import("../../scripts/generate-updater-manifest.mjs");
const {
  buildUpdaterManifest,
  classifySignature,
  updaterNotesFromReleaseBody,
  main,
} = manifestModule;

function assetPair(name: string, id: number) {
  return [
    { id, name },
    { id: id + 1, name: `${name}.sig` },
  ];
}

const assets = [
  ...assetPair("XiaoBaiSwitch_aarch64.app.tar.gz", 1),
  ...assetPair("XiaoBaiSwitch_x64.app.tar.gz", 3),
  ...assetPair("XiaoBaiSwitch_0.0.1_arm64-setup.exe", 5),
  ...assetPair("XiaoBaiSwitch_0.0.1_x64_en-US.msi", 7),
  ...assetPair("XiaoBaiSwitch_0.0.1_x64-setup.exe", 9),
];

const signatures = new Map(
  assets
    .filter(({ name }) => name.endsWith(".sig"))
    .map(({ name }) => [name, `signature:${name}`]),
);

describe("updater notes from release body", () => {
  it("keeps changelog and drops the install footer after the marker", () => {
    expect(
      updaterNotesFromReleaseBody(`## 更新内容

### 🐛 Bug 修复
- **updater**: 检测更新走应用代理规则

<!-- updater-notes-end -->

## 下载
- **macOS（Apple Silicon）**: \`.dmg\`
`),
    ).toBe(`## 更新内容

### 🐛 Bug 修复
- **updater**: 检测更新走应用代理规则`);
  });

  it("returns the whole body when the marker is missing", () => {
    expect(updaterNotesFromReleaseBody("just notes")).toBe("just notes");
    expect(updaterNotesFromReleaseBody("")).toBe("");
  });
});

describe("updater manifest generation", () => {
  it("classifies supported updater signature assets", () => {
    expect(classifySignature("XiaoBaiSwitch_aarch64.app.tar.gz.sig")).toEqual({
      os: "darwin",
      arch: "aarch64",
      bundle: "app",
    });
    expect(classifySignature("XiaoBaiSwitch_0.0.1_x64-setup.exe.sig")).toEqual({
      os: "windows",
      arch: "x86_64",
      bundle: "nsis",
    });
    expect(classifySignature("XiaoBaiSwitch_v0.0.1_windows-x64-portable.zip.sig")).toBeNull();
  });

  it("builds one complete updater manifest after all platform uploads", () => {
    const manifest = buildUpdaterManifest({
      version: "0.0.1",
      notes: "release notes",
      pubDate: "2026-08-19T15:00:00.000Z",
      repository: "Licoy/xiaobai-switch",
      serverUrl: "https://github.com",
      tag: "v0.0.1",
      assets,
      signatures,
    });

    expect(manifest.version).toBe("0.0.1");
    expect(manifest.notes).toBe("release notes");
    expect(manifest.platforms["windows-aarch64"].url).toBe(
      "https://github.com/Licoy/xiaobai-switch/releases/download/v0.0.1/XiaoBaiSwitch_0.0.1_arm64-setup.exe",
    );
    expect(manifest.platforms["windows-aarch64-nsis"].signature).toBe(
      "signature:XiaoBaiSwitch_0.0.1_arm64-setup.exe.sig",
    );
    expect(manifest.platforms["darwin-x86_64-app"].url).toBe(
      "https://github.com/Licoy/xiaobai-switch/releases/download/v0.0.1/XiaoBaiSwitch_x64.app.tar.gz",
    );
    expect(manifest.platforms["windows-x86_64"].url).toBe(
      "https://github.com/Licoy/xiaobai-switch/releases/download/v0.0.1/XiaoBaiSwitch_0.0.1_x64_en-US.msi",
    );
    expect(Object.keys(manifest.platforms)).toHaveLength(9);
  });

  it("fails instead of publishing an incomplete updater manifest", () => {
    expect(() =>
      buildUpdaterManifest({
        version: "0.0.1",
        notes: "",
        pubDate: "2026-08-19T15:00:00.000Z",
        repository: "Licoy/xiaobai-switch",
        serverUrl: "https://github.com",
        tag: "v0.0.1",
        assets: assets.filter(({ name }) => !name.includes("XiaoBaiSwitch_x64.app.tar.gz")),
        signatures,
      }),
    ).toThrow("missing updater platforms: darwin-x86_64, darwin-x86_64-app");
  });
});

// Exercise the actual draft upload path without contacting GitHub or reading credentials.
describe("bridge updater publication", () => {
  afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals(); });

  function publication(tag: string, failAlias = false) {
    vi.stubEnv("GITHUB_TOKEN", "fixture-token");
    vi.stubEnv("GITHUB_REPOSITORY", "Licoy/xiaobai-switch");
    vi.stubEnv("GITHUB_REF_NAME", tag);
    vi.stubEnv("GITHUB_API_URL", "https://api.example.test");
    vi.stubEnv("GITHUB_SERVER_URL", "https://github.com");
    const { publicKey, privateKey } = generateKeyPairSync("ed25519");
    const pub = publicKey.export({ type: "spki", format: "der" }).subarray(-32);
    const payload = Buffer.from("installer fixture"), id = Buffer.alloc(8);
    const sig = sign(null, createHash("blake2b512").update(payload).digest(), privateKey);
    const packet = Buffer.concat([Buffer.from("ED"), id, sig]);
    const comment = "fixture";
    const signature = Buffer.from(`untrusted comment: fixture\n${packet.toString("base64")}\ntrusted comment: ${comment}\n${sign(null, Buffer.concat([sig, Buffer.from(comment)]), privateKey).toString("base64")}\n`).toString("base64");
    const tauriPublicKey = Buffer.from(`untrusted comment: fixture\n${Buffer.concat([Buffer.from("Ed"), id, pub]).toString("base64")}\n`).toString("base64");
    const uploads = new Map<string, string>();
    vi.stubGlobal("fetch", vi.fn(async (url: string, options: RequestInit) => {
      if (options.method === "POST") {
        const name = new URL(url).searchParams.get("name")!;
        if (failAlias && name === "native-transition.json") return new Response("upload failed", { status: 500 });
        uploads.set(name, options.body as string);
        return Response.json({});
      }
      if (url.includes("/releases?")) return Response.json([{ id: 123, tag_name: tag, draft: true, body: "Bridge release", upload_url: "https://uploads.example.test/assets{?name,label}" }]);
      if (url.includes("/releases/123/assets?")) return Response.json(assets);
      if (/\/releases\/assets\/\d+$/.test(url)) return new Response(signature);
      throw new Error(`Unexpected GitHub request: ${url}`);
    }));
    return { uploads, payload, tauriPublicKey, signature };
  }

  it.each(["v0.0.11", "v0.0.12"])("publishes a complete transition alias only for bridge %s", async (tag) => {
    const fixture = publication(tag);
    await main();
    const manifest = JSON.parse(fixture.uploads.get("latest.json")!);
    expect(manifest.version).toBe(tag.slice(1));
    expect(Object.keys(manifest.platforms)).toHaveLength(9);
    if (tag === "v0.0.12") {
      expect(fixture.uploads.get("native-transition.json")).toBe(fixture.uploads.get("latest.json"));
      // @ts-expect-error The release helper is native ESM without TypeScript declarations.
      const { verifyTauriSignature } = await import("../../scripts/native-release-lib.mjs");
      for (const platform of Object.values(manifest.platforms) as { signature: string; url: string }[]) {
        expect(platform.url).toContain("/releases/download/v0.0.12/");
        expect(platform.signature).toBe(fixture.signature);
        expect(verifyTauriSignature(fixture.payload, platform.signature, fixture.tauriPublicKey)).toBe(true);
      }
    } else {
      expect([...fixture.uploads.keys()]).toEqual(["latest.json"]);
    }
  });

  it("fails the publication step if the bridge alias cannot upload", async () => {
    publication("v0.0.12", true);
    await expect(main()).rejects.toThrow("500 upload failed");
  });
});
