import { get, post } from './tools';

export async function getVersion(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/version/info',
    params,
    options,
  });
}

export async function checkVersion(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/version/check',
    params,
    options,
  });
}

export async function upgradeVersion(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/version/upgrade',
    body,
    options,
  });
}
