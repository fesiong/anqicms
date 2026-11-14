// 提交短信配置前的 payload 护栏（规格 §3.5 / §8.3，任务 14 R123）。
// 纯函数，不依赖 React，可单独用 esbuild 转译后交给 node:assert 验证。

// 场景模板 / 文案 map 的声明式路径清单，按 config/sms.go 实际结构穷举。
// 不写死总数：新增厂商或区域时只需在此补一条路径。
// 后端 MergeJSONPayload 只处理顶层字段，厂商对象是整体替换；若这些 map
// 以 null 进入 payload，json.Unmarshal 会把站长已配的模板静默清空。
const NULL_MAP_PATHS: string[][] = [
  ['aliyun', 'domestic', 'templates'],
  ['aliyun', 'international', 'templates'],
  ['tencent', 'domestic', 'templates'],
  ['tencent', 'international', 'templates'],
  ['twilio', 'international', 'texts'],
];

// 深拷贝：绝不就地把 null 从表单 store 里删掉（那会污染 getFieldsValue(true) 的源数据）。
function cloneValue(value: any): any {
  if (Array.isArray(value)) {
    return value.map((item) => cloneValue(item));
  }
  if (value !== null && typeof value === 'object') {
    const copy: Record<string, any> = {};
    Object.keys(value).forEach((key) => {
      copy[key] = cloneValue(value[key]);
    });
    return copy;
  }
  return value;
}

function parentAt(root: Record<string, any>, parts: string[]): Record<string, any> | null {
  let node: any = root;
  for (let i = 0; i < parts.length - 1; i += 1) {
    if (node === null || typeof node !== 'object') {
      return null;
    }
    node = node[parts[i]];
  }
  return node !== null && typeof node === 'object' ? node : null;
}

/**
 * 把 ProForm.getFieldsValue(true) 的全量表单值整理成保存端点的 payload：
 * 三个厂商对象必须带完整子树提交（未生效厂商也提交），凭据掩码 ******** 原样保留，
 * 仅把值为 null 的 templates / texts map 从 payload 中省略。
 */
export function buildSmsPayload(values: Record<string, any>): Record<string, any> {
  const payload = cloneValue(values) as Record<string, any>;
  // 命中 null 的 map 一律省略该键，避免后端静默清空已配模板。
  NULL_MAP_PATHS.forEach((path) => {
    const parent = parentAt(payload, path);
    const leaf = path[path.length - 1];
    if (parent !== null && parent[leaf] === null) {
      delete parent[leaf];
    }
  });
  return payload;
}
