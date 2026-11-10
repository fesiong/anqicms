import { get, post } from './tools';

export async function getModules(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/module/list',
    params,
    options,
  });
}

export async function getModuleInfo(params: any, options?: { [key: string]: any }) {
  return get({
    url: '/module/detail',
    params,
    options,
  });
}

export async function saveModule(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/module/detail',
    body,
    options,
  });
}

export async function deleteModule(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/module/delete',
    body,
    options,
  });
}

export async function deleteModuleField(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/module/field/delete',
    body,
    options,
  });
}
