import { useState } from "react";

/** First-run pairing: paste the URI shown on the desktop's 设置 → 远程 page
 *  (or scan-free copy of the QR payload) and complete the sealed handshake. */

export function PairScreen({
  onPair,
  onDone,
}: {
  /** Returns the paired host name on success. */
  onPair: (uri: string, deviceName: string) => Promise<string>;
  onDone: () => void;
}): React.JSX.Element {
  const [uri, setUri] = useState("");
  const [name, setName] = useState("手机");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (): Promise<void> => {
    const trimmed = uri.trim();
    if (!trimmed) {
      setError("请先粘贴配对 URI");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await onPair(trimmed, name.trim() || "手机");
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  };

  return (
    <div className="pair">
      <div className="pair-hero">
        <h1>连接电脑上的 Wuu</h1>
        <p>手机作为遥控器：会话和 agent 都跑在你的电脑上</p>
      </div>

      <ol className="pair-steps">
        <li>
          <span className="step-num">1</span>
          <span>
            电脑上的 Wuu：打开<b>设置 → 远程</b>，开启远程访问，点击<b>显示配对码</b>
          </span>
        </li>
        <li>
          <span className="step-num">2</span>
          <span>复制二维码下方的配对 URI</span>
        </li>
        <li>
          <span className="step-num">3</span>
          <span>粘贴到下面并点击“配对”</span>
        </li>
      </ol>

      <div className="field">
        <label htmlFor="pair-uri">配对 URI</label>
        <textarea
          id="pair-uri"
          value={uri}
          onChange={(e) => setUri(e.target.value)}
          placeholder="wuu://pair/…"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
        />
      </div>

      <div className="field">
        <label htmlFor="pair-name">本机名称（显示在电脑的设备列表里）</label>
        <input
          id="pair-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={32}
        />
      </div>

      {error ? <div className="pair-error">{error}</div> : null}

      <button className="pair-submit" disabled={busy} onClick={() => void submit()}>
        {busy ? "配对中…" : "配对"}
      </button>
    </div>
  );
}
