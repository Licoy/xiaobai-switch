import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  missingReleaseTargets,
  packageManagerVersion,
  parsePnpmActionSetups,
  parseReleaseRustTargets,
  pnpmSetupConflictsWithPackageManager,
  REQUIRED_RELEASE_TARGETS,
} from "./packagingWorkflow";

const root = resolve(import.meta.dirname, "../..");

function readRepoFile(rel: string): string {
  return readFileSync(resolve(root, rel), "utf8");
}

describe("packageManagerVersion", () => {
  it("strips the pnpm@ prefix from package.json packageManager", () => {
    const pkg = JSON.parse(readRepoFile("package.json")) as {
      packageManager: string;
    };
    expect(pkg.packageManager.startsWith("pnpm@")).toBe(true);
    expect(packageManagerVersion(pkg.packageManager)).toBe(
      pkg.packageManager.slice("pnpm@".length),
    );
  });
});

describe("parsePnpmActionSetups", () => {
  it("detects the dual-version conflict that failed Actions run 31116820861", () => {
    const pkg = JSON.parse(readRepoFile("package.json")) as {
      packageManager: string;
    };
    const failingCi = `
jobs:
  frontend:
    steps:
      - uses: pnpm/action-setup@v4
        with:
          version: 10
      - uses: actions/setup-node@v4
`;
    const setups = parsePnpmActionSetups(failingCi);
    expect(setups).toEqual([{ versionInput: "10" }]);
    expect(pnpmSetupConflictsWithPackageManager(setups, pkg.packageManager)).toBe(
      true,
    );
  });

  it("treats a missing version input as using packageManager", () => {
    const pkg = JSON.parse(readRepoFile("package.json")) as {
      packageManager: string;
    };
    const yaml = `
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
`;
    const setups = parsePnpmActionSetups(yaml);
    expect(setups).toEqual([{ versionInput: undefined }]);
    expect(pnpmSetupConflictsWithPackageManager(setups, pkg.packageManager)).toBe(
      false,
    );
  });
});

describe("shipped GitHub workflows", () => {
  it("does not pin a conflicting pnpm version in CI or Release", () => {
    const pkg = JSON.parse(readRepoFile("package.json")) as {
      packageManager: string;
    };
    const ci = readRepoFile(".github/workflows/ci.yml");
    const release = readRepoFile(".github/workflows/release.yml");

    const ciSetups = parsePnpmActionSetups(ci);
    const releaseSetups = parsePnpmActionSetups(release);

    expect(ciSetups.length).toBeGreaterThan(0);
    expect(releaseSetups.length).toBeGreaterThan(0);
    expect(pnpmSetupConflictsWithPackageManager(ciSetups, pkg.packageManager)).toBe(
      false,
    );
    expect(
      pnpmSetupConflictsWithPackageManager(releaseSetups, pkg.packageManager),
    ).toBe(false);
  });

  it("Release matrix covers macOS arm64/intel and Windows x64/arm64", () => {
    const release = readRepoFile(".github/workflows/release.yml");
    expect(release).toMatch(/^name:\s*Release\s*$/m);
    expect(release).toMatch(/tags:\s*\n\s+-\s+"v0\.\*\.\*"/);

    const targets = parseReleaseRustTargets(release);
    expect(missingReleaseTargets(targets)).toEqual([]);
    for (const target of REQUIRED_RELEASE_TARGETS) {
      expect(targets).toContain(target);
    }
  });

  it("enables signed updater artifacts in tauri.conf", () => {
    const tauri = JSON.parse(readRepoFile("src-tauri/tauri.conf.json")) as {
      bundle: {
        createUpdaterArtifacts?: boolean;
        macOS?: { signingIdentity?: string | null };
      };
      plugins: { updater?: { pubkey?: string; endpoints?: string[] } };
    };
    expect(tauri.bundle.createUpdaterArtifacts).toBe(true);
    expect(tauri.plugins.updater?.endpoints).toEqual([
      "https://github.com/Licoy/xiaobai-switch/releases/latest/download/latest.json",
    ]);
    expect(tauri.plugins.updater?.pubkey).toMatch(/^dW50cnVzdGVk/);
    // "-" makes Tauri codesign the .app bundle. Without it, only the linker
    // ad-hoc-signs the Mach-O (Sealed Resources=none) and Gatekeeper says
    // the downloaded app is damaged — Privacy & Security never shows Open Anyway.
    expect(tauri.bundle.macOS?.signingIdentity).toBe("-");
  });

  it("signs updater artifacts and publishes a combined latest.json", () => {
    const release = readRepoFile(".github/workflows/release.yml");
    expect(release).toMatch(/TAURI_SIGNING_PRIVATE_KEY/);
    expect(release).toMatch(/includeUpdaterJson:\s*false/);
    expect(release).toMatch(/scripts\/generate-updater-manifest\.mjs/);
    expect(release.indexOf("run: node scripts/generate-updater-manifest.mjs")).toBeLessThan(
      release.indexOf("- name: Publish draft release"),
    );
    expect(release).toMatch(/scripts\/validate-updater-signing-secret\.mjs/);
    expect(release).toMatch(/codesign --verify --deep --strict/);
  });

  it("generates release notes from git-cliff instead of a static body", () => {
    const release = readRepoFile(".github/workflows/release.yml");
    const cliff = readRepoFile("cliff.toml");

    expect(release).toMatch(/^\s+draft-release-notes:\s*$/m);
    expect(release).toMatch(/orhun\/git-cliff-action@v4/);
    expect(release).toMatch(/args:\s*--latest --strip header --offline/);
    expect(release).toMatch(/fetch-depth:\s*0/);
    expect(release).toMatch(
      /needs:\s*\[check-version,\s*draft-release-notes\]/,
    );
    expect(release).toMatch(
      /releaseBody:\s*\$\{\{\s*needs\.draft-release-notes\.outputs\.body\s*\}\}/,
    );
    expect(release).not.toMatch(/Built from tag/);

    expect(cliff).toMatch(/conventional_commits = true/);
    expect(cliff).toMatch(/\^feat/);
    expect(cliff).toMatch(/\^fix/);
    expect(cliff).toContain("^chore\\\\(version\\\\)");
    expect(cliff).toMatch(/xattr -cr \/Applications\/XiaoBaiSwitch\.app/);
    expect(cliff).toMatch(/updater-notes-end/);
  });

  it("does not use the PowerShell Join-Path trailing-comma pitfall in the portable zip step", () => {
    const release = readRepoFile(".github/workflows/release.yml");
    // `Join-Path $dir "a.exe", Join-Path $dir "b.exe"` is one call, not two paths.
    expect(release).not.toMatch(
      /Join-Path \$releaseDir ["'](?:xiaobai-switch|XiaoBaiSwitch)\.exe["'],/,
    );
    expect(release).toMatch(
      /foreach \(\$name in @\(["']XiaoBaiSwitch\.exe["']/,
    );
  });

  it("separates legacy bridge tags from all six signed native release targets", () => {
    const release = readRepoFile(".github/workflows/native-release.yml");
    const check = readRepoFile(".github/workflows/native-check.yml");
    expect(release).toMatch(/^name:\s*Native Release\s*$/m);
    expect(release).toContain("tags: ['v[1-9]*.*.*']");
    for (const target of ["darwin-amd64", "darwin-arm64", "windows-amd64", "windows-arm64", "linux-amd64", "linux-arm64"]) {
      expect(release).toContain(`target: ${target}`);
    }
    for (const workflow of [release, check]) {
      expect(workflow).toContain("go-version: 1.27.1");
    }
    expect(release).toContain("MYGO_UPDATER_PRIVATE_KEY");
    expect(release).toContain("gh release download v0.0.12 --pattern latest.json");
    expect(release).toContain("node scripts/native-transition.mjs");
    expect(check).toContain("node scripts/native-check-installer.mjs");
  });

  it("rebuilds the website after Release publishes, not on the version tag", () => {
    const website = readRepoFile(".github/workflows/website.yml");
    const release = readRepoFile(".github/workflows/release.yml");

    // Tag pushes race installer uploads. GITHUB_TOKEN also cannot fire
    // `on: release` for other workflows, so Website listens to Release
    // completing instead of polling from a tag job.
    expect(website).not.toMatch(/tags:\s*\n\s+-\s+"v\*\.\*\.\*"/);
    expect(website).not.toMatch(/Wait for published GitHub release/);
    expect(website).toMatch(
      /workflow_run:\s*\n\s+workflows:\s*\n\s+-\s+Release\s*\n\s+-\s+Native Release\s*\n\s+types:\s*\n\s+-\s+completed/,
    );
    expect(website).toMatch(/workflow_dispatch:/);
    expect(website).toMatch(/release_tag:/);
    expect(website).toMatch(/RELEASE_TAG:/);
    expect(website).not.toMatch(/RELEASE_TAG:.*head_branch/);
    expect(website).not.toMatch(/contains\(github\.event\.workflow_run\.head_branch/);
    expect(website).toContain("github.event_name == 'workflow_dispatch' && github.event.inputs.release_tag || ''");
    expect(website).toMatch(
      /github\.event_name == 'workflow_run' \|\| github\.event_name == 'workflow_dispatch'/,
    );
    expect(release).toMatch(/name:\s*Publish Release/);
  });

  it("bakes download links at build time and does not call GitHub from the browser", () => {
    const page = readRepoFile("website/src/templates/DownloadPage.astro");
    const websiteTests = readRepoFile("src/lib/websiteReleases.test.ts");

    expect(page).toMatch(/loadLatestRelease\(process\.env\.GITHUB_TOKEN/);
    expect(page).not.toMatch(/\bfetch\s*\(/);
    expect(page).not.toMatch(/GITHUB_API_LATEST/);
    expect(websiteTests).not.toMatch(/website\/src/);
  });
});
