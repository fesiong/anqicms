/**
 * 护栏：插入视频不能变成「正文里显示源码」。
 *
 * 背景：aieditor 的 video 节点 parseHTML 只有 `video[src]` 一条规则，
 * 旧写法 <video poster="..."><source src="..."></video> 在 video 上没有 src，
 * 解析不出节点，整段 HTML 被 tiptap 当纯文本插入 → 用户看到源码。
 *
 * 这里的断言全部在真实 Chromium + 真实 aieditor 里跑，
 * 并且第 1 条是**反向对照**：旧写法必须仍然变成文本，否则说明测的不是这条路径。
 */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');
const { chromium } = require('playwright');

const bundle = buildSync({
  stdin: {
    contents: `
      import { AiEditor } from 'aieditor';
      import {
        buildVideoHtml,
        normalizeVideoHtml,
        sanitizeEditorHtml,
        isVideoUrl,
        isImageUrl,
      } from './mediaHtml';
      window.media = {
        buildVideoHtml,
        normalizeVideoHtml,
        sanitizeEditorHtml,
        isVideoUrl,
        isImageUrl,
      };
      import { Iframe } from './iframeExtension';
      window.createEditor = (content, options) => {
        const editor = new AiEditor({
          element: document.querySelector('#editor'),
          content,
          ...(options && options.withIframe ? { extensions: [Iframe] } : {}),
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

const LEGACY_VIDEO =
  '<video controls="controls" controlslist="nodownload" poster="https://demo.anqicms.com/en-US/uploads/202611/12/dragon-2.mp4"><source src="https://demo.anqicms.com/en-US/uploads/202611/12/dragon-2.mp4" type="video/mp4">Your browser does not support video tags.</video>';

function videoNodes(json) {
  const walk = (node, out = []) => {
    if (node.type === 'video') out.push(node);
    (node.content || []).forEach((child) => walk(child, out));
    return out;
  };
  return walk(json);
}

test('legacy <video><source> markup still degrades to text (negative control)', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate((legacy) => {
      const editor = window.createEditor('<p>start</p>');
      editor.editor.commands.insertContent(legacy);
      return { html: editor.getHTML(), json: editor.getJSON() };
    }, LEGACY_VIDEO);

    assert.equal(
      videoNodes(result.json).length,
      0,
      '旧写法本来就解析不出 video 节点——这正是线上 bug 的成因',
    );
    assert.match(
      result.html,
      /&lt;video/,
      '旧写法必须落成被转义的文本，才证明修复针对的是这条路径',
    );
  } finally {
    await browser.close();
  }
});

test('buildVideoHtml inserts a real video node', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate(() => {
      const editor = window.createEditor('<p>start</p>');
      editor.editor.commands.insertContent(
        window.media.buildVideoHtml('https://x.test/uploads/a.mp4'),
      );
      return { html: editor.getHTML(), json: editor.getJSON() };
    });

    const nodes = videoNodes(result.json);
    assert.equal(nodes.length, 1, '必须生成 1 个 video 节点');
    assert.equal(nodes[0].attrs.src, 'https://x.test/uploads/a.mp4');
    assert.doesNotMatch(result.html, /&lt;video/, '不允许再出现被转义的源码');
    assert.doesNotMatch(
      result.html,
      /<source\b/,
      '输出里不该再有 <source> 子标签',
    );
  } finally {
    await browser.close();
  }
});

test('normalizeVideoHtml rescues legacy article content', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate((legacy) => {
      const raw = window.createEditor(legacy);
      const rawNodes = JSON.stringify(raw.getJSON());
      raw.destroy();
      document.querySelector('#editor').innerHTML = '';
      const fixed = window.createEditor(
        window.media.normalizeVideoHtml(legacy),
      );
      return {
        rawNodes,
        nodes: JSON.stringify(fixed.getJSON()),
        html: fixed.getHTML(),
      };
    }, LEGACY_VIDEO);

    assert.doesNotMatch(result.rawNodes, /"type":"video"/);
    assert.match(
      result.nodes,
      /"type":"video"/,
      '归一化后必须解析出 video 节点',
    );
    assert.doesNotMatch(result.html, /&lt;video/);
  } finally {
    await browser.close();
  }
});

test('media helper edge cases', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate(() => {
      const {
        buildVideoHtml,
        normalizeVideoHtml,
        sanitizeEditorHtml,
        isVideoUrl,
        isImageUrl,
      } = window.media;
      return {
        // 附件 logo 对视频文件会回落到 mp4 地址，不能当 poster
        noPosterForVideo: buildVideoHtml('a.mp4', 'a.mp4'),
        poster: buildVideoHtml('a.mp4', 'cover.jpg'),
        quote: buildVideoHtml('a.mp4?x="1"'),
        alreadyNormalized: normalizeVideoHtml(
          '<video src="b.mp4" poster="p.png"></video>',
        ),
        singleQuote: normalizeVideoHtml(
          "<video poster='p.png'><source src='c.mp4'></video>",
        ),
        noSource: normalizeVideoHtml('<video controls>oops</video>'),
        sanitize: sanitizeEditorHtml(
          '<p><a href="/x" rel="noopener">a</a></p><video src="a.mp4" controls=""></video>',
        ),
        sanitizeKeepsExisting: sanitizeEditorHtml(
          '<video src="a.mp4" controlslist="nodownload"></video>',
        ),
        extCheck: [
          isVideoUrl('https://x/a.mp4'),
          isVideoUrl('https://x/a.mp4?t=1'),
          isVideoUrl('https://x/a.jpg'),
          isImageUrl('https://x/cover.webp'),
          isImageUrl('https://x/a.mp4'),
          isImageUrl(''),
        ],
      };
    });

    assert.doesNotMatch(
      result.noPosterForVideo,
      /poster=/,
      '视频地址不能当海报',
    );
    assert.match(result.poster, /poster="cover\.jpg"/);
    assert.match(result.quote, /src="a\.mp4\?x=&quot;1&quot;"/, '引号必须转义');
    assert.match(
      result.alreadyNormalized,
      /<video src="b\.mp4"[^>]*><\/video>/,
    );
    assert.match(
      result.singleQuote,
      /<video poster='p\.png' src="c\.mp4"><\/video>/,
    );
    assert.equal(
      result.noSource,
      '<video controls>oops</video>',
      '取不到地址就别动',
    );
    assert.doesNotMatch(result.sanitize, /rel=/);
    assert.match(result.sanitize, /controlslist="nodownload"/);
    assert.equal(
      (result.sanitizeKeepsExisting.match(/controlslist/g) || []).length,
      1,
      '已有 controlslist 不能重复追加',
    );
    assert.deepEqual(result.extCheck, [true, true, false, true, false, false]);
  } finally {
    await browser.close();
  }
});

test('article bodies already polluted with escaped markup are repaired on load', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate((legacy) => {
      // 1. 用旧的插入方式造出线上那份坏数据（这就是库里存的 HTML）
      const pollutedEditor = window.createEditor('<p>before</p>');
      pollutedEditor.editor.commands.insertContent(legacy);
      const polluted = pollutedEditor.getHTML();
      pollutedEditor.destroy();
      document.querySelector('#editor').innerHTML = '';

      // 2. 载入时归一化，应该把视频救回来
      const repaired = window.createEditor(
        window.media.normalizeVideoHtml(polluted),
      );
      return {
        polluted,
        json: JSON.stringify(repaired.getJSON()),
        html: repaired.getHTML(),
      };
    }, LEGACY_VIDEO);

    assert.match(result.polluted, /&lt;video/, '前提：库里存的确实是转义文本');
    assert.match(
      result.json,
      /"type":"video"/,
      '坏数据必须能被救回成 video 节点',
    );
    assert.match(result.json, /dragon-2\.mp4/, '视频地址不能丢');
    assert.doesNotMatch(result.html, /&lt;video/);
    assert.match(result.html, /<p>before<\/p>/, '周围正文要保留');
  } finally {
    await browser.close();
  }
});

test('escaped video inside pre/code stays as sample code', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate(() => {
      const sample =
        '<pre><code>&lt;video src="a.mp4"&gt;&lt;/video&gt;</code></pre>';
      return window.media.normalizeVideoHtml(sample);
    });
    assert.equal(
      result,
      '<pre><code>&lt;video src="a.mp4"&gt;&lt;/video&gt;</code></pre>',
      '示例代码不许被自动改成播放器',
    );
  } finally {
    await browser.close();
  }
});

test('iframe embeds polluted into text are repaired on load too', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    const result = await page.evaluate(() => {
      const embed =
        '<iframe src="//player.bilibili.com/player.html?bvid=BV1xx411c7mD" width="100%" height="480" allowfullscreen="true"></iframe>';
      // 没有 iframe 扩展时插入 = 线上坏数据的产生方式
      const broken = window.createEditor('<p>before</p>');
      broken.editor.commands.insertContent(embed);
      const polluted = broken.getHTML();
      broken.destroy();
      document.querySelector('#editor').innerHTML = '';

      const repaired = window.createEditor(
        window.media.normalizeVideoHtml(polluted),
        { withIframe: true },
      );
      return {
        polluted,
        html: repaired.getHTML(),
        json: JSON.stringify(repaired.getJSON()),
      };
    });

    assert.match(result.polluted, /&lt;iframe/, '前提：库里存的是转义文本');
    assert.match(result.json, /"type":"iframe"/, 'iframe 坏数据也要能救回');
    assert.match(result.json, /player\.bilibili\.com/);
    assert.doesNotMatch(result.html, /&lt;iframe/);
    assert.match(result.html, /<p>before<\/p>/);
  } finally {
    await browser.close();
  }
});
