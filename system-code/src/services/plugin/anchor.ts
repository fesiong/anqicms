import { get, post } from '../tools';

export async function pluginGetAnchors(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/anchor/list',
    params,
    options,
  });
}

export async function pluginGetAnchorInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/anchor/detail',
    params,
    options,
  });
}

export async function pluginSaveAnchor(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/anchor/detail',
    body,
    options,
  });
}

export async function pluginReplaceAnchor(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/anchor/replace',
    body,
    options,
  });
}

export async function pluginDeleteAnchor(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/anchor/delete',
    body,
    options,
  });
}

export async function pluginExportAnchor(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/anchor/export',
    body,
    options,
  });
}

export async function pluginImportAnchor(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/anchor/import',
    body,
    options,
  });
}

export async function pluginGetAnchorSetting(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/anchor/setting',
    params,
    options,
  });
}

export async function pluginSaveAnchorSetting(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/anchor/setting',
    body,
    options,
  });
}
