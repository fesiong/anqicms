const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');

const bundle = buildSync({
  entryPoints: [require('node:path').join(__dirname, 'site.ts')],
  bundle: true,
  write: false,
  platform: 'node',
  format: 'cjs',
}).outputFiles[0].text;
const mod = { exports: {} };
new Function('module', 'exports', 'require', bundle)(mod, mod.exports, require);
const { normalizeSites, resolveSiteSelection } = mod.exports;

test('site selection treats API ids and stored ids as the same value', () => {
  const sites = normalizeSites([
    { id: 1, name: 'Main', is_main: 1 },
    { id: 7, name: 'English', is_main: 0 },
  ]);
  assert.deepEqual(resolveSiteSelection(sites, '7'), {
    activeKey: '7',
    isSubsite: true,
  });
});

test('site selection falls back to the first site when the stored site is stale', () => {
  const sites = normalizeSites([{ id: 3, name: 'Main', is_main: 1 }]);
  assert.deepEqual(resolveSiteSelection(sites, '999'), {
    activeKey: '3',
    isSubsite: false,
  });
});

test('empty site lists do not produce an invalid active tab', () => {
  assert.deepEqual(resolveSiteSelection([], '7'), {
    activeKey: '',
    isSubsite: false,
  });
});
