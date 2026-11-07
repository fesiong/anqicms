const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');
const { buildSync } = require('esbuild');
const filename = path.join(__dirname, 'selection.ts');
let selection = {};
if (fs.existsSync(filename)) {
  const output = buildSync({
    entryPoints: [filename],
    bundle: true,
    platform: 'node',
    format: 'cjs',
    write: false,
  });
  const loaded = new Module(filename, module);
  loaded._compile(output.outputFiles[0].text, filename);
  selection = loaded.exports;
}

test('selection retains rows from other pages and removes unchecked rows', () => {
  assert.equal(typeof selection.mergeSelectedArchives, 'function');
  const result = selection.mergeSelectedArchives(
    [
      { id: 1, title: 'First' },
      { id: 2, title: 'Second' },
    ],
    [2, 3],
    [{ id: 3, title: 'Third' }],
  );
  assert.deepEqual(result, [
    { id: 2, title: 'Second' },
    { id: 3, title: 'Third' },
  ]);
});

test('preserved keys tolerate rows not loaded on the current page', () => {
  assert.deepEqual(
    selection.mergeSelectedArchives(
      [{ id: 2, title: 'Second' }],
      [2],
      [undefined],
    ),
    [{ id: 2, title: 'Second' }],
  );
});

test('selected IDs without loaded titles are retained rather than silently cleared', () => {
  assert.equal(typeof selection.mergeSelectedArchives, 'function');
  assert.deepEqual(selection.mergeSelectedArchives([], [99], []), [
    { id: 99, title: '#99' },
  ]);
});

test('one unavailable archive does not discard other selected titles', async () => {
  const rows = await selection.loadSelectedArchives([1, 2], async ({ id }) => {
    if (id === 1) throw new Error('unavailable');
    return { data: { title: 'Second' } };
  });
  assert.deepEqual(rows, [
    { id: 1, title: '#1' },
    { id: 2, title: 'Second' },
  ]);
});

test('selected title hydration fetches every ID without a list page limit', async () => {
  assert.equal(typeof selection.loadSelectedArchives, 'function');
  const ids = Array.from({ length: 25 }, (_, i) => i + 1);
  const result = await selection.loadSelectedArchives(ids, async ({ id }) => ({
    data: { id, title: `Document ${id}` },
  }));
  assert.equal(result.length, 25);
  assert.deepEqual(result[24], { id: 25, title: 'Document 25' });
});
