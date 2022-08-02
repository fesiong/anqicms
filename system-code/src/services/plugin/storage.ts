import { get, post } from '../tools';

export async function pluginGetStorage(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/storage',
    params,
    options,
  });
}

export async function pluginSaveStorage(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/storage',
    body,
    options,
  });
}
