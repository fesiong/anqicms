const { test } = require('node:test');
const assert = require('node:assert/strict');
const { buildSync } = require('esbuild');
const path = require('node:path');

const bundle = buildSync({
  entryPoints: [path.join(__dirname, 'logistics.ts')],
  bundle: true,
  write: false,
  platform: 'node',
  format: 'cjs',
}).outputFiles[0].text;
const mod = { exports: {} };
new Function('module', 'exports', 'require', bundle)(mod, mod.exports, require);
const { orderLogisticsFields } = mod.exports;

// 订单详情过去只显示发货时间，数据库里已有的快递公司/运单号看不到。
test('order detail exposes express company and tracking number', () => {
  const fields = orderLogisticsFields({
    express_company: 'SF Express',
    tracking_number: 'SF123456789',
  });
  assert.deepEqual(fields, [
    { labelKey: 'plugin.order.express-company', value: 'SF Express' },
    { labelKey: 'plugin.order.tracking-number', value: 'SF123456789' },
  ]);
});

test('missing logistics falls back to a dash', () => {
  const fields = orderLogisticsFields({});
  assert.deepEqual(fields, [
    { labelKey: 'plugin.order.express-company', value: '-' },
    { labelKey: 'plugin.order.tracking-number', value: '-' },
  ]);
});
