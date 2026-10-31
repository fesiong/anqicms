import { get, post } from '../tools';

export async function pluginGetLimiterSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/limiter/setting',
    params,
    options,
  });
}

export async function pluginSaveLimiterSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/limiter/setting',
    body,
    options,
  });
}

export async function pluginGetBlockedIPs(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/limiter/blockedips',
    params,
    options,
  });
}

export async function pluginRemoveBlockedIP(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/limiter/blockedip/remove',
    body,
    options,
  });
}
