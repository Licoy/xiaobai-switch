import { afterEach, describe, expect, it, vi } from "vitest";
import {
  GITHUB_API_LATEST,
  classifyAsset,
  assetsFromRelease,
  githubReleaseApiUrl,
  loadLatestRelease,
} from "./githubRelease";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe("githubReleaseApiUrl", () => {
  it("uses /releases/latest when no tag is provided", () => {
    expect(githubReleaseApiUrl()).toBe(GITHUB_API_LATEST);
    expect(githubReleaseApiUrl("")).toBe(GITHUB_API_LATEST);
    expect(githubReleaseApiUrl("   ")).toBe(GITHUB_API_LATEST);
  });

  it("uses /releases/tags/{tag} so a post-release website build can pin the new version", () => {
    expect(githubReleaseApiUrl("v0.0.6")).toBe(
      "https://api.github.com/repos/Licoy/xiaobai-switch/releases/tags/v0.0.6",
    );
  });

  it("strips refs/tags/ from workflow_run head_branch", () => {
    expect(githubReleaseApiUrl("refs/tags/v0.0.6")).toBe(
      "https://api.github.com/repos/Licoy/xiaobai-switch/releases/tags/v0.0.6",
    );
  });
});

describe("loadLatestRelease", () => {
  it("requests the tagged release when a tag is passed", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        tag_name: "v0.0.6",
        draft: false,
        assets: [
          {
            name: "XiaoBaiSwitch_0.0.6_aarch64.dmg",
            browser_download_url:
              "https://github.com/Licoy/xiaobai-switch/releases/download/v0.0.6/XiaoBaiSwitch_0.0.6_aarch64.dmg",
          },
        ],
      }),
    });
    vi.stubGlobal("fetch", fetchMock);

    const latest = await loadLatestRelease("token", "v0.0.6");

    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.github.com/repos/Licoy/xiaobai-switch/releases/tags/v0.0.6",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer token",
        }),
      }),
    );
    expect(latest?.tag).toBe("v0.0.6");
    expect(latest?.assets["mac-arm"]?.url).toContain("v0.0.6");
  });

  it("does not bake a draft release into the download page", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ tag_name: "v0.0.6", draft: true, assets: [] }),
      }),
    );

    await expect(loadLatestRelease(undefined, "v0.0.6")).rejects.toThrow(
      /draft/i,
    );
  });

  it("fails CI instead of shipping a download page without installer assets", async () => {
    vi.stubEnv("CI", "true");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ tag_name: "v0.0.6", draft: false, assets: [] }),
      }),
    );

    await expect(loadLatestRelease("token")).rejects.toThrow(/asset/i);
  });
});

describe("release download assets", () => {
  it.each([
    ["XiaoBaiSwitch_1.0.0_darwin-amd64.dmg", "mac-intel"],
    ["XiaoBaiSwitch_1.0.0_darwin-arm64.dmg", "mac-arm"],
    ["XiaoBaiSwitch_1.0.0_windows-amd64-setup.exe", "win-x64-nsis"],
    ["XiaoBaiSwitch_1.0.0_windows-arm64-setup.exe", "win-arm-nsis"],
    ["XiaoBaiSwitch_1.0.0_windows-amd64.zip", "win-x64-zip"],
    ["XiaoBaiSwitch_1.0.0_windows-arm64.zip", "win-arm-zip"],
    ["XiaoBaiSwitch_1.0.0_linux-amd64.tar.gz", "linux-x64"],
    ["XiaoBaiSwitch_1.0.0_linux-arm64.tar.gz", "linux-arm"],
    ["XiaoBaiSwitch_0.0.12_aarch64.dmg", "mac-arm"],
    ["XiaoBaiSwitch_0.0.12_x64.dmg", "mac-intel"],
    ["XiaoBaiSwitch_0.0.12_x86_64.dmg", "mac-intel"],
    ["XiaoBaiSwitch_0.0.12_x64_en-US.msi", "win-x64-msi"],
    ["XiaoBaiSwitch_0.0.12_x64-setup.exe", "win-x64-nsis"],
    ["XiaoBaiSwitch_0.0.12_arm64-setup.exe", "win-arm-nsis"],
    ["XiaoBaiSwitch_0.0.12_windows-x64-portable.zip", "win-x64-zip"],
    ["XiaoBaiSwitch_0.0.12_windows-arm64-portable.zip", "win-arm-zip"],
  ])("classifies %s as %s", (name, kind) => {
    expect(classifyAsset(name)).toBe(kind);
  });

  it.each([
    "XiaoBaiSwitch_1.0.0_darwin-amd64.tar.gz",
    "XiaoBaiSwitch_1.0.0_windows-amd64.tar.gz",
    "XiaoBaiSwitch_1.0.0_darwin-arm64.app.tar.gz",
    "XiaoBaiSwitch_1.0.0_linux-amd64.tar.gz.sig",
    "update-linux-amd64.json", "latest.json", "delta.tar.gz", "source.zip",
    "XiaoBaiSwitch_1.0.0_linux-arm64.tar.gz.delta",
    "XiaoBaiSwitch_1.0.0_windows-arm64-setup.exe.sha256",
  ])("excludes updater or unrelated asset %s", (name) => {
    expect(classifyAsset(name)).toBeNull();
  });

  it("preserves an absent architecture instead of inventing a link", () => {
    const result = assetsFromRelease({ tag_name: "v1.0.0", assets: [{
      name: "XiaoBaiSwitch_1.0.0_linux-arm64.tar.gz", browser_download_url: "https://example.test/linux-arm64.tar.gz",
    }] });
    expect(result.tag).toBe("v1.0.0");
    expect(result.assets["linux-x64"]).toBeUndefined();
    expect(result.assets["linux-arm"]?.url).toBe("https://example.test/linux-arm64.tar.gz");
  });
});

describe("explicit release failures outside CI", () => {
  it("rejects HTTP failures instead of rendering an empty download page", async () => {
    vi.stubEnv("CI", "");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 403 }));
    await expect(loadLatestRelease()).rejects.toThrow("latest: 403");
  });
  it("preserves a network failure", async () => {
    vi.stubEnv("CI", "");
    const error = new Error("network unavailable");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(error));
    await expect(loadLatestRelease()).rejects.toBe(error);
  });
  it("rejects a release containing only updater archives", async () => {
    vi.stubEnv("CI", "");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({
      tag_name: "v1.0.0", assets: [{ name: "XiaoBaiSwitch_1.0.0_windows-amd64.tar.gz", browser_download_url: "https://example.test/update.tar.gz" }],
    }) }));
    await expect(loadLatestRelease()).rejects.toThrow("no installer assets");
  });
});
