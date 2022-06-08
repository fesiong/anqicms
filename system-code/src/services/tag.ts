import { get, post } from './tools';

export async function getTags(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/tag/list',
    params,
    options,
  });
}

export async function getTagInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/tag/detail',
    params,
    options,
  });
}

export async function saveTag(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/tag/detail',
    body,
    options,
  });
}

export async function deleteTag(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/tag/delete',
    body,
    options,
  });
}
