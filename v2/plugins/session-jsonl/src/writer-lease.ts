import { createHash } from "node:crypto";
import { chmod, mkdir, rm } from "node:fs/promises";
import { createConnection, createServer, type Server } from "node:net";
import { tmpdir } from "node:os";
import { resolve } from "node:path";

export interface WriterLease {
  release(): Promise<void>;
}

function leaseEndpoint(directory: string): string {
  const digest = createHash("sha256").update(resolve(directory)).digest("hex").slice(0, 24);
  return process.platform === "win32"
    ? `\\\\.\\pipe\\wuu-v2-writer-${digest}`
    : `${tmpdir()}/wuu-v2-writer-${digest}.sock`;
}

function listen(server: Server, endpoint: string): Promise<void> {
  return new Promise((resolveListen, reject) => {
    const onError = (error: Error) => reject(error);
    server.once("error", onError);
    server.listen(endpoint, () => {
      server.off("error", onError);
      resolveListen();
    });
  });
}

function ownerIsListening(endpoint: string): Promise<boolean> {
  return new Promise((resolveProbe) => {
    const socket = createConnection(endpoint);
    socket.once("connect", () => {
      socket.destroy();
      resolveProbe(true);
    });
    socket.once("error", (error: NodeJS.ErrnoException) => {
      socket.destroy();
      resolveProbe(!["ECONNREFUSED", "ENOENT"].includes(error.code ?? ""));
    });
  });
}

async function close(server: Server): Promise<void> {
  if (!server.listening) return;
  await new Promise<void>((resolveClose, reject) => {
    server.close((error) => error ? reject(error) : resolveClose());
  });
}

export async function acquireWriterLease(directory: string): Promise<WriterLease> {
  await mkdir(directory, { recursive: true });
  const endpoint = leaseEndpoint(directory);
  let server: Server | undefined;
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const candidate = createServer((socket) => socket.end());
    candidate.unref();
    try {
      await listen(candidate, endpoint);
      server = candidate;
      break;
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "EADDRINUSE") throw error;
      if (await ownerIsListening(endpoint)) {
        throw new Error("Session storage already has an active writer");
      }
      if (process.platform !== "win32") await rm(endpoint, { force: true });
    }
  }
  if (!server) throw new Error("Could not acquire the Session writer lease");
  try {
    if (process.platform !== "win32") await chmod(endpoint, 0o600);
  } catch (error) {
    await close(server);
    throw error;
  }

  let released = false;
  return {
    async release() {
      if (released) return;
      released = true;
      await close(server!);
    },
  };
}
