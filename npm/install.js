// postinstall: download the correct wuu binary for this platform.
const https = require("https");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");
const os = require("os");

const REPO = "blueberrycongee/wuu";

function getPlatform() {
  const p = process.platform;
  if (p === "darwin") return "darwin";
  if (p === "linux") return "linux";
  throw new Error(`Unsupported platform: ${p}`);
}

function getArch() {
  const a = process.arch;
  if (a === "x64") return "amd64";
  if (a === "arm64") return "arm64";
  throw new Error(`Unsupported architecture: ${a}`);
}

function fetchJSON(url) {
  return new Promise((resolve, reject) => {
    https
      .get(url, { headers: { "User-Agent": "wuu-npm-installer" } }, (res) => {
        if (
          res.statusCode >= 300 &&
          res.statusCode < 400 &&
          res.headers.location
        ) {
          return fetchJSON(res.headers.location).then(resolve, reject);
        }
        let data = "";
        res.on("data", (chunk) => (data += chunk));
        res.on("end", () => {
          try {
            resolve(JSON.parse(data));
          } catch (e) {
            reject(e);
          }
        });
        res.on("error", reject);
      })
      .on("error", reject);
  });
}

async function main() {
  const platform = getPlatform();
  const arch = getArch();

  console.log(`Installing wuu for ${platform}/${arch}...`);

  // Get latest release.
  const release = await fetchJSON(
    `https://api.github.com/repos/${REPO}/releases/latest`,
  );
  const version = release.tag_name.replace(/^v/, "");
  const filename = `wuu_${version}_${platform}_${arch}.tar.gz`;
  const asset = release.assets.find((a) => a.name === filename);
  if (!asset) {
    throw new Error(
      `No binary found for ${platform}/${arch} in release ${release.tag_name}`,
    );
  }

  // Download to temp.
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "wuu-"));
  const tarPath = path.join(tmpDir, filename);

  console.log(`Downloading ${asset.browser_download_url}...`);
  execSync(`curl -fsSL "${asset.browser_download_url}" -o "${tarPath}"`);

  // Extract.
  execSync(`tar -xzf "${tarPath}" -C "${tmpDir}"`);

  // Move binary.
  const binDir = path.join(__dirname, "bin");
  fs.mkdirSync(binDir, { recursive: true });
  const dest = path.join(binDir, "wuu-bin");
  fs.copyFileSync(path.join(tmpDir, "wuu"), dest);
  fs.chmodSync(dest, 0o755);

  // Cleanup.
  fs.rmSync(tmpDir, { recursive: true, force: true });

  console.log(`wuu v${version} installed successfully.`);
}

main().catch((err) => {
  console.error("Failed to install wuu:", err.message);
  process.exit(1);
});
