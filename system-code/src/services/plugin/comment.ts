import { get, post } from '../tools';

export async function pluginGetComments(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/comment/list',
    params,
    options,
  });
}

export async function pluginGetCommentInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/comment/detail',
    params,
    options,
  });
}

export async function pluginSaveComment(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/comment/detail',
    body,
    options,
  });
}

export async function pluginDeleteComment(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/comment/delete',
    body,
    options,
  });
}

export async function pluginCheckComment(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/comment/check',
    body,
    options,
  });
}
