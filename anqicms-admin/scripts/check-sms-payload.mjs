#!/usr/bin/env node
// 任务 14：buildSmsPayload 纯函数护栏（R123）。
// 仓库无 jest/vitest，不新增测试框架：用 esbuild 把 payload.ts 转成 CJS 写进 tmp/，
// 再用 node:assert 跑 R123 的四条断言 + 反向对照。退出码非 0 即失败。
// 用法（仓库根执行）：node anqicms-admin/scripts/check-sms-payload.mjs
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ADMIN = path.resolve(__dirname, '..');
const ROOT = path.resolve(ADMIN, '..');
const SRC = path.join(ADMIN, 'src/pages/plugin/sms/payload.ts');
const ESBUILD = path.join(ADMIN, 'node_modules/.bin/esbuild');
// tmp/ 已被 .gitignore 忽略，临时产物不得提交。
const TMP_DIR = path.join(ROOT, 'tmp/smscheck');
const OUT_REAL = path.join(TMP_DIR, 'payload.cjs');
const OUT_BAD = path.join(TMP_DIR, 'payload-bad.cjs');
const BAD_SRC = path.join(TMP_DIR, 'payload-bad.ts');

// 声明式路径清单：按 config/sms.go 实际结构穷举 map 字段，遍历时不写死总数。
const MAP_PATHS = [
  ['aliyun', 'domestic', 'templates'],
  ['aliyun', 'international', 'templates'],
  ['tencent', 'domestic', 'templates'],
  ['tencent', 'international', 'templates'],
  ['twilio', 'international', 'texts'],
];

const SCENES = ['login', 'register', 'reset', 'bind_old', 'bind_new'];
const MASK = '********';

// 12 个凭据字段（与 controller/manageController/pluginSms.go 的哨兵回写逐一对应）。
const MASK_FIELDS = [
  ['aliyun', 'domestic', 'access_key_id'],
  ['aliyun', 'domestic', 'access_key_secret'],
  ['aliyun', 'international', 'access_key_id'],
  ['aliyun', 'international', 'access_key_secret'],
  ['tencent', 'domestic', 'secret_id'],
  ['tencent', 'domestic', 'secret_key'],
  ['tencent', 'international', 'secret_id'],
  ['tencent', 'international', 'secret_key'],
  ['twilio', 'international', 'account_sid'],
  ['twilio', 'international', 'auth_token'],
  ['twilio', 'international', 'api_key_sid'],
  ['twilio', 'international', 'api_key_secret'],
];

const getPath = (obj, parts) =>
  parts.reduce((cur, key) => (cur == null ? undefined : cur[key]), obj);

const setPath = (obj, parts, value) => {
  let cur = obj;
  for (let i = 0; i < parts.length - 1; i += 1) {
    cur = cur[parts[i]];
  }
  cur[parts[parts.length - 1]] = value;
};

// 一份完整配置：三个厂商对象的完整子树 + 全部顶层参数，
// 模拟 ProForm.getFieldsValue(true) 取到的全量表单值。
const makeFullValues = () => {
  const templateMap = () =>
    SCENES.reduce((acc, scene) => {
      acc[scene] = `TPL_${scene}`;
      return acc;
    }, {});
  const aliyunRegion = () => ({
    access_key_id: 'AKID_REAL',
    access_key_secret: 'SECRET_REAL',
    endpoint: 'dysmsapi.aliyuncs.com',
    sign_name: '示例签名',
    param_name: 'code',
    templates: templateMap(),
  });
  const tencentRegion = () => ({
    secret_id: 'SECRET_ID_REAL',
    secret_key: 'SECRET_KEY_REAL',
    sms_sdk_app_id: '1400000000',
    region: 'ap-guangzhou',
    sign_name: '示例签名',
    templates: templateMap(),
  });
  return {
    provider: 'aliyun',
    aliyun: { domestic: aliyunRegion(), international: aliyunRegion() },
    tencent: { domestic: tencentRegion(), international: tencentRegion() },
    twilio: {
      international: {
        account_sid: 'ACxxxx',
        auth_token: 'AUTHTOKEN_REAL',
        api_key_sid: 'SKxxxx',
        api_key_secret: 'APIKEY_SECRET_REAL',
        from: '+15005550006',
        messaging_service_sid: 'MGxxxx',
        texts: templateMap(),
      },
    },
    code_length: 6,
    code_ttl: 300,
    resend_interval: 60,
    max_verify_attempts: 5,
    daily_limit_per_phone: 10,
    hourly_limit_per_ip: 10,
    daily_limit_per_ip: 30,
    log_retention_days: 90,
    scenes: [...SCENES],
  };
};

// ① null map 护栏：5 处 map 路径上的 null 一律不进 payload（省略键或 {}）。
// 反向对照会拿「不处理 null」的源码跑同一套断言，必须判红。
const runNullMapAssertions = (build) => {
  const values = makeFullValues();
  MAP_PATHS.forEach((p) => setPath(values, p, null));
  const out = build(values);
  MAP_PATHS.forEach((p) => {
    const v = getPath(out, p);
    assert.ok(
      v === undefined ||
        (v !== null && typeof v === 'object' && !Array.isArray(v)),
      `${p.join('.')} 的 null map 未处理（值=${String(v)}）`,
    );
  });
  return out;
};

const checkReal = (build) => {
  // ① null map 护栏（声明式路径遍历，不写死总数）
  runNullMapAssertions(build);
  // 附加：不得就地污染入参（表单 store 不能被 delete）
  const untouched = makeFullValues();
  MAP_PATHS.forEach((p) => setPath(untouched, p, null));
  build(untouched);
  MAP_PATHS.forEach((p) => {
    assert.strictEqual(
      getPath(untouched, p),
      null,
      `${p.join('.')} 入参被就地修改`,
    );
  });
  console.log('  ① null map 护栏（声明式路径清单）+ 不污染入参：OK');

  // ② 完整子树：只改 aliyun.domestic.sign_name，其余键与国际子树仍在
  const values2 = makeFullValues();
  values2.aliyun.domestic.sign_name = '新签名';
  const out2 = build(values2);
  assert.strictEqual(out2.aliyun.domestic.sign_name, '新签名');
  ['access_key_id', 'access_key_secret', 'endpoint', 'param_name', 'templates'].forEach(
    (key) => {
      assert.deepStrictEqual(
        out2.aliyun.domestic[key],
        values2.aliyun.domestic[key],
        `aliyun.domestic.${key} 丢失`,
      );
    },
  );
  assert.deepStrictEqual(
    out2.aliyun.international,
    values2.aliyun.international,
    'aliyun.international 整棵子树丢失',
  );
  console.log('  ② 完整子树（厂商对象整体替换）：OK');

  // ③ 掩码 ******** 原样保留（不清空、不转空串）
  const values3 = makeFullValues();
  MASK_FIELDS.forEach((p) => setPath(values3, p, MASK));
  const out3 = build(values3);
  MASK_FIELDS.forEach((p) => {
    assert.strictEqual(
      getPath(out3, p),
      MASK,
      `${p.join('.')} 掩码被清空或改写`,
    );
  });
  console.log('  ③ ******** 掩码原样保留：OK');

  // ④ 未生效厂商也提交（provider='aliyun' 时 tencent / twilio 子树仍在）
  const values4 = makeFullValues();
  values4.provider = 'aliyun';
  const out4 = build(values4);
  assert.deepStrictEqual(out4.tencent, values4.tencent, '未生效厂商 tencent 子树丢失');
  assert.deepStrictEqual(out4.twilio, values4.twilio, '未生效厂商 twilio 子树丢失');
  assert.deepStrictEqual(out4.aliyun, values4.aliyun, '生效厂商 aliyun 子树丢失');
  console.log('  ④ 未生效厂商也提交：OK');
};

const checkReverse = (badBuild) => {
  // 合成「不处理 null」的源码：同一套 null map 断言必须判红。
  assert.throws(
    () => runNullMapAssertions(badBuild),
    /null map/,
    '反向对照失效：不处理 null 的源码竟通过了 null map 断言',
  );
  // 且 ②③④ 在坏模块上仍为绿 —— 证明判红只由 null 护栏缺失导致。
  const v2 = makeFullValues();
  v2.aliyun.domestic.sign_name = '新签名';
  assert.deepStrictEqual(badBuild(v2).aliyun, v2.aliyun, '坏模块在②上应仍为绿');
  const v3 = makeFullValues();
  MASK_FIELDS.forEach((p) => setPath(v3, p, MASK));
  assert.strictEqual(
    getPath(badBuild(v3), MASK_FIELDS[0]),
    MASK,
    '坏模块在③上应仍为绿',
  );
  const v4 = makeFullValues();
  v4.provider = 'aliyun';
  assert.deepStrictEqual(badBuild(v4).tencent, v4.tencent, '坏模块在④上应仍为绿');
};

const main = () => {
  assert.ok(
    existsSync(SRC),
    `RED：找不到 ${SRC}（步骤 2 预期 payload.ts 尚未实现）`,
  );
  mkdirSync(TMP_DIR, { recursive: true });
  const realSrc = readFileSync(SRC, 'utf8');

  // 反向对照：把真实源码里的 null 护栏分支整段删掉，合成「不处理 null」的源码。
  const GUARD_RE =
    /if \(parent !== null && parent\[leaf\] === null\) \{\s*delete parent\[leaf\];\s*\}/;
  assert.ok(
    GUARD_RE.test(realSrc),
    '反向对照：源码中未找到 null 护栏分支（无法合成坏源码）',
  );
  const badSrc = realSrc.replace(
    GUARD_RE,
    '/* SMS_NULL_GUARD removed for reverse control */',
  );
  assert.notStrictEqual(badSrc, realSrc, '反向对照：坏源码与真实源码相同');
  writeFileSync(BAD_SRC, badSrc);

  [
    [SRC, OUT_REAL],
    [BAD_SRC, OUT_BAD],
  ].forEach(([input, out]) => {
    execFileSync(
      ESBUILD,
      [input, '--format=cjs', '--platform=node', `--outfile=${out}`],
      { stdio: 'pipe' },
    );
  });

  const realMod = require(OUT_REAL);
  const badMod = require(OUT_BAD);
  assert.equal(
    typeof realMod.buildSmsPayload,
    'function',
    'payload.ts 未导出 buildSmsPayload',
  );
  assert.equal(
    typeof badMod.buildSmsPayload,
    'function',
    '合成坏源码未导出 buildSmsPayload',
  );

  console.log('R123 四条断言（真实实现）：');
  checkReal(realMod.buildSmsPayload);
  console.log('反向对照（合成不处理 null 的源码必红）：');
  checkReverse(badMod.buildSmsPayload);
  console.log('  null map 断言判红 ✔；②③④ 仍为绿 ✔');
  console.log('GREEN：全部通过');
};

try {
  main();
} catch (err) {
  console.error('FAIL:', err && err.message ? err.message : err);
  process.exitCode = 1;
}
