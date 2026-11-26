const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');
const path = require('node:path');

const bundle = buildSync({
  entryPoints: [path.join(__dirname, 'mediaHtml.ts')],
  bundle: true,
  write: false,
  platform: 'node',
  format: 'cjs',
}).outputFiles[0].text;
const mod = { exports: {} };
new Function('module', 'exports', 'require', bundle)(mod, mod.exports, require);
const {
  buildAttachmentVideoHtml,
  getAttachmentVideoPoster,
  getAttachmentVideoSource,
} = mod.exports;

test('pending video attachments keep their original playback URL', () => {
  const item = {
    file_path: '/uploads/source.mov',
    logo: '/uploads/source.mov',
    video_task: { status: 1 },
  };
  assert.equal(getAttachmentVideoSource(item), '/uploads/source.mov');
  assert.equal(getAttachmentVideoPoster(item), '');
  assert.doesNotMatch(buildAttachmentVideoHtml(item), /poster=/);
});

test('completed HLS attachments use the processed playlist and generated poster', () => {
  const item = {
    file_path: '/uploads/source.mov',
    logo: '/uploads/source.mov',
    play_url: '/uploads/video/master.m3u8',
    video_task: {
      status: 2,
      playlist: { poster_url: '/uploads/video/poster.jpg' },
    },
  };
  assert.equal(getAttachmentVideoSource(item), item.play_url);
  assert.equal(getAttachmentVideoPoster(item), '/uploads/video/poster.jpg');
  assert.match(
    buildAttachmentVideoHtml(item),
    /src="\/uploads\/video\/master\.m3u8"[^>]*poster="\/uploads\/video\/poster\.jpg"/,
  );
});

test('attachment video URLs are escaped and missing URLs are rejected', () => {
  const html = buildAttachmentVideoHtml({ file_path: '/a.mp4?x="1"' });
  assert.match(html, /src="\/a\.mp4\?x=&quot;1&quot;"/);
  assert.equal(buildAttachmentVideoHtml({}), '');
});
