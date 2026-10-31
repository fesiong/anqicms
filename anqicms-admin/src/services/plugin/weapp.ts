import { get, post } from '../tools';

export async function pluginGetWeappConfig(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/weapp/config',
    params,
    options,
  });
}

export async function pluginSaveWeappConfig(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/weapp/config',
    body,
    options,
  });
}
