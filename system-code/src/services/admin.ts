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

export async function getAdminLoginLogs(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/admin/logs/login',
    params,
    options,
  });
}

export async function getAdminActionLogs(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/admin/logs/action',
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
