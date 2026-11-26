const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');
const path = require('node:path');

const bundle = buildSync({
  entryPoints: [path.join(__dirname, 'videoInstall.ts')],
  bundle: true,
  write: false,
  platform: 'node',
  format: 'cjs',
}).outputFiles[0].text;
const mod = { exports: {} };
new Function('module', 'exports', 'require', bundle)(mod, mod.exports, require);
const { parseSSEFrames } = mod.exports;

test('SSE parser keeps an incomplete frame for the next network chunk', () => {
  const first = parseSSEFrames('event: message\ndata: {"stage":"checking"');
  assert.deepEqual(first.events, []);
  assert.equal(first.rest, 'event: message\ndata: {"stage":"checking"');

  const second = parseSSEFrames(first.rest + '}\n\n');
  assert.deepEqual(second, {
    events: [{ event: 'message', data: { stage: 'checking' } }],
    rest: '',
  });
});

test('SSE parser accepts CRLF and joins multiline data', () => {
  const result = parseSSEFrames(
    'event: message\r\ndata: {"stage":"output",\r\ndata: "output":"apt update"}\r\n\r\n',
  );
  assert.deepEqual(result.events, [
    { event: 'message', data: { stage: 'output', output: 'apt update' } },
  ]);
});

test('SSE parser processes multiple frames and ignores comments', () => {
  const result = parseSSEFrames(
    ': heartbeat\n\nevent: done\ndata: {"stage":"done"}\n\nevent: error\ndata: {"stage":"error","message":"failed"}\n\n',
  );
  assert.deepEqual(result.events, [
    { event: 'done', data: { stage: 'done' } },
    { event: 'error', data: { stage: 'error', message: 'failed' } },
  ]);
});
