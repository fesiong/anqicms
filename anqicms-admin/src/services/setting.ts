import { get, post } from './tools';

export async function getSettingSystem(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/system',
    params,
    options,
  });
}

export async function getSettingShop(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/shop',
    params,
    options,
  });
}

export async function getSettingContent(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/content',
    params,
    options,
  });
}

export async function getSettingIndex(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/index',
    params,
    options,
  });
}

export async function getSettingNav(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/nav',
    params,
    options,
  });
}

export async function getSettingNavTypes(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/nav/type',
    params,
    options,
  });
}

export async function getSettingContact(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/contact',
    params,
    options,
  });
}

export async function getSettingCache(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/cache',
    params,
    options,
  });
}

export async function getSettingSafe(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/safe',
    params,
    options,
  });
}

export async function getSettingDiyFields(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/diyfield',
    params,
    options,
  });
}

export async function saveSettingSystem(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/system',
    body,
    options,
  });
}

export async function saveSettingShop(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/shop',
    body,
    options,
  });
}

export async function saveSettingContent(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/content',
    body,
    options,
  });
}

export async function rebuildThumb(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/thumb/rebuild',
    body,
    options,
  });
}

export async function saveSettingIndex(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/index',
    body,
    options,
  });
}

export async function saveSettingNav(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/nav',
    body,
    options,
  });
}

export async function deleteSettingNav(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/nav/delete',
    body,
    options,
  });
}

export async function saveSettingNavType(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/nav/type',
    body,
    options,
  });
}

export async function deleteSettingNavType(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/nav/type/delete',
    body,
    options,
  });
}

export async function saveSettingContact(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/contact',
    body,
    options,
  });
}

export async function saveSettingCache(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/cache',
    body,
    options,
  });
}

export async function saveSettingMigrateDB(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/migratedb',
    body,
    options,
  });
}

export async function convertImagetoWebp(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/convert/webp',
    body,
    options,
  });
}

export async function saveSettingSafe(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/safe',
    body,
    options,
  });
}

export async function saveSystemFavicon(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/favicon',
    body,
    options,
  });
}

export async function deleteSystemFavicon(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/favicon/delete',
    body,
    options,
  });
}

export async function getSettingBanners(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/banner',
    params,
    options,
  });
}

export async function saveSettingBanner(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/banner',
    body,
    options,
  });
}

export async function deleteSettingBanner(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/banner/delete',
    body,
    options,
  });
}

export async function getSettingSensitiveWords(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/sensitive/words',
    params,
    options,
  });
}

export async function saveSettingSensitiveWords(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/sensitive/words',
    body,
    options,
  });
}

export async function checkSettingSensitiveWords(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/sensitive/check',
    body,
    options,
  });
}

export async function syncSettingSensitiveWords(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/sensitive/sync',
    body,
    options,
  });
}

export async function saveSettingDiyFields(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/diyfield',
    body,
    options,
  });
}

export async function getSettingAi(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/ai',
    params,
    options,
  });
}

export async function getSettingCurrency(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/setting/currency',
    params,
    options,
  });
}

export async function saveSettingAi(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/ai',
    body,
    options,
  });
}

export async function saveSettingCurrency(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/setting/currency',
    body,
    options,
  });
}
