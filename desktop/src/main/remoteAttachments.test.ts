import { expect, it } from "vitest";
import { RemoteAttachments } from "./remoteAttachments";
it("defers images, reads bounded chunks and hydrates edits without changing history", () => {
  const store = new RemoteAttachments();
  const image = { media_type: "image/png", data: "a".repeat(400_000) };
  const original = { images: [image], text: "keep text" };
  const projected = store.project(original) as {images: Array<typeof image & {remote_ref:string}>};
  expect(JSON.stringify(projected).length).toBeLessThan(500);
  expect(image.data.length).toBe(400_000);
  expect(store.hydrate(projected)).toEqual(original);
  const ref = projected.images[0].remote_ref;
  const parts: string[] = [];
  for (let offset = 0; offset < image.data.length;) {
    const part = store.read({ ref, offset });
    expect(part.data.length).toBeLessThanOrEqual(128 * 1024);
    parts.push(part.data); offset += part.data.length;
  }
  expect(parts.join("")).toBe(image.data);
  expect(() => store.read({ref, offset:-1})).toThrow();
  expect(() => store.read({ref, offset:0.5})).toThrow();
  expect(() => store.read({ref:"unknown"})).toThrow("expired");
  store.clear();
  expect(() => store.hydrate(projected)).toThrow("expired");
});
