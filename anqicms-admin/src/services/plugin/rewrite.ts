import { get, post } from '../tools';

export async function pluginGetRewrite(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/rewrite',
    params,
    options,
  });
}

export async function pluginSaveRewrite(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/rewrite',
    body,
    options,
  });
}
