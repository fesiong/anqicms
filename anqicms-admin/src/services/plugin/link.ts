import { get, post } from '../tools';

export async function pluginGetLinks(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/link/list',
    params,
    options,
  });
}

export async function pluginSaveLink(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/link/detail',
    body,
    options,
  });
}

export async function pluginDeleteLink(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/link/delete',
    body,
    options,
  });
}

export async function pluginCheckLink(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/link/check',
    body,
    options,
  });
}
