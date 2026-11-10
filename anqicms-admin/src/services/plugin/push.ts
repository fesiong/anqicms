import { get, post } from '../tools';

export async function pluginGetPush(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/push',
    params,
    options,
  });
}

export async function pluginGetPushLogs(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/push/logs',
    params,
    options,
  });
}

export async function pluginSavePush(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/push',
    body,
    options,
  });
}
