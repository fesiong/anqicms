import { get, post } from '../tools';

export async function pluginGetRobots(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/robots',
    params,
    options,
  });
}

export async function pluginSaveRobots(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/robots',
    body,
    options,
  });
}
