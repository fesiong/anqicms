import { get, post } from './tools';

export async function anqiLogin(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/anqi/login',
    body,
    options,
  });
}

// 后台内嵌注册：账号在后台完成创建，成功后直接进入登录态，不跳官网。
export async function anqiRegister(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/anqi/register',
    body,
    options,
  });
}

// 充值套餐：价格由官网下发，客户端不写死。
export async function getAnqiPackages(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/packages',
    params,
    options,
  });
}

// 下单：返回微信二维码或收银台跳转地址，资金由官网代收。
export async function anqiRecharge(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/recharge',
    body,
    options,
  });
}

// 支付结果轮询（内网部署收不到官网回调）。
export async function anqiRechargeStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/recharge/status',
    params,
    options,
  });
}

export async function getAnqiInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/info',
    params,
    options,
  });
}

export async function checkAnqiInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/check',
    params,
    options,
  });
}

export async function anqiUpload(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/anqi/upload',
    body,
    options,
  });
}

export async function anqiShareTemplate(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/template/share',
    body,
    options,
  });
}

export async function anqiDownloadTemplate(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/template/download',
    body,
    options,
  });
}

export async function anqiSendFeedback(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/feedback',
    body,
    options,
  });
}

export async function anqiRestart(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/anqi/restart',
    body,
    options,
  });
}

export async function anqiTranslateArchive(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/translate',
    body,
    options,
  });
}

export async function anqiAiPseudoArchive(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/ai/pseudo',
    body,
    options,
  });
}

export async function anqiAiGenerateStream(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/ai/stream',
    body,
    options,
  });
}

export async function anqiAiGenerateStreamData(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/ai/stream/data',
    params,
    options,
  });
}

export async function anqiAiChat(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/anqi/ai/chat',
    body,
    options,
  });
}

// P0: 工具执行审批 — 主会话写操作需要前端确认
// decision: "allow" 本次允许 | "deny" 拒绝 | "once_allow" 本会话允许
export async function anqiAiToolConfirm(
  body: {
    tool_call_id: string;
    decision: 'allow' | 'deny' | 'once_allow' | 'full_control';
  },
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/ai/chat/confirm',
    body,
    options,
  });
}

// P2-7: 执行链路追踪 — 返回最近的结构化追踪事件 (GET /anqi/ai/chat/trace)
export async function anqiAiChatTrace(
  params?: { limit?: number },
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/ai/chat/trace',
    params,
    options,
  });
}

export async function anqiAiImageGenerate(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/ai/image',
    body,
    options,
  });
}

export async function anqiAiImageConfirm(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/ai/image/confirm',
    body,
    options,
  });
}

export async function anqiAiImageHistories(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/ai/image/histories',
    params,
    options,
  });
}

export async function anqiExtractDescription(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/extract/description',
    body,
    options,
  });
}

export async function anqiExtractKeywords(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/extract/keywords',
    body,
    options,
  });
}

export async function getAiHistories(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/ai/history',
    params,
    options,
  });
}

export async function getAiSessions(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/ai/sessions',
    params,
    options,
  });
}

export async function anqiAiUpload(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/ai/upload',
    body,
    options,
  });
}

export async function getAiSettings(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/ai/settings',
    params,
    options,
  });
}

export async function saveAiSettings(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/ai/settings',
    body,
    options,
  });
}

export async function getAiAgents(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/ai/agents',
    params,
    options,
  });
}

// ── Skill 管理 ──

export async function getSkillList(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/skill/list',
    params,
    options,
  });
}

export async function getSkillDetail(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/anqi/skill/detail',
    params,
    options,
  });
}

export async function anqiSkillEdit(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/skill/edit',
    body,
    options,
  });
}

export async function anqiSkillDelete(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/skill/delete',
    body,
    options,
  });
}

export async function anqiSkillReload(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/anqi/skill/reload',
    body,
    options,
  });
}

export async function getAiAgentLogs(
  agentId: number,
  options?: { [key: string]: any },
) {
  return get({
    url: `/anqi/ai/agents/${agentId}/logs`,
    options,
  });
}
