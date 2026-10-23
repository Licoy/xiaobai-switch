export const GITHUB_API_LATEST =
  "https://api.github.com/repos/Licoy/xiaobai-switch/releases/latest";

export function githubReleaseApiUrl(tag?: string): string {
  const normalized = tag?.trim().replace(/^refs\/tags\//, "");
  if (!normalized) return GITHUB_API_LATEST;
  return `https://api.github.com/repos/Licoy/xiaobai-switch/releases/tags/${encodeURIComponent(normalized)}`;
}

export type AssetKind =
  | "mac-arm"
  | "mac-intel"
  | "win-x64-msi"
  | "win-x64-nsis"
  | "win-x64-zip"
  | "win-arm-nsis"
  | "win-arm-zip"
  | "linux-x64"
  | "linux-arm";

export type AssetMap = Partial<Record<AssetKind, { name: string; url: string }>>;

export type LatestRelease = {
  tag: string;
  assets: AssetMap;
};

export function classifyAsset(name: string): AssetKind | null {
  const n = name.toLowerCase();
  if (n.endsWith(".sig") || n.endsWith(".json") || n.includes(".app.tar.gz")) return null;
  const native = /^xiaobaiswitch_.+_(darwin|windows|linux)-(amd64|arm64)(\.dmg|\.zip|-setup\.exe|\.tar\.gz)$/.exec(n);
  if (native) {
    const [, os, arch, extension] = native;
    if (os === "darwin" && extension === ".dmg") return arch === "arm64" ? "mac-arm" : "mac-intel";
    if (os === "linux" && extension === ".tar.gz") return arch === "arm64" ? "linux-arm" : "linux-x64";
    if (os === "windows" && extension === ".zip") return arch === "arm64" ? "win-arm-zip" : "win-x64-zip";
    if (os === "windows" && extension === "-setup.exe") return arch === "arm64" ? "win-arm-nsis" : "win-x64-nsis";
    return null;
  }
  if (n.endsWith(".dmg") && /aarch64|arm64/.test(n)) return "mac-arm";
  if (n.endsWith(".dmg") && /(x64|x86_64|amd64)/.test(n)) return "mac-intel";
  if (n.endsWith(".msi") && n.includes("x64")) return "win-x64-msi";
  if (n.endsWith("x64-setup.exe")) return "win-x64-nsis";
  if (n.endsWith("arm64-setup.exe")) return "win-arm-nsis";
  if (n.endsWith("windows-x64-portable.zip")) return "win-x64-zip";
  if (n.endsWith("windows-arm64-portable.zip")) return "win-arm-zip";
  return null;
}

export function assetsFromRelease(release: {
  tag_name?: string;
  assets?: { name: string; browser_download_url: string }[];
}): LatestRelease {
  const assets: AssetMap = {};
  for (const asset of release.assets ?? []) {
    const kind = classifyAsset(asset.name);
    if (kind) assets[kind] = { name: asset.name, url: asset.browser_download_url };
  }
  return { tag: release.tag_name ?? "", assets };
}

export function hrefFor(assets: AssetMap, kind: AssetKind, fallback: string): string {
  return assets[kind]?.url ?? fallback;
}

type GithubReleasePayload = Parameters<typeof assetsFromRelease>[0] & {
  draft?: boolean;
};

export async function loadLatestRelease(
  token?: string,
  tag?: string,
): Promise<LatestRelease> {
  const headers: Record<string, string> = {
    Accept: "application/vnd.github+json",
    "User-Agent": "xiaobai-switch-website",
  };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(githubReleaseApiUrl(tag), { headers });
  if (!res.ok) {
    throw new Error(`Failed to load GitHub release ${tag?.trim() || "latest"}: ${res.status}`);
  }
  const payload = (await res.json()) as GithubReleasePayload;
  if (payload.draft) {
    throw new Error(`GitHub release ${payload.tag_name ?? tag ?? "unknown"} is still a draft`);
  }
  const latest = assetsFromRelease(payload);
  if (Object.keys(latest.assets).length === 0) {
    throw new Error(`GitHub release ${latest.tag || tag || "latest"} has no installer assets`);
  }
  return latest;
}
