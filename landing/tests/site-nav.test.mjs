import test from "node:test";
import assert from "node:assert/strict";
import { initializeNavigation } from "../site-nav.mjs";

function navigation(hover = true) {
  const listeners = new WeakMap();
  const element = () => {
    const node = {
      open: false,
      classList: { add() {}, toggle() {} },
      setAttribute() {},
      append() {},
      after() {},
      addEventListener(type, handler) {
        const events = listeners.get(this) ?? {};
        events[type] = handler;
        listeners.set(this, events);
      },
      contains(target) {
        return target === this || target?.parent === this;
      },
    };
    return node;
  };
  const document = element();
  const menus = Array.from({ length: 4 }, () => element());
  for (const menu of menus) {
    const panel = element();
    const summary = {
      ...element(),
      parent: menu,
      focus() {
        document.activeElement = this;
      },
    };
    menu.querySelector = selector => selector === ".nav-panel" ? panel : summary;
  }
  const header = element();
  const surfaceStates = [];
  const shell = { ...element(),
    style: { setProperty() {} },
    classList: { add() {}, toggle(name, value) { surfaceStates.push(value); } },
    querySelector: () => header, querySelectorAll: () => menus,
  };
  document.querySelector = () => shell;
  const pending = new Map();
  let id = 0;
  initializeNavigation(
    document,
    { matches: hover },
    {
      setTimeout(fn) {
        pending.set(++id, fn);
        return id;
      },
      clearTimeout(timer) {
        pending.delete(timer);
      },
    },
  );
  return {
    document,
    menus,
    header,
    surfaceStates,
    emit(node, type, event = {}) {
      listeners.get(node)?.[type]?.(event);
    },
    flush() {
      for (const [id, fn] of pending) {
        pending.delete(id);
        fn();
      }
    },
  };
}

test("moving across header destinations opens only the current panel and cancels the old close", () => {
  const ui = navigation();
  for (const menu of ui.menus) {
    ui.emit(menu, "pointerenter");
    ui.flush();
    assert.deepEqual(
      ui.menus.map((m) => m.open),
      ui.menus.map((m) => m === menu),
    );
    ui.emit(menu, "pointerleave");
  }
  ui.flush();
  assert.ok(ui.menus.every((m) => !m.open));
});

test("crossing into the panel keeps it open; Escape restores its trigger and outside presses close it", () => {
  const ui = navigation();
  const menu = ui.menus[1];
  ui.emit(menu, "pointerenter");
  ui.emit(menu, "pointerleave");
  ui.emit(menu, "pointerenter");
  ui.flush();
  assert.equal(menu.open, true);
  ui.emit(ui.document, "keydown", { key: "Escape" });
  assert.equal(menu.open, false);
  assert.equal(ui.document.activeElement, menu.querySelector("summary"));
  ui.emit(menu, "pointerenter");
  ui.emit(ui.document, "pointerdown", { target: {} });
  assert.equal(menu.open, false);
});

test("touch does not hover-open; native toggle remains exclusive and focus leaving closes the panel", () => {
  const ui = navigation(false);
  ui.emit(ui.menus[0], "pointerenter");
  assert.equal(ui.menus[0].open, false);
  ui.menus[0].open = true;
  ui.emit(ui.menus[0], "toggle");
  ui.menus[2].open = true;
  ui.emit(ui.menus[2], "toggle");
  assert.deepEqual(
    ui.menus.map((m) => m.open),
    [false, false, true, false],
  );
  ui.emit(ui.menus[2], "focusout", { relatedTarget: {} });
  assert.equal(ui.menus[2].open, false);
});

test('entering a detached panel cancels closure and keeps its links usable', () => {
  const ui = navigation();
  const menu = ui.menus[0];
  const panel = menu.querySelector('.nav-panel');
  ui.emit(menu, 'pointerenter');
  ui.emit(menu, 'pointerleave');
  ui.emit(panel, 'pointerenter');
  ui.flush();
  assert.equal(menu.open, true);
  assert.equal(panel.inert, false);
  ui.emit(menu, 'focusout', { relatedTarget: { parent: panel } });
  assert.equal(menu.open, true);
  ui.emit(ui.document, 'keydown', { key: 'Escape' });
  assert.equal(panel.inert, true);
  ui.emit(menu, 'pointerenter');
  ui.flush();
  assert.equal(panel.inert, false);
});

test('crossing a button gap never collapses the shared surface', () => {
  const ui = navigation();
  ui.emit(ui.menus[0], 'pointerenter');
  ui.surfaceStates.length = 0;
  ui.emit(ui.menus[0], 'pointerleave', { relatedTarget: ui.header });
  ui.flush();
  assert.equal(ui.menus[0].open, true);
  ui.emit(ui.menus[1], 'pointerenter');
  ui.emit(ui.menus[0], 'toggle');
  assert.ok(ui.surfaceStates.every(Boolean));
  assert.equal(ui.menus[1].open, true);
});
