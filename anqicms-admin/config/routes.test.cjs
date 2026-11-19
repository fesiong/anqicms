const assert = require('node:assert/strict');
const test = require('node:test');
const Module = require('node:module');
const path = require('node:path');
const { buildSync } = require('esbuild');

const filename = path.join(__dirname, 'routes.ts');
const compiled = buildSync({
  entryPoints: [filename],
  bundle: true,
  platform: 'node',
  format: 'cjs',
  write: false,
});
const routeModule = new Module(filename, module);
routeModule._compile(compiled.outputFiles[0].text, filename);
const routes = routeModule.exports.default;
const plugins = routes.find((route) => route.path === '/plugin');

test('visible plugin categories can render their child cards', () => {
  const categories = plugins.routes.filter(
    (route) => !route.hideInMenu && route.name,
  );
  assert.ok(categories.length > 0);
  for (const category of categories) {
    assert.ok(Array.isArray(category.routes), `${category.path} has no child routes`);
    assert.ok(category.routes.length > 0, `${category.path} has no plugin cards`);
  }
});

test('message channels live inside the distribution channel group', () => {
  const channel = plugins.routes.find((route) => route.path === '/plugin/channel');
  assert.ok(channel, 'distribution channel group exists');
  const entry = channel.routes.find((route) => route.name === 'channels');
  assert.ok(entry, 'channels entry is nested under /plugin/channel');
  assert.equal(entry.path, '/plugin/channel/channels');
  assert.equal(entry.component, '@/pages/plugin/channels/index');
  assert.ok(!plugins.routes.some((route) => route.path === '/plugin/channels'));
});

test('Google and LLMs remain available through grouped plugin routes', () => {
  const children = plugins.routes.flatMap((route) => route.routes || []);
  for (const routePath of ['/plugin/channel/google', '/plugin/seo/llms']) {
    assert.ok(children.some((route) => route.path === routePath && route.component));
  }
  for (const routePath of ['/plugin/google', '/plugin/llms']) {
    assert.ok(!plugins.routes.some((route) => route.path === routePath));
  }
});

// 菜单标题按完整层级取 i18n（见 HeaderSearch.resolveTitle: 'menu.' + names.join('.')）。
// channel 子项只能在 menu.ts 里配齐，缺一个就会在侧栏/搜索里露出裸 key。
// 穷举 /plugin/channel 下所有可见子路由，逐语种校验，避免新增渠道时再次漏配。
test('every channel menu entry is translated in all locales', () => {
  const channel = plugins.routes.find((route) => route.path === '/plugin/channel');
  assert.ok(channel && Array.isArray(channel.routes), 'distribution channel group exists');

  const keys = channel.routes
    .filter((route) => !route.hideInMenu && route.name)
    .map((route) => `menu.plugin.channel.${route.name}`);
  assert.ok(keys.length > 0, 'channel group has no visible named children');

  const localesDir = path.join(__dirname, '..', 'src', 'locales');
  for (const locale of ['zh-CN', 'en-US', 'zh-TW']) {
    const menuFile = path.join(localesDir, locale, 'menu.ts');
    const compiled = buildSync({
      entryPoints: [menuFile],
      bundle: true,
      platform: 'node',
      format: 'cjs',
      write: false,
    });
    const menuModule = new Module(menuFile, module);
    menuModule._compile(compiled.outputFiles[0].text, menuFile);
    const messages = menuModule.exports.default;
    // 先证明这份 locale 确实加载了菜单文案，避免空对象让下面断言空转。
    assert.equal(typeof messages['menu.plugin.channel.sendmail'], 'string',
      `[${locale}] locale menu did not load`);
    for (const key of keys) {
      assert.equal(typeof messages[key], 'string', `[${locale}] missing menu i18n: ${key}`);
      assert.ok(messages[key].length > 0, `[${locale}] empty menu i18n: ${key}`);
    }
  }
});
