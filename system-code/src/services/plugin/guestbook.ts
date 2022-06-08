import { get, post } from '../tools';

export async function pluginGetGuestbooks(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/guestbook/list',
    params,
    options,
  });
}

export async function pluginDeleteGuestbook(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/guestbook/delete',
    body,
    options,
  });
}

export async function pluginExportGuestbook(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/guestbook/export',
    body,
    options,
  });
}

export async function pluginGetGuestbookSetting(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/guestbook/setting',
    params,
    options,
  });
}

export async function pluginSaveGuestbookSetting(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/guestbook/setting',
    body,
    options,
  });
}
