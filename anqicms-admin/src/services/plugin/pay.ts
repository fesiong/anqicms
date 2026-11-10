import { get, post } from '../tools';

export async function pluginGetPayConfig(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/pay/config',
    params,
    options,
  });
}

export async function pluginSavePayConfig(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/pay/config',
    body,
    options,
  });
}

export async function pluginGetPayAccounts(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/pay/accounts',
    params,
    options,
  });
}

export async function pluginGetPayAccountStatistic(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/pay/statistic',
    params,
    options,
  });
}

export async function pluginGetPayAccountInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/pay/detail',
    params,
    options,
  });
}

export async function pluginSavePayAccount(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/pay/detail',
    body,
    options,
  });
}

export async function pluginDeletePayAccount(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/pay/delete',
    body,
    options,
  });
}

export async function pluginPayUploadFile(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/pay/upload',
    body,
    options,
  });
}
