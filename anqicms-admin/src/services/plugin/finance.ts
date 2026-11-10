import { get } from '../tools';

export async function pluginGetFinances(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/finance/list',
    params,
    options,
  });
}
