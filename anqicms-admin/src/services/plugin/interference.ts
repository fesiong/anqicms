import { get, post } from '../tools';

export async function pluginGetInterferenceConfig(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/interference/config',
    params,
    options,
  });
}

export async function pluginSaveInterferenceConfig(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/interference/config',
    body,
    options,
  });
}
