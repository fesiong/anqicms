import { get, post } from '../tools';

export async function pluginGetMultiLangConfig(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/multilang/config',
    params,
    options,
  });
}

export async function pluginSaveMultiLangConfig(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/multilang/config',
    body,
    options,
  });
}

export async function pluginGetMultiLangSites(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/multilang/sites',
    params,
    options,
  });
}

export async function pluginGetMultiLangValidSites(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/multilang/validsites',
    params,
    options,
  });
}

export async function pluginRemoveMultiLangSite(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/multilang/site/remove',
    body,
    options,
  });
}

export async function pluginSaveMultiLangSite(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/multilang/site/save',
    body,
    options,
  });
}

export async function pluginSyncMultiLangSiteContent(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/multilang/site/sync',
    body,
    options,
  });
}

export async function pluginGetMultiLangSyncStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/multilang/site/sync/status',
    params,
    options,
  });
}

export async function pluginGetMultiLangTranslateHtmlLog(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/multilang/site/html/logs',
    params,
    options,
  });
}

export async function pluginGetMultiLangTranslateHtmlCache(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/multilang/site/html/caches',
    params,
    options,
  });
}

export async function pluginMultiLangTranslateHtmlCacheRemove(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/multilang/site/html/cache/remove',
    body,
    options,
  });
}
