import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const pairs = [
  ['index', 'zh'],
  ...['features', 'blog', 'context-projection', 'note-compaction'].map((name) => [name, `zh-${name}`]),
];
const read = name => readFileSync(new URL(`../${name}.html`, import.meta.url), 'utf8');
const links = source => [...source.matchAll(/<a\b([^>]*)>/g)].map(([, attrs]) => Object.fromEntries([...attrs.matchAll(/([\w-]+)="([^"]*)"/g)].map(([, key, value]) => [key, value])));

test('every marketing page switches directly to its translated counterpart', () => {
  for (const [en, zh] of pairs) {
    const forward = links(read(en)).filter(a => a.hreflang);
    const back = links(read(zh)).filter(a => a.hreflang);
    assert.equal(forward.length, 1, en);
    assert.equal(back.length, 1, zh);
    assert.equal(forward[0].href, `${zh}.html`);
    assert.equal(back[0].href, en === 'index' ? './' : `${en}.html`);
    assert.equal(forward[0].hreflang, 'zh-CN');
    assert.equal(back[0].hreflang, 'en');
  }
});

test('Chinese navigation and article links do not fall back to English', () => {
  const chinese = new Set(pairs.map(([, zh]) => `${zh}.html`));
  for (const [, zh] of pairs) {
    assert.match(read(zh), /<html lang="zh-CN">/);
    for (const a of links(read(zh))) {
      if (a.hreflang || !a.href) continue;
      assert.ok(!a.href.includes('/wuu/en/'), `${zh}: ${a.href}`);
      const path = a.href.split('#')[0];
      if (path.endsWith('.html') && !path.includes('://')) assert.ok(chinese.has(path), `${zh}: ${path}`);
      assert.notEqual(path, './', `${zh}: homepage loses language`);
    }
  }
});
