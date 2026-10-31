import { get, post } from '../tools';

export async function pluginGetHtmlCache(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/htmlcache/config',
    params,
    options,
  });
}

export async function pluginSaveHtmlCache(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/config',
    body,
    options,
  });
}

export async function pluginBuildHtmlIndexCache(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/build/index',
    body,
    options,
  });
}

export async function pluginBuildHtmlCategoryCache(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/build/category',
    body,
    options,
  });
}

export async function pluginBuildHtmlArchiveCache(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/build/archive',
    body,
    options,
  });
}

export async function pluginBuildHtmlTagCache(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/build/tag',
    body,
    options,
  });
}

export async function pluginBuildHtmlCache(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/build',
    body,
    options,
  });
}

export async function pluginGetBuildHtmlCacheStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/htmlcache/build/status',
    params,
    options,
  });
}

export async function pluginCleanHtmlCache(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/clean',
    body,
    options,
  });
}

export async function pluginHtmlCacheUploadFile(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/upload',
    body,
    options,
  });
}

export async function pluginHtmlCachePush(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/htmlcache/push',
    body,
    options,
  });
}

export async function pluginGetHtmlCachePushStatus(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/htmlcache/push/status',
    params,
    options,
  });
}

export async function pluginGetHtmlCachePushLogs(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/htmlcache/push/logs',
    params,
    options,
  });
}
