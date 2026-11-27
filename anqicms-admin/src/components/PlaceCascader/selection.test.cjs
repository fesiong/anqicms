const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');
const path = require('node:path');

const bundle = buildSync({
  entryPoints: [path.join(__dirname, 'selection.ts')],
  bundle: true,
  write: false,
  platform: 'node',
  format: 'cjs',
}).outputFiles[0].text;
const mod = { exports: {} };
new Function('module', 'exports', 'require', bundle)(mod, mod.exports, require);
const { hasSelectedPlace } = mod.exports;

// 空选择曾返回 {country_code:'',country:''} 这个真值对象，绕过 required，
// 导致“未选发货地也能保存自定义运费”。
test('single place requires a non-empty country_code', () => {
  assert.equal(hasSelectedPlace(undefined, false), false);
  assert.equal(hasSelectedPlace(null, false), false);
  assert.equal(hasSelectedPlace({ country_code: '' }, false), false);
  assert.equal(hasSelectedPlace({ country_code: 'CN' }, false), true);
});

test('multiple places require a non-empty list with country_code on every item', () => {
  assert.equal(hasSelectedPlace([], true), false);
  assert.equal(hasSelectedPlace(undefined, true), false);
  assert.equal(hasSelectedPlace([{ country_code: '' }], true), false);
  assert.equal(hasSelectedPlace([{ country_code: 'CN' }], true), true);
  assert.equal(
    hasSelectedPlace([{ country_code: 'CN' }, { country_code: 'US' }], true),
    true,
  );
});
