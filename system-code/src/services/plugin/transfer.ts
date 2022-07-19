import { get, post } from '../tools';

export async function pluginGetTransferTask(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/transfer/task',
    params,
    options,
  });
}

export async function pluginDownloadProvider(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/transfer/download',
    body,
    options,
  });
}

export async function pluginCreateTransferTask(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/transfer/create',
    body,
    options,
  });
}

export async function pluginStartTransferTask(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/transfer/start',
    body,
    options,
  });
}
