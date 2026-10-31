import { get, post } from '../tools';

export async function pluginGetUploadFiles(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/fileupload/list',
    params,
    options,
  });
}

export async function pluginUploadFile(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/fileupload/upload',
    body,
    options,
  });
}

export async function pluginDeleteFile(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/fileupload/delete',
    body,
    options,
  });
}
