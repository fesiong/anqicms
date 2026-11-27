const { test } = require('node:test');
const assert = require('node:assert/strict');
const { build } = require('esbuild');
const { chromium } = require('playwright');

async function browserBundle(contents, plugins = []) {
  const result = await build({
    stdin: {
      contents,
      resolveDir: __dirname,
      loader: 'tsx',
    },
    bundle: true,
    write: false,
    format: 'esm',
    platform: 'browser',
    target: 'esnext',
    loader: { '.less': 'empty' },
    plugins,
  });
  return result.outputFiles[0].text;
}

test('VideoPlayer ignores a rejected HLS import after unmount', async () => {
  const bundle = await browserBundle(`
    import React from 'react';
    import { createRoot } from 'react-dom/client';
    import { flushSync } from 'react-dom';
    import VideoPlayer from './index';
    const root = createRoot(document.querySelector('#root'));
    window.mountPlayer = () => {
      window.loadCalls = 0;
      HTMLMediaElement.prototype.canPlayType = () => '';
      const load = HTMLMediaElement.prototype.load;
      HTMLMediaElement.prototype.load = function () {
        window.loadCalls += 1;
        return load.call(this);
      };
      flushSync(() => root.render(
        <VideoPlayer src="/pending.m3u8" onError={() => { window.playerError = true; }} />,
      ));
    };
    window.unmountPlayer = () => flushSync(() => root.render(null));
  `, [
    {
      name: 'pending-hls-import',
      setup(build) {
        build.onResolve({ filter: /^hls\.js$/ }, () => ({
          path: 'hls.js',
          namespace: 'pending-hls',
        }));
        build.onLoad({ filter: /.*/, namespace: 'pending-hls' }, () => ({
          contents: `
            await new Promise((_, reject) => { window.rejectHlsImport = reject; });
            export default {};
          `,
          loader: 'js',
        }));
      },
    },
  ]);
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="root"></div>');
    await page.addScriptTag({ content: bundle, type: 'module' });
    await page.waitForFunction(() => typeof window.mountPlayer === 'function');
    await page.evaluate(() => window.mountPlayer());
    await page.waitForFunction(() => typeof window.rejectHlsImport === 'function');
    await page.evaluate(() => window.unmountPlayer());
    await page.evaluate(() => window.rejectHlsImport(new Error('load failed')));
    await page.waitForTimeout(50);
    const loads = await page.evaluate(() => window.loadCalls);
    assert.equal(loads, 2, '卸载清理后的迟到 import 不得再次操作 video');
  } finally {
    await browser.close();
  }
});

test('ImageItem previews video attachments instead of opening the source file', async () => {
  const bundle = await browserBundle(`
    import React from 'react';
    import { createRoot } from 'react-dom/client';
    import { flushSync } from 'react-dom';
    import ImageItem from '../attachment/image';
    const root = createRoot(document.querySelector('#root'));
    window.renderVideo = () => flushSync(() => root.render(
      <ImageItem
        isImage={2}
        src="/movie.mkv"
        previewSrc="/movie.mkv"
        posterSrc="/cover.jpg"
        alt="movie"
      />,
    ));
  `);
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="root"></div>');
    await page.addScriptTag({ content: bundle, type: 'module' });
    await page.waitForFunction(() => typeof window.renderVideo === 'function');
    await page.evaluate(() => window.renderVideo());
    await page.waitForFunction(() => document.querySelector('#root').children.length > 0);
    const initial = await page.evaluate(() => ({
      images: [...document.querySelectorAll('#root img')].map((img) => img.getAttribute('src')),
      links: [...document.querySelectorAll('#root a')].map((link) => link.getAttribute('href')),
    }));
    assert.equal(initial.images.length, 1, '视频附件必须显示封面缩略图');
    assert.match(initial.images[0], /\/cover\.jpg/);
    assert.equal(initial.links.length, 0, '视频附件不能退化成新开原文件链接');
    await page.click('#root .ant-image');
    await page.waitForSelector('.ant-image-preview video');
    const source = await page.$eval('.ant-image-preview video', (video) => video.src);
    assert.match(source, /\/movie\.mkv$/);
  } finally {
    await browser.close();
  }
});
