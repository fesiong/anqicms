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
