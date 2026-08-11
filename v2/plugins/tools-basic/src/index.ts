import { spawn } from "node:child_process";
import { readFile, realpath, writeFile } from "node:fs/promises";
import { dirname, isAbsolute, relative, resolve, sep } from "node:path";
import type { JsonValue, ToolDefinition } from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";

export interface BasicToolsConfig {
  cwd: string;
}

function objectInput(input: JsonValue): Record<string, JsonValue> {
  if (!input || Array.isArray(input) || typeof input !== "object") {
    throw new Error("tool input must be an object");
  }
  return input;
}

function stringField(input: JsonValue, name: string): string {
  const value = objectInput(input)[name];
  if (typeof value !== "string") throw new Error(`${name} must be a string`);
  return value;
}

function assertContained(root: string, path: string): void {
  const rel = relative(root, path);
  if (isAbsolute(rel) || rel === ".." || rel.startsWith(`..${sep}`)) {
    throw new Error("path escapes the workspace");
  }
}

async function workspacePath(cwd: string, inputPath: string, allowCreate = false): Promise<string> {
  const root = await realpath(cwd);
  const path = resolve(root, inputPath);
  assertContained(root, path);
  try {
    const target = await realpath(path);
    assertContained(root, target);
    return target;
  } catch (error) {
    if (!allowCreate || (error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
  }
  const parent = await realpath(dirname(path));
  assertContained(root, parent);
  return path;
}

function textResult(text: string, isError = false) {
  return { content: [{ type: "text" as const, text }], isError };
}

function readTool(cwd: string): ToolDefinition {
  return {
    name: "read",
    description: "Read a UTF-8 file in the current workspace.",
    access: "read",
    inputSchema: {
      type: "object",
      properties: { path: { type: "string" } },
      required: ["path"],
      additionalProperties: false,
    },
    async execute(input, execution) {
      execution.signal.throwIfAborted();
      const path = await workspacePath(cwd, stringField(input, "path"));
      execution.signal.throwIfAborted();
      return textResult(await readFile(path, { encoding: "utf8", signal: execution.signal }));
    },
  };
}

function writeTool(cwd: string): ToolDefinition {
  return {
    name: "write",
    description: "Write a UTF-8 file in the current workspace.",
    access: "write",
    inputSchema: {
      type: "object",
      properties: { path: { type: "string" }, content: { type: "string" } },
      required: ["path", "content"],
      additionalProperties: false,
    },
    async execute(input, execution) {
      execution.signal.throwIfAborted();
      const path = await workspacePath(cwd, stringField(input, "path"), true);
      execution.signal.throwIfAborted();
      await writeFile(path, stringField(input, "content"), {
        encoding: "utf8",
        signal: execution.signal,
      });
      return textResult(`wrote ${relative(cwd, path)}`);
    },
  };
}

function editTool(cwd: string): ToolDefinition {
  return {
    name: "edit",
    description: "Replace one exact UTF-8 text occurrence in a workspace file.",
    access: "write",
    inputSchema: {
      type: "object",
      properties: {
        path: { type: "string" },
        oldText: { type: "string" },
        newText: { type: "string" },
      },
      required: ["path", "oldText", "newText"],
      additionalProperties: false,
    },
    async execute(input, execution) {
      execution.signal.throwIfAborted();
      const path = await workspacePath(cwd, stringField(input, "path"));
      const oldText = stringField(input, "oldText");
      execution.signal.throwIfAborted();
      const content = await readFile(path, { encoding: "utf8", signal: execution.signal });
      const first = content.indexOf(oldText);
      if (first < 0) throw new Error("oldText was not found");
      if (content.indexOf(oldText, first + oldText.length) >= 0) {
        throw new Error("oldText is not unique");
      }
      execution.signal.throwIfAborted();
      await writeFile(
        path,
        content.slice(0, first) + stringField(input, "newText") + content.slice(first + oldText.length),
        { encoding: "utf8", signal: execution.signal },
      );
      return textResult(`edited ${relative(cwd, path)}`);
    },
  };
}

function bashTool(cwd: string): ToolDefinition {
  return {
    name: "bash",
    description: "Run a shell command in the current workspace.",
    access: "execute",
    inputSchema: {
      type: "object",
      properties: { command: { type: "string" } },
      required: ["command"],
      additionalProperties: false,
    },
    execute(input, execution) {
      const command = stringField(input, "command");
      return new Promise((resolveResult) => {
        const child = spawn(command, {
          cwd,
          shell: true,
          signal: execution.signal,
          stdio: ["ignore", "pipe", "pipe"],
        });
        let output = "";
        const collect = (chunk: Buffer) => {
          if (output.length < 1_000_000) output += chunk.toString("utf8");
        };
        child.stdout.on("data", collect);
        child.stderr.on("data", collect);
        child.once("error", (error) => resolveResult(textResult(error.message, true)));
        child.once("close", (code) => {
          resolveResult(textResult(output || `process exited with code ${code}`, code !== 0));
        });
      });
    },
  };
}

export const basicToolsPlugin: Plugin<BasicToolsConfig> = function toolsBasic(
  ctx: Context,
  config: BasicToolsConfig,
) {
  for (const tool of [readTool(config.cwd), writeTool(config.cwd), editTool(config.cwd), bashTool(config.cwd)]) {
    ctx.tools.register(tool.name, tool);
  }
};

basicToolsPlugin.inject = ["tools"];
