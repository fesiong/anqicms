import { get, post } from '../tools';

export async function pluginGetRedirects(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/redirect/list',
    params,
    options,
  });
}

export async function pluginSaveRedirect(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/redirect/detail',
    body,
    options,
  });
}

export async function pluginDeleteRedirect(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/redirect/delete',
    body,
    options,
  });
}

export async function pluginImportRedirect(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/redirect/import',
    body,
    options,
  });
}
