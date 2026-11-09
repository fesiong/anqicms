const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');
const { buildSync } = require('esbuild');

const filename = path.join(__dirname, 'policy.ts');
let policy = {};
if (fs.existsSync(filename)) {
  const output = buildSync({
    entryPoints: [filename],
    bundle: true,
    platform: 'node',
    format: 'cjs',
    write: false,
  });
  const loaded = new Module(filename, module);
  loaded._compile(output.outputFiles[0].text, filename);
  policy = loaded.exports;
}

const capabilities = [
  {
    name: 'content_article',
    title: '文章管理',
    risk: 'write',
    default_off: true,
    actions: ['list', 'save', 'delete'],
  },
  {
    name: 'traffic_statistics',
    title: '查看统计数据',
    risk: 'read',
    actions: ['dashboard', 'traffic'],
  },
  { name: 'mcp_list_intents', title: '列出意图', risk: 'read', actions: [] },
  {
    name: 'fs_write',
    title: '写入文件',
    risk: 'system',
    default_off: true,
    actions: [],
    blocked: true,
    reason: '首版关闭',
  },
];

test('parsePolicy treats empty and blank input as an empty policy', () => {
  assert.equal(typeof policy.parsePolicy, 'function');
  assert.deepEqual(policy.parsePolicy(''), { intents: {}, valid: true });
  assert.deepEqual(policy.parsePolicy('   '), { intents: {}, valid: true });
});

test('parsePolicy reports invalid JSON but keeps deny semantics', () => {
  const parsed = policy.parsePolicy('{"intents":');
  assert.equal(parsed.valid, false);
  assert.deepEqual(parsed.intents, {});
  assert.equal(policy.resolveIntentDecision(parsed.intents, 'api'), 'deny');
});

test('parsePolicy rejects arrays, scalars and null', () => {
  for (const raw of ['[]', '"x"', 'null', '1']) {
    const parsed = policy.parsePolicy(raw);
    assert.equal(parsed.valid, false, raw);
    assert.deepEqual(parsed.intents, {}, raw);
  }
});

test('parsePolicy drops unknown decisions and blank intent names', () => {
  const parsed = policy.parsePolicy(
    JSON.stringify({
      intents: {
        '': { '*': 'allow' },
        api: { list: 'allow', invoke: 'maybe', '*': 'confirm' },
      },
    }),
  );
  assert.equal(parsed.valid, true);
  assert.deepEqual(parsed.intents, {
    api: { list: 'allow', '*': 'confirm' },
  });
});

test('resolveIntentDecision reads the intent-level wildcard', () => {
  const intents = {
    api: { '*': 'allow' },
    content_article: { '*': 'confirm' },
    fs_write: { '*': 'deny' },
  };
  assert.equal(policy.resolveIntentDecision(intents, 'api'), 'allow');
  assert.equal(policy.resolveIntentDecision(intents, 'content_article'), 'confirm');
  assert.equal(policy.resolveIntentDecision(intents, 'fs_write'), 'deny');
  assert.equal(policy.resolveIntentDecision(intents, 'unknown'), 'deny');
});

test('resolveIntentDecision keeps legacy action-level policies conservative', () => {
  // 历史手写策略按动作展开：意图级展示取最严的一项，未列出的动作按拒绝计。
  assert.equal(
    policy.resolveIntentDecision(
      { api: { list: 'allow', invoke: 'confirm' } },
      'api',
    ),
    'deny',
  );
  assert.equal(
    policy.resolveIntentDecision(
      { api: { list: 'allow', '*': 'confirm' } },
      'api',
    ),
    'confirm',
  );
  assert.equal(
    policy.resolveIntentDecision(
      { api: { list: 'deny', '*': 'allow' } },
      'api',
    ),
    'deny',
  );
});

test('buildPolicyRows lists one row per intent and resolves its decision', () => {
  const rows = policy.buildPolicyRows(
    capabilities,
    JSON.stringify({
      intents: {
        content_article: { '*': 'confirm' },
        traffic_statistics: { '*': 'allow' },
      },
    }),
  );
  const byKey = Object.fromEntries(rows.map((row) => [row.key, row]));
  assert.equal(rows.length, capabilities.length);
  assert.equal(byKey.content_article.decision, 'confirm');
  assert.equal(byKey.traffic_statistics.decision, 'allow');
  assert.equal(byKey.mcp_list_intents.decision, 'deny');
  assert.equal(byKey.content_article.title, '文章管理');
  assert.equal(byKey.content_article.risk, 'write');
  assert.equal(byKey.content_article.defaultOff, true);
  assert.equal(byKey.traffic_statistics.defaultOff, false);
});

test('buildPolicyRows collapses legacy action-level entries to one row', () => {
  const rows = policy.buildPolicyRows(
    capabilities,
    JSON.stringify({
      intents: { content_article: { list: 'allow', '*': 'confirm' } },
    }),
  );
  const row = rows.find((item) => item.key === 'content_article');
  assert.equal(row.decision, 'confirm');
});

test('buildPolicyRows keeps blocked metadata on rows', () => {
  const rows = policy.buildPolicyRows(capabilities, '');
  const blocked = rows.find((item) => item.intent === 'fs_write');
  assert.equal(blocked.blocked, true);
  assert.equal(blocked.reason, '首版关闭');
  assert.equal(blocked.decision, 'deny');
});

test('serializePolicy writes intent-level wildcards and omits deny', () => {
  const rows = policy.buildPolicyRows(capabilities, '').map((row) => {
    if (row.key === 'content_article') {
      return { ...row, decision: 'allow' };
    }
    if (row.key === 'mcp_list_intents') {
      return { ...row, decision: 'confirm' };
    }
    return row;
  });
  const parsed = JSON.parse(policy.serializePolicy(rows));
  assert.deepEqual(parsed, {
    intents: {
      content_article: { '*': 'allow' },
      mcp_list_intents: { '*': 'confirm' },
    },
  });
});

test('serializePolicy returns an empty object when everything is denied', () => {
  const rows = policy.buildPolicyRows(capabilities, '');
  assert.equal(policy.serializePolicy(rows), '{}');
});

test('serializePolicy collapses legacy action-level policies to one wildcard', () => {
  const raw = JSON.stringify({
    intents: { content_article: { list: 'allow', '*': 'confirm' } },
  });
  const rows = policy.buildPolicyRows(capabilities, raw);
  assert.deepEqual(JSON.parse(policy.serializePolicy(rows)), {
    intents: { content_article: { '*': 'confirm' } },
  });
});

test('serializePolicy round-trips decisions through buildPolicyRows', () => {
  const rows = policy.buildPolicyRows(
    capabilities,
    JSON.stringify({
      intents: {
        content_article: { '*': 'confirm' },
        traffic_statistics: { '*': 'allow' },
        fs_write: { '*': 'deny' },
      },
    }),
  );
  const rebuilt = policy.buildPolicyRows(capabilities, policy.serializePolicy(rows));
  assert.deepEqual(
    rebuilt.map((row) => [row.key, row.decision]),
    rows.map((row) => [row.key, row.decision]),
  );
});

test('summarizePolicyRaw counts intents and their decisions', () => {
  assert.deepEqual(policy.summarizePolicyRaw(''), {
    intents: 0,
    allow: 0,
    confirm: 0,
  });
  assert.deepEqual(
    policy.summarizePolicyRaw(
      JSON.stringify({
        intents: {
          api: { '*': 'allow' },
          content_article: { '*': 'confirm' },
          '': { '*': 'allow' },
        },
      }),
    ),
    { intents: 2, allow: 1, confirm: 1 },
  );
});

test('summarizePolicyRaw counts a legacy mixed intent once, as its strictest decision', () => {
  assert.deepEqual(
    policy.summarizePolicyRaw(
      JSON.stringify({
        intents: { api: { list: 'allow', invoke: 'confirm' } },
      }),
    ),
    { intents: 1, allow: 0, confirm: 0 },
  );
});

test('summarizePolicyRaw treats invalid JSON as unconfigured', () => {
  assert.deepEqual(policy.summarizePolicyRaw('not-json'), {
    intents: 0,
    allow: 0,
    confirm: 0,
  });
});

test('applyPolicyPreset readonly allows read intents only', () => {
  const rows = policy.buildPolicyRows(capabilities, '');
  const applied = policy.applyPolicyPreset(rows, 'readonly');
  const decisions = Object.fromEntries(applied.map((row) => [row.key, row.decision]));
  assert.equal(decisions.traffic_statistics, 'allow');
  assert.equal(decisions.mcp_list_intents, 'allow');
  assert.equal(decisions.content_article, 'deny');
  assert.equal(decisions.fs_write, 'deny');
});

test('applyPolicyPreset deny resets every row', () => {
  const rows = policy.buildPolicyRows(
    capabilities,
    JSON.stringify({ intents: { content_article: { '*': 'allow' } } }),
  );
  const applied = policy.applyPolicyPreset(rows, 'deny');
  assert.equal(
    applied.every((row) => row.decision === 'deny'),
    true,
  );
});

test('applyPolicyPreset regular allows default-open intents only', () => {
  const rows = policy.buildPolicyRows(capabilities, '');
  const applied = policy.applyPolicyPreset(rows, 'regular');
  const decisions = Object.fromEntries(applied.map((row) => [row.key, row.decision]));
  assert.equal(decisions.traffic_statistics, 'allow');
  assert.equal(decisions.mcp_list_intents, 'allow');
  // 默认关闭的意图保持拒绝，硬拦截的同样
  assert.equal(decisions.content_article, 'deny');
  assert.equal(decisions.fs_write, 'deny');
});

test('applyPolicyPreset allow allows every configurable intent', () => {
  const rows = policy.buildPolicyRows(capabilities, '');
  const applied = policy.applyPolicyPreset(rows, 'allow');
  const decisions = Object.fromEntries(applied.map((row) => [row.key, row.decision]));
  assert.equal(decisions.traffic_statistics, 'allow');
  assert.equal(decisions.content_article, 'allow');
  assert.equal(decisions.mcp_list_intents, 'allow');
  // 硬拦截由代码层固定关闭，任何快捷设置都不能把它们打开
  assert.equal(decisions.fs_write, 'deny');
});

test('serializePolicy never writes hard-blocked intents', () => {
  const rows = policy.buildPolicyRows(capabilities, '').map((row) =>
    row.intent === 'fs_write' ? { ...row, decision: 'allow' } : row,
  );
  assert.equal(policy.serializePolicy(rows), '{}');
});
