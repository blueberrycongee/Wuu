import { expect, it } from "vitest";
import { RemoteHistory } from "./remoteHistory";
it("pages immutable history newest first without losing or repeating turns", () => {
  const history = new RemoteHistory();
  const turns = Array.from({length:63}, (_, i) => ({id:String(i), text:"x".repeat(20_000)}));
  const source = {thread:{id:"t",turns}};
  const projected = history.project(source) as {thread:{id:string;turns:typeof turns;history_cursor?:string}};
  expect(JSON.stringify(projected).length).toBeLessThan(270_000);
  expect(source.thread.turns.length).toBe(63);
  let cursor = projected.thread.history_cursor;
  let all = projected.thread.turns;
  while (cursor) {
    const page = history.read({cursor});
    expect(history.read({cursor})).toEqual(page);
    all = [...page.turns as typeof turns,...all]; cursor=page.history_cursor;
  }
  expect(all).toEqual(turns);
  expect(() => history.read({cursor:"missing"})).toThrow("expired");
});
it("keeps small histories complete and does not page unrelated arrays", () => {
  const history = new RemoteHistory();
  expect(history.project({thread:{id:"t",turns:[{id:"one"}]}})).toEqual({thread:{id:"t",turns:[{id:"one"}],history_cursor:undefined}});
  expect(history.project({turns:[{id:"one"}]})).toEqual({turns:[{id:"one"}]});
});
