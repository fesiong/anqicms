import { get, post } from './tools';

export async function getWebsiteList(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/website/list',
    params,
    options,
  });
}

export async function getWebsiteInfo(
  params: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/website/info',
    params,
    options,
  });
}

export async function saveWebsiteInfo(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/website/save',
    body,
    options,
  });
}

export async function deleteWebsiteInfo(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/website/delete',
    body,
    options,
  });
}

export async function getSiteInfo(
  params: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/siteinfo',
    params,
    options,
  });
}

export async function getSubsiteAdminLoginUrl(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/website/login',
    body,
    options,
  });
}
