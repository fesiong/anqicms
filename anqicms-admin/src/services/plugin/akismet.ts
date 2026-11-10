import { get, post } from '../tools';

export async function pluginGetAkismetSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/akismet/setting',
    params,
    options,
  });
}

export async function pluginSaveAkismetSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/akismet/setting',
    body,
    options,
  });
}
