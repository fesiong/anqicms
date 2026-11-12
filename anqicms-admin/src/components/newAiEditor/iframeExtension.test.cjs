/**
 * 护栏：非 YouTube 的 iframe 嵌入代码必须变成 iframe 节点，而不是正文源码。
 *
 * aieditor 只内置 youtube / twitch 嵌入节点，schema 里没有通用 iframe，
 * 所以没注册扩展时 B 站/优酷的 <iframe> 代码会被 tiptap 当纯文本插进正文。
 * 第 1 条是反向对照：不注册扩展必须仍然文本化。
 */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');
const { chromium } = require('playwright');

const bundle = buildSync({
  stdin: {
    contents: `
      import { AiEditor } from 'aieditor';
      import { Iframe, parseIframeHtml, isSafeEmbedSrc, normalizeSize } from './iframeExtension';
      window.iframeExt = { Iframe, parseIframeHtml, isSafeEmbedSrc, normalizeSize };
      window.createEditor = (content, withIframe) => {
        const editor = new AiEditor({
          element: document.querySelector('#editor'),
          content,
          ...(withIframe ? { extensions: [Iframe] } : {}),
        });
        window.editor = editor;
        return editor;
      };
    `,
    resolveDir: __dirname,
  },
  bundle: true,
  write: false,
  format: 'iife',
}).outputFiles[0].text;

const BILIBILI =
  '<iframe src="//player.bilibili.com/player.html?bvid=BV1xx411c7mD" scrolling="no" border="0" frameborder="no" framespacing="0" allowfullscreen="true" width="100%" height="480"></iframe>';

function nodesOfType(json, type) {
  const walk = (node, out = []) => {
    if (node.type === type) out.push(node);
    (node.content || []).forEach((child) => walk(child, out));
    return out;
  };
  return walk(json);
}

test('without the extension an iframe embed still degrades to text (negative control)', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate((embed) => {
      const editor = window.createEditor('<p>start</p>', false);
      editor.editor.commands.insertContent(embed);
      return { html: editor.getHTML(), json: editor.getJSON() };
    }, BILIBILI);

    assert.equal(
      nodesOfType(result.json, 'iframe').length,
      0,
      '不注册扩展时本来就解析不出 iframe 节点——这正是原来的 bug',
    );
    assert.match(result.html, /&lt;iframe/);
  } finally {
    await browser.close();
  }
});

test('with the extension the embed becomes an iframe node', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate((embed) => {
      const editor = window.createEditor('<p>start</p>', true);
      editor.editor.commands.insertContent(embed);
      return { html: editor.getHTML(), json: editor.getJSON() };
    }, BILIBILI);

    const nodes = nodesOfType(result.json, 'iframe');
    assert.equal(nodes.length, 1);
    assert.match(nodes[0].attrs.src, /player\.bilibili\.com/);
    assert.doesNotMatch(result.html, /&lt;iframe/);
    // 宽高必须落到 style，否则前台 iframe 会塌成 0 高度
    // （style 会被浏览器序列化成 `width: 100%` 带空格的形式）
    assert.match(result.html, /style="[^"]*width:\s*100%/);
    assert.match(result.html, /style="[^"]*height:\s*480px/);
    assert.match(result.html, /allowfullscreen="true"/);
  } finally {
    await browser.close();
  }
});

test('parsed attributes survive a reload round trip', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate((embed) => {
      const first = window.createEditor('<p>a</p>', true);
      first.editor.commands.insertContent(embed);
      const html = first.getHTML();
      first.destroy();
      document.querySelector('#editor').innerHTML = '';
      // 重新打开文章 = setContent(存下来的 HTML)
      const second = window.createEditor(html, true);
      return {
        html: second.getHTML(),
        nodes: JSON.stringify(second.getJSON()),
      };
    }, BILIBILI);

    assert.equal(nodesOfType(JSON.parse(result.nodes), 'iframe').length, 1);
    assert.match(result.html, /player\.bilibili\.com/);
  } finally {
    await browser.close();
  }
});

test('embed helpers', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate(() => {
      const { parseIframeHtml, isSafeEmbedSrc, normalizeSize } =
        window.iframeExt;
      return {
        double: parseIframeHtml(
          '<iframe src="https://x.test/a" width="100%" height="480"></iframe>',
        ),
        single: parseIframeHtml("<iframe src='https://x.test/b'></iframe>"),
        bare: parseIframeHtml('<iframe src=https://x.test/c></iframe>'),
        noSrc: parseIframeHtml('<iframe width="100%"></iframe>'),
        text: parseIframeHtml('https://x.test/c.mp4'),
        safe: [
          isSafeEmbedSrc('https://x.test/a'),
          isSafeEmbedSrc('//player.bilibili.com/x'),
          isSafeEmbedSrc('/embed/local'),
          isSafeEmbedSrc('javascript:alert(1)'),
          isSafeEmbedSrc('data:text/html,<script>'),
          isSafeEmbedSrc(''),
        ],
        sizes: [
          normalizeSize(480, 'x'),
          normalizeSize('480', 'x'),
          normalizeSize('480px', 'x'),
          normalizeSize('100%', 'x'),
          normalizeSize('', 'x'),
          normalizeSize(undefined, 'x'),
        ],
      };
    });

    assert.deepEqual(result.double, {
      src: 'https://x.test/a',
      width: '100%',
      height: '480',
      title: undefined,
    });
    assert.equal(result.single.src, 'https://x.test/b');
    assert.equal(result.bare.src, 'https://x.test/c');
    assert.equal(result.noSrc, null);
    assert.equal(result.text, null);
    assert.deepEqual(result.safe, [true, true, true, false, false, false]);
    assert.deepEqual(result.sizes, [
      '480px',
      '480px',
      '480px',
      '100%',
      'x',
      'x',
    ]);
  } finally {
    await browser.close();
  }
});
