import { get, post } from '../tools';

// 企业 IM 渠道（飞书长连接 / 钉钉 Stream）后台接口。
// 后端路由见 route/manage.go 的 manage.Party("/channels")，统一 {code,msg,data}。

export async function channelList(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels',
    params,
    options,
  });
}

export async function channelDetail(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels/detail',
    params,
    options,
  });
}

export async function channelSave(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/detail',
    body,
    options,
  });
}

export async function channelDelete(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/delete',
    body,
    options,
  });
}

export async function channelStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels/status',
    params,
    options,
  });
}

export async function channelCapabilities(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels/capabilities',
    params,
    options,
  });
}

export async function channelTest(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/test',
    body,
    options,
  });
}

export async function channelBindingToken(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/binding/token',
    body,
    options,
  });
}

export async function channelBindings(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels/bindings',
    params,
    options,
  });
}

export async function channelBindingRevoke(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/binding/revoke',
    body,
    options,
  });
}

export async function channelTargets(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels/targets',
    params,
    options,
  });
}

export async function channelTargetSave(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/target',
    body,
    options,
  });
}

export async function channelRules(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels/rules',
    params,
    options,
  });
}

export async function channelRuleSave(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/rule',
    body,
    options,
  });
}

export async function channelRuleDelete(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/rule/delete',
    body,
    options,
  });
}

export async function channelTemplates(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels/templates',
    params,
    options,
  });
}

export async function channelTemplateSave(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/template',
    body,
    options,
  });
}

export async function channelTemplatePreview(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/template/preview',
    body,
    options,
  });
}

export async function channelRecords(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/channels/records',
    params,
    options,
  });
}

export async function channelDeliveryRetry(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/channels/delivery/retry',
    body,
    options,
  });
}
