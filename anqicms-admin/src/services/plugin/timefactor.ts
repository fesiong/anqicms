import { get, post } from '../tools';

export async function pluginGetTimefactorSetting(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/timefactor/setting',
    params,
    options,
  });
}

export async function pluginSaveTimefactorSetting(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/timefactor/setting',
    body,
    options,
  });
}
