import { Component, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import {
  ClientModuleSystem,
  Context,
  SlotOutlet,
  clientKernelPlugin,
} from "@wuu-v2/client-runtime";
import { arriveDefaultClientProfile } from "@wuu-v2/profile-default/client";
import "./shell.css";

const element = document.getElementById("root");
if (!element) throw new Error("Missing renderer root");
const root = createRoot(element);
const desktop = window.wuuV2;
let disposeClient: (() => Promise<void>) | undefined;
let handlingFailure = false;
let clientGeneration = 0;

function Diagnostic({ error, restarting = false }: { error: string; restarting?: boolean }) {
  return (
    <main className="shell-diagnostic">
      <section>
        <p className="shell-kicker">Wuu V2 Harness</p>
        <h1>Agent runtime is unavailable</h1>
        <p>{error}</p>
        <button type="button" disabled={restarting || !desktop} onClick={async () => {
          if (!desktop) return;
          clientGeneration += 1;
          const dispose = disposeClient;
          disposeClient = undefined;
          root.render(<Diagnostic error="Restarting Agent runtime…" restarting />);
          try {
            await dispose?.();
            const result = await desktop.restart();
            if (result.ready) window.location.reload();
            else root.render(<Diagnostic error={result.error ?? "Harness restart failed"} />);
          } catch (error) {
            root.render(<Diagnostic error={error instanceof Error ? error.message : String(error)} />);
          }
        }}>{restarting ? "Restarting…" : "Restart"}</button>
      </section>
    </main>
  );
}

class ShellErrorBoundary extends Component<
  { children: ReactNode; onError(error: unknown): void },
  { error?: string }
> {
  state: { error?: string } = {};

  static getDerivedStateFromError(error: unknown) {
    return { error: error instanceof Error ? error.message : String(error) };
  }

  componentDidCatch(error: unknown): void {
    console.error(error);
    this.props.onError(error);
  }

  render() {
    return this.state.error
      ? <Diagnostic error={this.state.error} />
      : this.props.children;
  }
}

root.render(<main className="shell-loading">Starting Wuu…</main>);

async function boot(generation: number): Promise<() => Promise<void>> {
  if (!desktop) throw new Error("Desktop bridge is unavailable");
  const result = await desktop.boot();
  if (!result.ready) throw new Error(result.error ?? "Harness failed to start");
  const client = new Context();
  const kernel = client.plugin(clientKernelPlugin);
  const modules = new ClientModuleSystem(client);
  let disconnectActions: (() => void) | undefined;
  let disconnectProjections: (() => void) | undefined;
  let disposed = false;
  const dispose = async () => {
    if (disposed) return;
    disposed = true;
    disconnectProjections?.();
    disconnectActions?.();
    await modules.dispose();
    await kernel.dispose();
    await client.fiber.dispose();
  };
  const assertCurrent = () => {
    if (generation !== clientGeneration) throw new Error("Client boot was superseded");
  };
  try {
    assertCurrent();
    await kernel.await();
    assertCurrent();
    arriveDefaultClientProfile(modules, result.manifest);
    await modules.activateAll(result.manifest.map(({ id }) => id));
    modules.auditReady();
    assertCurrent();
    disconnectActions = client.clientActions.connect((action, input) =>
      desktop.action(action, input));
    disconnectProjections = client.clientProjections.connect({
      follow: (sessionId, listener) => desktop.follow(sessionId, listener),
    });
    assertCurrent();
    root.render(
      <ShellErrorBoundary onError={() => {
        if (generation === clientGeneration) clientGeneration += 1;
        if (disposeClient === dispose) disposeClient = undefined;
        void dispose();
      }}>
        <SlotOutlet
          client={client}
          slot={client.slots.root}
          ownerProps={{ sessionId: result.sessionId }}
          empty={<Diagnostic error="No active layout plugin is available" />}
        />
      </ShellErrorBoundary>,
    );
    return dispose;
  } catch (error) {
    await dispose();
    throw error;
  }
}

const stopHarnessState = desktop?.onHarnessState((state) => {
  if (state.state !== "failed" || handlingFailure) return;
  handlingFailure = true;
  clientGeneration += 1;
  const dispose = disposeClient;
  disposeClient = undefined;
  root.render(<Diagnostic error={state.error} />);
  void (dispose ? dispose() : Promise.resolve()).finally(() => {
    handlingFailure = false;
  });
}) ?? (() => {});

async function startClient(): Promise<void> {
  const generation = ++clientGeneration;
  try {
    const dispose = await boot(generation);
    if (generation !== clientGeneration) {
      await dispose();
      return;
    }
    disposeClient = dispose;
  } catch (error) {
    if (generation !== clientGeneration) return;
    root.render(<Diagnostic error={error instanceof Error ? error.message : String(error)} />);
  }
}

if (desktop) await startClient();
else root.render(<Diagnostic error="Desktop bridge is unavailable" />);

window.addEventListener("pagehide", () => {
  clientGeneration += 1;
  stopHarnessState();
  root.unmount();
  void disposeClient?.();
  disposeClient = undefined;
}, { once: true });
