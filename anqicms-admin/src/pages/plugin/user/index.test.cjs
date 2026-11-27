const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const src = fs.readFileSync(path.join(__dirname, 'index.tsx'), 'utf8');

// 2026-11-23 现场：点击“更改余额”报 `BalanceForm is not defined`，
// 原因是页面渲染了 <BalanceForm> 却没有 import。这条护栏锁住两者必须成对出现。
test('plugin user page imports BalanceForm before rendering it', () => {
  assert.match(src, /<BalanceForm\b/, '页面应渲染 BalanceForm');

  assert.match(
    src,
    /import\s+BalanceForm\s+from\s+['"]\.\/components\/balanceForm['"]/,
    'BalanceForm 必须从 ./components/balanceForm 导入，否则点击“更改余额”会 ReferenceError',
  );
});
