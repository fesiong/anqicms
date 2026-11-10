import { get, post } from '../tools';

export async function pluginGetUsers(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/user/list',
    params,
    options,
  });
}

export async function pluginGetUserInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/user/detail',
    params,
    options,
  });
}

export async function pluginSaveUserInfo(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/user/detail',
    body,
    options,
  });
}

export async function pluginAddUserBalance(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/user/balance',
    body,
    options,
  });
}

export async function pluginDeleteUserInfo(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/user/delete',
    body,
    options,
  });
}

export async function pluginGetUserGroups(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/user/group/list',
    params,
    options,
  });
}

export async function pluginGetUserGroupInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/user/group/detail',
    params,
    options,
  });
}

export async function pluginSaveUserGroupInfo(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/user/group/detail',
    body,
    options,
  });
}

export async function pluginDeleteUserGroup(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/user/group/delete',
    body,
    options,
  });
}

export async function pluginGetUserFieldsSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/user/fields',
    params,
    options,
  });
}

export async function pluginSaveUserFieldsSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/user/fields',
    body,
    options,
  });
}

export async function pluginDeleteUserField(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/user/field/delete',
    body,
    options,
  });
}
