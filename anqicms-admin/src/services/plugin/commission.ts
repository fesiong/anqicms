import { get } from '../tools';

export async function pluginGetCommissions(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/commission/list',
    params,
    options,
  });
}

export async function pluginGetCommissionInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/commission/detail',
    params,
    options,
  });
}
