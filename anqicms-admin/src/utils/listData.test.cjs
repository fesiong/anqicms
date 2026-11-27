const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');
const path = require('node:path');

const bundle = buildSync({
  entryPoints: [path.join(__dirname, 'listData.ts')],
  bundle: true,
  write: false,
  platform: 'node',
  format: 'cjs',
}).outputFiles[0].text;
const mod = { exports: {} };
new Function('module', 'exports', 'require', bundle)(mod, mod.exports, require);
const { normalizeListData } = mod.exports;

// Go 的 nil slice 会序列化成 null；ProTable 在 total=0 时求值 null.length 会抛错。
test('null list data with a numeric total becomes an empty array', () => {
  const out = normalizeListData({ code: 0, total: 0, data: null });
  assert.deepEqual(out.data, []);
  assert.equal(out.total, 0);
});

test('detail responses with null data are left untouched', () => {
  const payload = { code: 0, data: null };
  assert.equal(normalizeListData(payload), payload);
});

test('non-null list data is left untouched', () => {
  const payload = { code: 0, total: 1, data: [{ id: 1 }] };
  assert.equal(normalizeListData(payload), payload);
});
