import { get, post } from '../tools';

export async function pluginGetImportApiSetting(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/import/api',
    params,
    options,
  });
}

export async function pluginUpdateApiToken(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/import/token',
    body,
    options,
  });
}
