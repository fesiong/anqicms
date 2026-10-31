import { get, post } from './tools';

export async function login(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/login',
    body,
    options,
  });
}

export async function getAdminList(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/admin/list',
    params,
    options,
  });
}

export async function getAdminInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/admin/detail',
    params,
    options,
  });
}

export async function deleteAdminInfo(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/admin/delete',
    body,
    options,
  });
}

export async function getAdminLoginLogs(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/admin/logs/login',
    params,
    options,
  });
}

export async function getAdminActionLogs(
  params?: any,
  options?: { [key: string]: any },
) {
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

export async function getCaptcha(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/captcha',
    // 验证码是一次性凭据：同一个 id 只能校验一次，且默认几分钟即过期。
    // 不带时间戳时 URL 恒定，浏览器 / 中间代理会把上一次的响应（同一张图、
    // 同一个 captcha_id）回放出来，表现为"点了图片不换"或"验证码永远校验失败"。
    params: { ...params, _t: Date.now() },
    options,
  });
}

export async function getPermissionMenus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/admin/menus',
    params,
    options,
  });
}

export async function getAdminGroups(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/admin/group/list',
    params,
    options,
  });
}

export async function getAdminGroupInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/admin/group/detail',
    params,
    options,
  });
}

export async function saveAdminGroupInfo(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/admin/group/detail',
    body,
    options,
  });
}

export async function deleteAdminGroupInfo(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/admin/group/delete',
    body,
    options,
  });
}

export async function findPasswordChoose(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/password/choose',
    body,
    options,
  });
}

export async function findPasswordVerify(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/password/verify',
    params,
    options,
  });
}

export async function findPasswordReset(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/password/reset',
    body,
    options,
  });
}
