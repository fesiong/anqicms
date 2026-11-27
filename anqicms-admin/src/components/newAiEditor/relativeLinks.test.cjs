const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');
const { chromium } = require('playwright');

const bundle = buildSync({
  stdin: {
    contents: `import { AiEditor, LinkMenuItem } from 'aieditor'; window.LinkMenuItem = LinkMenuItem;
      import { enableRelativeLinks } from './relativeLinks';
      window.createEditor = () => {
        const editor = new AiEditor({ element: document.querySelector('#editor'), content: '<p>Link text</p>' });
        enableRelativeLinks(editor);
        window.editor = editor;
      };`,
    resolveDir: __dirname,
  },
  bundle: true,
  write: false,
  format: 'iife',
}).outputFiles[0].text;

test('link dialogs accept relative references without weakening link protocol safety', async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  try {
    const page = await browser.newPage();
    await page.setContent('<div id="editor"></div>');
    await page.addScriptTag({ content: bundle });
    await page.evaluate(() => window.createEditor());
    const result = await page.evaluate(() => {
      const editor = window.editor;
      const menus = [...editor.getMenuItems(), ...editor.getBubbleMenuItems()];
      const dialogs = menus
        .filter((menu) => menu instanceof window.LinkMenuItem)
        .map((menu) => {
          const button = menu.getElement();
          const id = button?.getAttribute('aria-controls');
          return id ? document.getElementById(id) : null;
        })
        .filter((dialog) => dialog?.querySelector('input[autocomplete="url"]'));
      const values = [
        '/help/test',
        '../article',
        './page',
        '#section',
        '?page=2',
        'https://example.com/path',
      ];
      const validations = dialogs.map((dialog) => {
        const input = dialog.querySelector('input[autocomplete="url"]');
        const valid = values.map((value) => {
          input.value = value;
          return input.checkValidity();
        });
        input.value = '';
        return { valid, emptyValid: input.checkValidity() };
      });
      const input = dialogs[0].querySelector('input[autocomplete="url"]');
      const editorInstance = editor.editor;
      editorInstance.commands.setTextSelection({ from: 1, to: 10 });
      menus
        .find((menu) => menu.id === 'link')
        .getElement()
        .click();
      input.value = '/help/test';
      dialogs[0]
        .querySelector('form')
        .requestSubmit(dialogs[0].querySelector('button[value="apply"]'));
      const href = editorInstance.getAttributes('link').href;
      editorInstance.commands.unsetLink();
      const unsafe = editorInstance.commands.setLink({
        href: 'javascript:alert(1)',
      });
      return {
        validations,
        href,
        unsafe,
        unsafeHref: editorInstance.getAttributes('link').href,
      };
    });
    assert.ok(
      result.validations.length >= 3,
      'covers toolbar, text bubble and link edit dialogs',
    );
    for (const validation of result.validations) {
      assert.deepEqual(validation.valid, [true, true, true, true, true, true]);
      assert.equal(validation.emptyValid, false);
    }
    assert.equal(result.href, '/help/test');
    assert.equal(result.unsafe, false);
    assert.equal(result.unsafeHref, undefined);
  } finally {
    await browser.close();
  }
});
