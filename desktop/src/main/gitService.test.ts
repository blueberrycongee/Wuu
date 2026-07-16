import { execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import type { RuntimeContext } from "../shared/protocol";
import { GitService, type GitCoauthor } from "./gitService";

const roots: string[] = [];

function makeRepository(): string {
  const root = mkdtempSync(join(tmpdir(), "wuu-git-service-"));
  roots.push(root);
  execFileSync("git", ["init", "-q", root]);
  execFileSync("git", ["-C", root, "config", "core.hooksPath", "/dev/null"]);
  execFileSync("git", ["-C", root, "config", "user.email", "test@example.com"]);
  execFileSync("git", ["-C", root, "config", "user.name", "Wuu Test"]);
  execFileSync("git", ["-C", root, "config", "commit.gpgsign", "false"]);
  writeFileSync(join(root, "README.md"), "workspace\n");
  execFileSync("git", ["-C", root, "add", "README.md"]);
  execFileSync("git", ["-C", root, "commit", "-qm", "initial"]);
  return root;
}

function serviceFor(root: string, coauthor?: GitCoauthor): GitService {
  const context: RuntimeContext = { kind: "no_project", cwd: root };
  return new GitService(() => context, () => coauthor);
}

afterEach(() => {
  for (const root of roots.splice(0)) {
    rmSync(root, { recursive: true, force: true });
  }
});

describe("GitService file previews", () => {
  it("returns ignored text files as complete new-file previews", () => {
    const root = makeRepository();
    writeFileSync(join(root, ".gitignore"), "/docs/plans/*\n");
    mkdirSync(join(root, "docs/plans"), { recursive: true });
    writeFileSync(join(root, "docs/plans/brief.md"), "# Brief\n\nBody\n");

    const result = serviceFor(root).fileDiff("docs/plans/brief.md");

    expect(result.status).toBe("ignored");
    expect(result.additions).toBe(3);
    expect(result.patch).toContain("new file mode 100644");
    expect(result.patch).toContain("+# Brief");
    expect(result.patch).toContain("+Body");
  });

  it("keeps ignored files out of the workspace Git change list", () => {
    const root = makeRepository();
    writeFileSync(join(root, ".gitignore"), "ignored.txt\n");
    writeFileSync(join(root, "ignored.txt"), "private draft\n");

    expect(
      serviceFor(root)
        .changes()
        .files.map((file) => file.path),
    ).not.toContain("ignored.txt");
  });
});

describe("GitService commit attribution", () => {
  it("adds an enabled agent identity as a co-author trailer", () => {
    const root = makeRepository();
    writeFileSync(join(root, "README.md"), "updated workspace\n");

    serviceFor(root, {
      name: "WUU Agent",
      email: "123+wuu-agent@users.noreply.github.com",
    }).commit({ message: "Update workspace" });

    const message = execFileSync(
      "git",
      ["-C", root, "log", "-1", "--format=%B"],
      { encoding: "utf8" },
    );
    expect(message).toBe(
      "Update workspace\n\nCo-authored-by: WUU Agent <123+wuu-agent@users.noreply.github.com>\n\n",
    );
  });

  it("keeps the existing commit message when no co-author is configured", () => {
    const root = makeRepository();
    writeFileSync(join(root, "README.md"), "updated workspace\n");

    serviceFor(root).commit({ message: "Update workspace" });

    const message = execFileSync(
      "git",
      ["-C", root, "log", "-1", "--format=%B"],
      { encoding: "utf8" },
    );
    expect(message).toBe("Update workspace\n\n");
  });

  it("rejects malformed co-author data before committing", () => {
    const root = makeRepository();
    writeFileSync(join(root, "README.md"), "updated workspace\n");

    expect(() =>
      serviceFor(root, {
        name: "WUU Agent\nSigned-off-by: forged",
        email: "wuu@example.com",
      }).commit({ message: "Update workspace" }),
    ).toThrow("git co-author name is invalid");
    expect(
      execFileSync("git", ["-C", root, "rev-list", "--count", "HEAD"], {
        encoding: "utf8",
      }).trim(),
    ).toBe("1");
  });
});
