import { get, post } from '../tools';

export async function pluginGetWithdraws(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/withdraw/list',
    params,
    options,
  });
}

export async function pluginGetWithdrawInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/withdraw/detail',
    params,
    options,
  });
}

export async function pluginSetWithdrawApproval(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/withdraw/approval',
    body,
    options,
  });
}

export async function pluginSetWithdrawFinished(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/withdraw/finished',
    body,
    options,
  });
}

export async function pluginSetWithdrawApply(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/withdraw/apply',
    body,
    options,
  });
}
