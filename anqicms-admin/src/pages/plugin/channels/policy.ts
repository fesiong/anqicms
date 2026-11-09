// 渠道「能力策略」的前端纯逻辑：解析/序列化/摘要/预设。
//
// 存储格式与后端 ParseChannelPolicy 完全一致：
//
//	{"intents":{"<意图>":{"<动作>":"allow|confirm|deny"}}}
//
// 授权粒度是「意图」：风险等级本来就按意图定义，动作只是端点的机械拆分。
// 因此界面一个意图一行，保存时只写意图级通配符 {"*": 裁决}；
// 读取历史手写策略（按动作展开）时逐动作求值，取最严的一项，
// 保证展示与保存都不会放宽任何一个动作。

export type PolicyDecision = 'allow' | 'confirm' | 'deny';

export type PolicyIntents = Record<string, Record<string, PolicyDecision>>;

export type ParsedPolicy = {
  intents: PolicyIntents;
  // valid=false 只表示原文不合法（非对象、JSON 语法错误）；此时 intents 为空，
  // 判定仍是一律拒绝，与运行时 ParseChannelPolicy 的口径一致。
  valid: boolean;
};

export type CapabilityIntent = {
  name: string;
  title?: string;
  risk?: string;
  // default_off 为 true 表示该意图不属于「默认开放」的常规意图（高危或补齐域）。
  default_off?: boolean;
  blocked?: boolean;
  reason?: string;
  agent_allowed?: boolean;
};

export type PolicyRow = {
  // key 就是意图名：一行一个意图，不再展开动作。
  key: string;
  intent: string;
  title: string;
  risk: string;
  defaultOff: boolean;
  blocked: boolean;
  reason: string;
  agentAllowed?: boolean;
  decision: PolicyDecision;
};

export type PolicySummary = {
  intents: number;
  allow: number;
  confirm: number;
};

const DECISIONS: PolicyDecision[] = ['allow', 'confirm', 'deny'];

// 越严越小：合并同一意图下多个动作的裁决时取最严的一项。
const RESTRICTION: Record<PolicyDecision, number> = {
  deny: 0,
  confirm: 1,
  allow: 2,
};

const mostRestrictive = (decisions: PolicyDecision[]): PolicyDecision => {
  if (decisions.length === 0) {
    return 'deny';
  }
  let result: PolicyDecision = decisions[0];
  for (const decision of decisions) {
    if (RESTRICTION[decision] < RESTRICTION[result]) {
      result = decision;
    }
  }
  return result;
};

const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

export const parsePolicy = (raw: string): ParsedPolicy => {
  const text = String(raw || '').trim();
  if (text === '') {
    return { intents: {}, valid: true };
  }
  let payload: unknown = null;
  try {
    payload = JSON.parse(text);
  } catch (e) {
    return { intents: {}, valid: false };
  }
  if (!isPlainObject(payload)) {
    return { intents: {}, valid: false };
  }
  const rawIntents = (payload as Record<string, unknown>).intents;
  if (rawIntents === undefined) {
    return { intents: {}, valid: true };
  }
  if (!isPlainObject(rawIntents)) {
    return { intents: {}, valid: false };
  }
  const intents: PolicyIntents = {};
  for (const [rawName, rawActions] of Object.entries(rawIntents)) {
    const name = rawName.trim();
    if (name === '' || !isPlainObject(rawActions)) {
      continue;
    }
    const actions: Record<string, PolicyDecision> = {};
    for (const [rawAction, rawDecision] of Object.entries(rawActions)) {
      const action = rawAction.trim();
      if (action === '') {
        continue;
      }
      if (typeof rawDecision !== 'string') {
        continue;
      }
      const decision = rawDecision.trim() as PolicyDecision;
      if (!DECISIONS.includes(decision)) {
        continue;
      }
      actions[action] = decision;
    }
    if (Object.keys(actions).length > 0) {
      intents[name] = actions;
    }
  }
  return { intents, valid: true };
};

// resolveIntentDecision 把「意图 + 动作」策略收敛成意图级裁决。
// 有 "*" 时它就是缺省裁决，显式动作只会更具体；两者不一致（历史手写策略）
// 时取最严的一项。没有 "*" 时未列出的动作按拒绝计，因此整体也是拒绝。
export const resolveIntentDecision = (
  intents: PolicyIntents,
  intent: string,
): PolicyDecision => {
  const actions = intents[intent];
  if (!actions) {
    return 'deny';
  }
  const decisions: PolicyDecision[] = [actions['*'] || 'deny'];
  for (const [action, decision] of Object.entries(actions)) {
    if (action !== '*') {
      decisions.push(decision);
    }
  }
  return mostRestrictive(decisions);
};

export const buildPolicyRows = (
  capabilities: CapabilityIntent[],
  raw: string,
): PolicyRow[] => {
  const { intents } = parsePolicy(raw);
  const rows: PolicyRow[] = [];
  for (const item of capabilities || []) {
    const name = String(item?.name || '').trim();
    if (name === '') {
      continue;
    }
    rows.push({
      key: name,
      intent: name,
      title: item.title || name,
      risk: item.risk || '',
      defaultOff: item.default_off === true,
      blocked: item.blocked === true,
      reason: item.reason || '',
      agentAllowed: item.agent_allowed,
      decision: resolveIntentDecision(intents, name),
    });
  }
  return rows;
};

// serializePolicy 只写意图级通配符：拒绝是缺省值，写出来只会让策略变长；
// 被硬拦截的意图永远不落策略——它们本就不受策略控制，写进去只会让界面显得「配置生效」。
export const serializePolicy = (rows: PolicyRow[]): string => {
  const intents: PolicyIntents = {};
  for (const row of rows || []) {
    if (row.blocked || row.decision === 'deny') {
      continue;
    }
    intents[row.intent] = { '*': row.decision };
  }
  if (Object.keys(intents).length === 0) {
    return '{}';
  }
  return JSON.stringify({ intents });
};

export const summarizePolicyRaw = (raw: string): PolicySummary => {
  const { intents } = parsePolicy(raw);
  const summary: PolicySummary = { intents: 0, allow: 0, confirm: 0 };
  for (const intent of Object.keys(intents)) {
    summary.intents += 1;
    const decision = resolveIntentDecision(intents, intent);
    if (decision === 'allow') {
      summary.allow += 1;
    } else if (decision === 'confirm') {
      summary.confirm += 1;
    }
  }
  return summary;
};

export type PolicyPreset = 'deny' | 'readonly' | 'regular' | 'allow';

// applyPolicyPreset 只改界面状态，仍需管理员点保存才落库。
// 硬拦截的意图始终拒绝：它们由代码层固定关闭，给它们配 confirm/allow 是误导。
// regular = 放行默认开放的常规意图（与 MCP 默认可见面同一口径），默认关闭的意图保持拒绝。
export const applyPolicyPreset = (
  rows: PolicyRow[],
  preset: PolicyPreset,
): PolicyRow[] =>
  (rows || []).map((row) => {
    if (row.blocked) {
      return { ...row, decision: 'deny' as PolicyDecision };
    }
    if (preset === 'deny') {
      return { ...row, decision: 'deny' as PolicyDecision };
    }
    if (preset === 'allow') {
      return { ...row, decision: 'allow' as PolicyDecision };
    }
    if (preset === 'regular') {
      return {
        ...row,
        decision: (row.defaultOff ? 'deny' : 'allow') as PolicyDecision,
      };
    }
    return {
      ...row,
      decision: (row.risk === 'read' ? 'allow' : 'deny') as PolicyDecision,
    };
  });
