import { lazy, Suspense, useEffect, useRef, useState } from "react";
import type { Credentials } from "@wuu/remote-core";

import { webCredStore } from "./lib/credStore";
import { RemoteDesktopBridge } from "./lib/desktopBridge";

const SharedWorkbench = lazy(() => import("./WebWorkspace"));

type Phase =
  | { kind: "boot" }
  | { kind: "pair"; error?: string }
  | { kind: "connecting" }
  | { kind: "ready" }
  | { kind: "error"; message: string };

export default function App(): React.JSX.Element {
  const [phase, setPhase] = useState<Phase>({ kind: "boot" });
  const bridgeRef = useRef<RemoteDesktopBridge | null>(null);

  const connect = async (credentials: Credentials): Promise<void> => {
    setPhase({ kind: "connecting" });
    const bridge = new RemoteDesktopBridge(credentials);
    bridgeRef.current = bridge;
    try {
      await bridge.connect();
      bridge.install();
      setPhase({ kind: "ready" });
    } catch (error) {
      await bridge.disconnect().catch(() => {});
      bridgeRef.current = null;
      setPhase({
        kind: "error",
        message: error instanceof Error ? error.message : "无法连接到 Wuu",
      });
    }
  };

  useEffect(() => {
    let active = true;
    void webCredStore.load().then((credentials) => {
      if (!active) return;
      if (credentials) void connect(credentials);
      else setPhase({ kind: "pair" });
    });
    return () => {
      active = false;
    };
  }, []);

  if (phase.kind === "ready") {
    return (
      <Suspense fallback={<StatusCard title="正在载入工作台…" />}>
        <SharedWorkbench />
      </Suspense>
    );
  }

  if (phase.kind === "pair") {
    return (
      <PairCard
        error={phase.error}
        onPair={async (uri, name) => {
          setPhase({ kind: "connecting" });
          try {
            const credentials = await RemoteDesktopBridge.pair(uri, name);
            await webCredStore.save(credentials);
            await connect(credentials);
          } catch (error) {
            setPhase({
              kind: "pair",
              error: error instanceof Error ? error.message : "配对失败",
            });
          }
        }}
      />
    );
  }

  if (phase.kind === "error") {
    return (
      <StatusCard title="连接失败" detail={phase.message}>
        <button
          type="button"
          onClick={() => {
            void webCredStore.clear().then(() => setPhase({ kind: "pair" }));
          }}
        >
          重新配对
        </button>
      </StatusCard>
    );
  }

  return <StatusCard title={phase.kind === "boot" ? "正在启动…" : "正在连接电脑…"} />;
}

function PairCard({
  error,
  onPair,
}: {
  error?: string;
  onPair: (uri: string, name: string) => Promise<void>;
}): React.JSX.Element {
  const [uri, setURI] = useState("");
  const [name, setName] = useState("Web Browser");
  return (
    <main className="web-gate">
      <section className="web-gate-card">
        <p className="web-gate-kicker">WUU / WEB</p>
        <h1>连接你的工作台</h1>
        <p className="web-gate-copy">Agent 继续运行在电脑上，浏览器只承载同一套 Wuu 工作界面。</p>
        <label>
          <span>设备名称</span>
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </label>
        <label>
          <span>配对 URI</span>
          <textarea
            value={uri}
            onChange={(event) => setURI(event.target.value)}
            placeholder="wuu://pair?…"
            rows={4}
          />
        </label>
        {error ? <p className="web-gate-error">{error}</p> : null}
        <button
          type="button"
          disabled={!uri.trim() || !name.trim()}
          onClick={() => void onPair(uri.trim(), name.trim())}
        >
          配对并进入
        </button>
      </section>
    </main>
  );
}

function StatusCard({
  title,
  detail,
  children,
}: {
  title: string;
  detail?: string;
  children?: React.ReactNode;
}): React.JSX.Element {
  return (
    <main className="web-gate">
      <section className="web-gate-card web-gate-status">
        <p className="web-gate-kicker">WUU / WEB</p>
        <h1>{title}</h1>
        {detail ? <p className="web-gate-error">{detail}</p> : null}
        {children}
      </section>
    </main>
  );
}
