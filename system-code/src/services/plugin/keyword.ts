import { get, post } from '../tools';

export async function pluginGetKeywords(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/keyword/list',
    params,
    options,
  });
}

export async function pluginSaveKeyword(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/keyword/detail',
    body,
    options,
  });
}

export async function pluginDeleteKeyword(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/keyword/delete',
    body,
    options,
  });
}

export async function pluginExportKeyword(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/keyword/export',
    body,
    options,
  });
}

export async function pluginImportKeyword(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/keyword/import',
    body,
    options,
  });
}
