// 提交短信配置前的 payload 护栏（规格 §3.5 / §8.3，任务 14 R123）。
// 纯函数，不依赖 React，可单独用 esbuild 转译后交给 node:assert 验证。

// 场景模板 / 文案 map 的声明式路径清单，按 config/sms.go 实际结构穷举。
// 不写死总数：新增厂商或区域时只需在此补一条路径。
// 后端 MergeJSONPayload 是深合并（未出现的键保留原值），但 map 字段一旦出现就
// 整段替换；若这些 map 以 null 进入 payload，json.Unmarshal 会把站长已配的模板
// 静默清空。
const NULL_MAP_PATHS: string[][] = [
  ['aliyun', 'domestic', 'templates'],
  ['aliyun', 'international', 'templates'],
  ['tencent', 'domestic', 'templates'],
  ['tencent', 'international', 'templates'],
  ['twilio', 'international', 'texts'],
];

// IPv4 段：与 Go 的 netip.ParseAddr 同口径——不接受前导零（01.2.3.4 在服务端会被
// 静默跳过），也不接受 >255。
function parseIPv4(addr: string): number[] | null {
  const parts = addr.split('.');
  if (parts.length !== 4) {
    return null;
  }
  const nums: number[] = [];
  for (const part of parts) {
    if (!/^(0|[1-9]\d{0,2})$/.test(part)) {
      return null;
    }
    const num = Number(part);
    if (num > 255) {
      return null;
    }
    nums.push(num);
  }
  return nums;
}

// 数 IPv6 的 16 位组个数（合法返回 1..8，非法返回 null）。允许末尾嵌入 IPv4
// （如 ::ffff:1.2.3.4，占两组）；'::' 必须压缩掉至少一组，与非压缩写法的 8 组互斥。
function ipv6GroupCount(addr: string): number | null {
  if (addr === '' || !/^[0-9a-fA-F:.]+$/.test(addr)) {
    return null;
  }
  const halves = addr.split('::');
  if (halves.length > 2) {
    return null;
  }
  const groups: string[] = [];
  halves.forEach((half) => {
    if (half === '') {
      return;
    }
    half.split(':').forEach((seg) => groups.push(seg));
  });
  if (groups.some((seg) => seg === '')) {
    return null;
  }
  let count = 0;
  for (let i = 0; i < groups.length; i += 1) {
    const seg = groups[i];
    const isLast = i === groups.length - 1;
    if (isLast && seg.includes('.')) {
      if (parseIPv4(seg) === null) {
        return null;
      }
      count += 2;
    } else if (/^[0-9a-fA-F]{1,4}$/.test(seg)) {
      count += 1;
    } else {
      return null;
    }
  }
  if (halves.length === 2) {
    return count < 8 ? count : null;
  }
  return count === 8 ? count : null;
}

/**
 * 校验 `trusted_proxies` 的一项：单个 IP 或 CIDR（IPv4 / IPv6）。
 *
 * 为什么必须在前端拦：后端 `smsParseTrustedProxies` 对非法项是**静默跳过**的，
 * 一项写错（比如 /33、01.2.3.4、少一段的 IPv6）不会有任何报错，站长只会看到
 * 「IP 限额好像没生效」或「莫名被限流」，排查成本极高。
 */
export function isValidProxyEntry(raw: any): boolean {
  const entry = String(raw ?? '').trim();
  if (entry === '' || entry.includes(' ')) {
    return false;
  }
  const slash = entry.indexOf('/');
  const addr = slash === -1 ? entry : entry.slice(0, slash);
  if (slash !== -1 && entry.indexOf('/', slash + 1) !== -1) {
    return false;
  }
  let maxPrefix: number;
  if (addr.includes(':')) {
    if (ipv6GroupCount(addr) === null) {
      return false;
    }
    maxPrefix = 128;
  } else if (parseIPv4(addr) !== null) {
    maxPrefix = 32;
  } else {
    return false;
  }
  if (slash === -1) {
    return true;
  }
  const prefixText = entry.slice(slash + 1);
  // 前导零同样与 netip.ParsePrefix 不一致（/08 服务端会跳过），一并拒掉。
  if (!/^(0|[1-9]\d{0,2})$/.test(prefixText)) {
    return false;
  }
  const prefix = Number(prefixText);
  return prefix >= 0 && prefix <= maxPrefix;
}

// /0 等于「信任全世界」：任意客户端自带 X-Forwarded-For 都会被采信，正是这个功能
// 要堵的漏洞。后端不做白名单校验，只在前端拦一道。
export function isOverBroadProxyEntry(raw: any): boolean {
  const entry = String(raw ?? '').trim();
  const slash = entry.lastIndexOf('/');
  return slash !== -1 && Number(entry.slice(slash + 1)) === 0;
}

/**
 * 归一化 `trusted_proxies`：去空白、丢空项、去重（保持输入顺序）。
 *
 * 未加载 / 未改动（undefined / null）时返回 undefined，表示「整个键不出现」，
 * 后端深合并会保留原值；而 `[]` 是明确的「清空」，两者语义不同，不能合并处理。
 */
export function normalizeTrustedProxies(value: any): string[] | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  const list: any[] = Array.isArray(value)
    ? value
    : String(value).split(/[\s,，]+/);
  const out: string[] = [];
  list.forEach((item) => {
    const entry = String(item ?? '').trim();
    if (entry === '' || out.includes(entry)) {
      return;
    }
    out.push(entry);
  });
  return out;
}

/** 返回第一个格式非法的项（用于给用户指出是哪一项错了），全部合法返回 null。 */
export function findInvalidProxyEntry(list: any): string | null {
  const items = Array.isArray(list) ? list : [];
  for (const item of items) {
    const entry = String(item ?? '').trim();
    if (entry !== '' && !isValidProxyEntry(entry)) {
      return entry;
    }
  }
  return null;
}

/** 返回第一个 /0 项（信任全部来源），没有返回 null。 */
export function findOverBroadProxyEntry(list: any): string | null {
  const items = Array.isArray(list) ? list : [];
  for (const item of items) {
    const entry = String(item ?? '').trim();
    if (entry !== '' && isValidProxyEntry(entry) && isOverBroadProxyEntry(entry)) {
      return entry;
    }
  }
  return null;
}

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
 * 按全量提交（未生效厂商也提交，与仓库既有 tab 式设置页一致），凭据掩码 ********
 * 原样保留，仅把值为 null 的 templates / texts map 从 payload 中省略。
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
  // trusted_proxies 是 []string：归一化后写回；undefined（未加载/未改动）时删键。
  // 注意 [] 与「键不出现」语义不同——[] 表示清空，键不出现表示保持后端原值。
  const proxies = normalizeTrustedProxies(payload.trusted_proxies);
  if (proxies === undefined) {
    delete payload.trusted_proxies;
  } else {
    payload.trusted_proxies = proxies;
  }
  return payload;
}
