import { get, post } from '../tools';

export async function pluginGetWechatConfig(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/wechat/config',
    params,
    options,
  });
}

export async function pluginSaveWechatConfig(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/wechat/config',
    body,
    options,
  });
}

export async function pluginGetWechatMessages(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/wechat/message/list',
    params,
    options,
  });
}

export async function pluginDeleteWechatMessage(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/wechat/message/delete',
    body,
    options,
  });
}

export async function pluginReplyWechatMessage(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/wechat/message/reply',
    body,
    options,
  });
}

export async function pluginGetWechatReplyRules(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/wechat/reply/rule/list',
    params,
    options,
  });
}

export async function pluginDeleteWechatReplyRule(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/wechat/reply/rule/delete',
    body,
    options,
  });
}

export async function pluginSaveWechatReplyRule(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/wechat/reply/rule/save',
    body,
    options,
  });
}

export async function pluginGetWechatMenus(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/wechat/menu/list',
    params,
    options,
  });
}

export async function pluginDeleteWechatMenu(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/wechat/menu/delete',
    body,
    options,
  });
}

export async function pluginSaveWechatMenu(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/wechat/menu/save',
    body,
    options,
  });
}

export async function pluginSyncWechatMenu(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/wechat/menu/sync',
    body,
    options,
  });
}
