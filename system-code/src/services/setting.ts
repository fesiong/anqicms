import { get, post } from './tools';

export async function getSettingSystem(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/setting/system',
    params,
    options,
  });
}

export async function getSettingContent(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/setting/content',
    params,
    options,
  });
}

export async function getSettingIndex(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/setting/index',
    params,
    options,
  });
}

export async function getSettingNav(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/setting/nav',
    params,
    options,
  });
}

export async function getSettingNavTypes(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/setting/nav/type',
    params,
    options,
  });
}

export async function getSettingContact(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/setting/contact',
    params,
    options,
  });
}

export async function getSettingCache(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/setting/cache',
    params,
    options,
  });
}

export async function saveSettingSystem(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/system',
    body,
    options,
  });
}

export async function saveSettingContent(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/content',
    body,
    options,
  });
}

export async function rebuildThumb(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/thumb/rebuild',
    body,
    options,
  });
}

export async function saveSettingIndex(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/index',
    body,
    options,
  });
}

export async function saveSettingNav(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/nav',
    body,
    options,
  });
}

export async function deleteSettingNav(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/nav/delete',
    body,
    options,
  });
}

export async function saveSettingNavType(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/nav/type',
    body,
    options,
  });
}

export async function deleteSettingNavType(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/nav/type/delete',
    body,
    options,
  });
}

export async function saveSettingContact(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/contact',
    body,
    options,
  });
}

export async function saveSettingCache(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/cache',
    body,
    options,
  });
}

export async function convertImagetoWebp(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/setting/convert/webp',
    body,
    options,
  });
}
