import { get, post } from './tools';

export async function login(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/login',
    body,
    options,
  });
}

export async function getAdminInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/admin/detail',
    params,
    options,
  });
}

export async function saveAdmin(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/admin/detail',
    body,
    options,
  });
}

export async function getCaptcha(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/captcha',
    params,
    options,
  });
}
