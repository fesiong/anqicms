const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');
const { buildSync } = require('esbuild');
const filename = path.join(__dirname, 'types.ts');
const output = buildSync({
  entryPoints: [filename],
  bundle: true,
  platform: 'node',
  format: 'cjs',
  write: false,
});
const loaded = new Module(filename, module);
loaded._compile(output.outputFiles[0].text, filename);
const texts = loaded.exports;

test('types.ts 存在且导出了条目模型的关键方法', () => {
  assert.ok(fs.existsSync(filename));
  [
    'normalizeTextsItems',
    'serializeTextsItems',
    'normalizeTextsItemType',
    'isMultiValueType',
    'parseTextsOptions',
    'createTextsItem',
    'changeTextsItemType',
    'textsItemDisplayText',
  ].forEach((name) => assert.equal(typeof texts[name], 'function', name));
});

test('历史数据（无 type）按 text 解析，且回写时不新增 type 字段', () => {
  const legacy = [{ key: '品牌', value: '', values: ['安企', 'AnQi'] }];
  const items = texts.normalizeTextsItems(legacy);
  assert.equal(items.length, 1);
  assert.equal(items[0].type, '');
  assert.deepEqual(items[0].values, ['安企', 'AnQi']);
  assert.equal(texts.textsItemDisplayText(items[0]), '安企\nAnQi');

  const payload = texts.serializeTextsItems(items);
  assert.equal('type' in payload[0], false);
  assert.deepEqual(payload[0].values, ['安企', 'AnQi']);
});

test('CustomField 包装体与 JSON 字符串都能解析', () => {
  const wrapped = { name: '参数', type: 'texts', value: [{ key: 'k', value: 'v' }] };
  assert.equal(texts.normalizeTextsItems(wrapped)[0].key, 'k');
  const json = JSON.stringify([{ key: 'a', value: '1', type: 'number' }]);
  const parsed = texts.normalizeTextsItems(json);
  assert.equal(parsed[0].type, 'number');
  assert.equal(parsed[0].value, '1');
  // 坏 JSON 不能抛异常
  assert.deepEqual(texts.normalizeTextsItems('not-json'), []);
});

test('类型归一化：空/非法按 text，多值类型判定正确', () => {
  assert.equal(texts.normalizeTextsItemType(''), 'text');
  assert.equal(texts.normalizeTextsItemType('NUMBER'), 'number');
  assert.equal(texts.normalizeTextsItemType('images'), 'images');
  // 编辑器/嵌套 texts 这类在条目层无意义的类型仍按 text 兜底
  assert.equal(texts.normalizeTextsItemType('editor'), 'text');
  assert.equal(texts.normalizeTextsItemType('texts'), 'text');
  assert.equal(texts.isMultiValueType('text'), true);
  assert.equal(texts.isMultiValueType('textarea'), true);
  assert.equal(texts.isMultiValueType('checkbox'), true);
  assert.equal(texts.isMultiValueType('images'), true);
  assert.equal(texts.isMultiValueType('select'), false);
  assert.equal(texts.isMultiValueType('number'), false);
  assert.equal(texts.isMultiValueType('image'), false);
});

test('选项解析：按行去空白去重，保持顺序', () => {
  assert.deepEqual(texts.parseTextsOptions('A\n  B \n\nA\n'), ['A', 'B']);
  assert.deepEqual(texts.parseTextsOptions(undefined), []);
});

test('切换类型不丢已填内容：单值↔多值互相补齐', () => {
  const base = texts.createTextsItem();
  const withText = { ...base, values: ['第一行', '第二行'] };
  const asNumber = texts.changeTextsItemType(withText, 'number');
  assert.equal(asNumber.value, '第一行');
  // 切回文本时 values 还在
  const backToText = texts.changeTextsItemType(asNumber, 'text');
  assert.deepEqual(backToText.values, ['第一行', '第二行']);

  const onlyValue = { ...base, value: '14:30' };
  const asTime = texts.changeTextsItemType(onlyValue, 'time');
  assert.equal(asTime.value, '14:30');

  const asCheckbox = texts.changeTextsItemType(onlyValue, 'checkbox');
  // 多值类型至少要有一个空占位，UI 才有可填的行
  assert.deepEqual(asCheckbox.values, ['']);
  assert.equal(asCheckbox.value, '14:30');
});

test('checkbox 展示用逗号，text 展示用换行', () => {
  assert.equal(
    texts.textsItemDisplayText({ type: 'checkbox', values: ['A', 'B'] }),
    'A, B',
  );
  assert.equal(texts.textsItemDisplayText({ type: 'textarea', values: ['x', 'y'] }), 'x\ny');
  assert.equal(texts.textsItemDisplayText({ type: 'select', value: 'A' }), 'A');
  // 多值类型 values 全空时退回 value
  assert.equal(texts.textsItemDisplayText({ value: '兜底', values: [''] }), '兜底');
});

test('序列化保留空行占位（新增值当场可见）', () => {
  const payload = texts.serializeTextsItems([
    { key: '', value: '', values: ['a', ''], type: '' },
  ]);
  assert.deepEqual(payload[0].values, ['a', '']);
  assert.equal('type' in payload[0], false);
});

test('images：多张图走 values，value 兜首图，展示用逗号', () => {
  const items = texts.normalizeTextsItems([
    { key: '相册', type: 'images', values: ['/uploads/a.png', '/uploads/b.png'] },
  ]);
  assert.equal(items[0].type, 'images');
  assert.deepEqual(items[0].values, ['/uploads/a.png', '/uploads/b.png']);
  assert.equal(texts.textsItemDisplayText(items[0]), '/uploads/a.png, /uploads/b.png');

  // 只有单值（旧数据/切类型过来的）要派生出 values，否则组图编辑器看不到图
  const legacy = texts.normalizeTextsItems([
    { key: '相册', type: 'images', value: '/uploads/only.png' },
  ]);
  assert.deepEqual(legacy[0].values, ['/uploads/only.png']);

  // 切到组图时，原来的单图要保留下来
  const switched = texts.changeTextsItemType(
    { key: 'k', value: '/uploads/one.png', values: [], type: 'image' },
    'images',
  );
  assert.deepEqual(switched.values, ['/uploads/one.png']);
});

test('images 全部删除后不能把旧首图带回去', () => {
  // 编辑器删图时同时清 value；序列化也不能回落 item.value，
  // 否则后端 SyncValues 会把已删除的图重新派生回 values。
  const payload = texts.serializeTextsItems([
    { key: '相册', value: '/uploads/a.png', values: [], type: 'images' },
  ]);
  assert.equal(payload[0].value, '');
  assert.deepEqual(payload[0].values, []);
  assert.equal(payload[0].type, 'images');
});
