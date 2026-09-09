import * as React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ComposerCameraPanel } from "./ComposerCamera";

let container: HTMLDivElement;
let root: Root | null = null;
const mediaDevicesDescriptor = Object.getOwnPropertyDescriptor(navigator, "mediaDevices");

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  Object.defineProperty(navigator, "mediaDevices", { configurable: true, get: () => undefined });
  vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  root = null;
  container.remove();
  vi.restoreAllMocks();
  if (mediaDevicesDescriptor) Object.defineProperty(navigator, "mediaDevices", mediaDevicesDescriptor);
  else Reflect.deleteProperty(navigator, "mediaDevices");
});

function render(ui: React.ReactElement): void {
  act(() => {
    root = createRoot(container);
    root.render(ui);
  });
}

function fakeStream(): MediaStream {
  const stop = vi.fn();
  const track = { stop } as unknown as MediaStreamTrack;
  return { getTracks: () => [track] } as unknown as MediaStream;
}

function installGetUserMedia(
  getUserMedia: (constraints?: MediaStreamConstraints) => Promise<MediaStream>,
): void {
  vi.spyOn(navigator, "mediaDevices", "get").mockReturnValue({ getUserMedia } as MediaDevices);
}

describe("ComposerCamera", () => {
  it("shows the native capture fallback when getUserMedia is unavailable", () => {
    vi.spyOn(navigator, "mediaDevices", "get").mockReturnValue(undefined as unknown as MediaDevices);
    const onCapture = vi.fn();
    render(<ComposerCameraPanel onCapture={onCapture} onClose={() => {}} />);

    expect(container.querySelector(".composer-camera")).not.toBeNull();
    expect(container.querySelector(".composer-camera-error")).not.toBeNull();
    expect(container.querySelector('input[capture="environment"]')).not.toBeNull();
    const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
    const click = vi.spyOn(input, "click").mockImplementation(() => {});
    act(() => { container.querySelector<HTMLButtonElement>('[role="alert"] button')!.click(); });
    expect(click).toHaveBeenCalledOnce();
    act(() => { input.dispatchEvent(new Event("cancel", { bubbles: true })); });
    expect(onCapture).not.toHaveBeenCalled();
    const file = new File(["photo"], "photo.jpg", { type: "image/jpeg" });
    Object.defineProperty(input, "files", { value: [file], configurable: true });
    act(() => { input.dispatchEvent(new Event("change", { bubbles: true })); });
    expect(onCapture).toHaveBeenCalledWith(file);
  });

  it("opens the rear camera and stops tracks when closed", async () => {
    const stream = fakeStream();
    const getUserMedia = vi.fn().mockResolvedValue(stream);
    installGetUserMedia(getUserMedia);
    const onClose = vi.fn();

    render(<ComposerCameraPanel onCapture={() => {}} onClose={onClose} />);
    await act(async () => {});

    expect(getUserMedia).toHaveBeenCalledWith({
      video: { facingMode: "environment" },
      audio: false,
    });
    expect(container.querySelector("video")).not.toBeNull();

    const closeButton = container.querySelector<HTMLButtonElement>(".composer-camera-close");
    act(() => {
      closeButton?.click();
    });

    expect(stream.getTracks()[0].stop).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("drops a stream that resolves after the panel closes", async () => {
    let resolveStream: ((stream: MediaStream) => void) | undefined;
    const getUserMedia = vi.fn(
      () =>
        new Promise<MediaStream>((resolve) => {
          resolveStream = resolve;
        }),
    );
    installGetUserMedia(getUserMedia);

    render(<ComposerCameraPanel onCapture={() => {}} onClose={() => {}} />);
    act(() => {
      root?.unmount();
    });
    root = null;

    const stream = fakeStream();
    await act(async () => {
      resolveStream?.(stream);
    });

    expect(stream.getTracks()[0].stop).toHaveBeenCalledTimes(1);
  });

  it("shows the permission-denied fallback when the user refuses the camera", async () => {
    installGetUserMedia(
      vi.fn().mockRejectedValue(Object.assign(new Error("denied"), { name: "NotAllowedError" })),
    );

    render(<ComposerCameraPanel onCapture={() => {}} onClose={() => {}} />);
    await act(async () => {});

    expect(container.querySelector('[role="alert"] button')).not.toBeNull();
    expect(container.querySelector<HTMLButtonElement>(".composer-camera-shutter")!.disabled).toBe(true);
    expect(container.querySelector('input[capture="environment"]')).not.toBeNull();
  });

  it("captures the current frame into an attachment file", async () => {
    const drawImage = vi.fn();
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({
      drawImage,
    } as unknown as CanvasRenderingContext2D);
    const toBlob = vi.fn(function toBlob(
      this: HTMLCanvasElement,
      callback: BlobCallback,
    ): void {
      callback(new Blob(["jpeg"], { type: "image/jpeg" }));
    });
    vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation(toBlob);

    installGetUserMedia(vi.fn().mockResolvedValue(fakeStream()));
    const onCapture = vi.fn();

    render(<ComposerCameraPanel onCapture={onCapture} onClose={() => {}} />);
    await act(async () => {});

    const video = container.querySelector<HTMLVideoElement>("video");
    expect(video).not.toBeNull();
    Object.defineProperty(video as HTMLVideoElement, "readyState", {
      configurable: true,
      value: 4,
    });
    Object.defineProperty(video as HTMLVideoElement, "videoWidth", {
      configurable: true,
      value: 640,
    });
    Object.defineProperty(video as HTMLVideoElement, "videoHeight", {
      configurable: true,
      value: 480,
    });

    const shutter = container.querySelector<HTMLButtonElement>(".composer-camera-shutter");
    expect(shutter).not.toBeNull();
    expect(shutter?.disabled).toBe(false);

    await act(async () => {
      shutter?.click();
    });

    expect(drawImage).toHaveBeenCalledTimes(1);
    const file = onCapture.mock.calls[0][0] as File;
    expect(file).toBeInstanceOf(File);
    expect(file.type).toBe("image/jpeg");
  });

  it.each(["close", "unmount", "background"])("discards encoding after %s and blocks duplicate shutters", async (reason) => {
    const stream = fakeStream();
    installGetUserMedia(vi.fn().mockResolvedValue(stream));
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({ drawImage: vi.fn() } as unknown as CanvasRenderingContext2D);
    let finish!: BlobCallback;
    const encode = vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation((callback) => { finish = callback; });
    const onCapture = vi.fn();
    render(<ComposerCameraPanel onCapture={onCapture} onClose={() => {}} />);
    await act(async () => {});
    const video = container.querySelector("video")!;
    Object.defineProperties(video, {
      readyState: { value: 4 }, videoWidth: { value: 640 }, videoHeight: { value: 480 },
    });
    const shutter = container.querySelector<HTMLButtonElement>(".composer-camera-shutter")!;
    act(() => { shutter.click(); shutter.click(); });
    expect(encode).toHaveBeenCalledOnce();
    expect(shutter.disabled).toBe(true);
    act(() => {
      if (reason === "close") container.querySelector<HTMLButtonElement>(".composer-camera-close")!.click();
      else if (reason === "unmount") { root!.unmount(); root = null; }
      else window.dispatchEvent(new Event("pagehide"));
    });
    await act(async () => { finish(new Blob(["jpeg"], { type: "image/jpeg" })); });
    expect(stream.getTracks()[0].stop).toHaveBeenCalledOnce();
    expect(onCapture).not.toHaveBeenCalled();
  });

  it("stops the camera in the background and only resumes on request", async () => {
    const stream = fakeStream();
    const next = fakeStream();
    const getUserMedia = vi.fn().mockResolvedValueOnce(stream).mockResolvedValueOnce(next);
    installGetUserMedia(getUserMedia);
    render(<ComposerCameraPanel onCapture={() => {}} onClose={() => {}} />);
    await act(async () => {});
    expect(container.querySelector("video")!.srcObject).toBe(stream);
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    act(() => { document.dispatchEvent(new Event("visibilitychange")); });
    expect(stream.getTracks()[0].stop).toHaveBeenCalledOnce();
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    act(() => { document.dispatchEvent(new Event("visibilitychange")); });
    expect(getUserMedia).toHaveBeenCalledOnce();
    await act(async () => { container.querySelector<HTMLButtonElement>('[role="status"] button')!.click(); });
    expect(container.querySelector("video")!.srcObject).toBe(next);
  });

  it("stops a permission request that resolves while backgrounded", async () => {
    let resolve!: (stream: MediaStream) => void;
    installGetUserMedia(() => new Promise((done) => { resolve = done; }));
    render(<ComposerCameraPanel onCapture={() => {}} onClose={() => {}} />);
    act(() => { window.dispatchEvent(new Event("pagehide")); });
    const stream = fakeStream();
    await act(async () => { resolve(stream); });
    expect(stream.getTracks()[0].stop).toHaveBeenCalledOnce();
    expect(container.querySelector("video")).toBeNull();
    expect(container.querySelector('[role="status"] button')).not.toBeNull();
  });

  it("releases the camera if video playback fails", async () => {
    const stream = fakeStream();
    installGetUserMedia(vi.fn().mockResolvedValue(stream));
    vi.spyOn(HTMLMediaElement.prototype, "play").mockRejectedValue(new Error("playback failed"));
    render(<ComposerCameraPanel onCapture={() => {}} onClose={() => {}} />);
    await act(async () => {});
    expect(stream.getTracks()[0].stop).toHaveBeenCalledOnce();
    expect(container.querySelector('[role="alert"] button')).not.toBeNull();
  });

  it.each(["throw", "empty"])("offers fallback and releases the camera when encoding returns %s", async (failure) => {
    const stream = fakeStream();
    installGetUserMedia(vi.fn().mockResolvedValue(stream));
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({ drawImage: vi.fn() } as unknown as CanvasRenderingContext2D);
    vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation((callback) => {
      if (failure === "throw") throw new Error("encode failed");
      callback(null);
    });
    const onCapture = vi.fn();
    render(<ComposerCameraPanel onCapture={onCapture} onClose={() => {}} />);
    await act(async () => {});
    Object.defineProperties(container.querySelector("video")!, {
      readyState: { value: 4 }, videoWidth: { value: 640 }, videoHeight: { value: 480 },
    });
    await act(async () => { container.querySelector<HTMLButtonElement>(".composer-camera-shutter")!.click(); });
    expect(stream.getTracks()[0].stop).toHaveBeenCalledOnce();
    expect(onCapture).not.toHaveBeenCalled();
    expect(container.querySelector('[role="alert"] button')).not.toBeNull();
  });
});
