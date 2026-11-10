import { get, post } from '../tools';

export async function pluginGetRetailers(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/retailer/list',
    params,
    options,
  });
}

export async function pluginGetRetailerConfig(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/retailer/config',
    params,
    options,
  });
}

export async function pluginSaveRetailerConfig(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/retailer/config',
    body,
    options,
  });
}

export async function pluginSaveRetailerRealname(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/retailer/realname',
    body,
    options,
  });
}

export async function pluginSaveRetailer(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/retailer/apply',
    body,
    options,
  });
}
