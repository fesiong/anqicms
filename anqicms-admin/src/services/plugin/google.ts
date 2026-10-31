import { get, post } from '../tools';

export async function pluginGetGoogleSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/google/setting',
    params,
    options,
  });
}

export async function pluginSaveGoogleSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/google/setting',
    body,
    options,
  });
}
