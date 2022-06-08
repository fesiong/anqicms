import { get, post } from './tools';

export async function getAttachments(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/attachment/list',
    params,
    options,
  });
}

export async function uploadAttachment(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/attachment/upload',
    body,
    options,
  });
}

export async function deleteAttachment(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/attachment/delete',
    body,
    options,
  });
}

export async function getAttachmentCategories(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/attachment/category/list',
    params,
    options,
  });
}

export async function changeAttachmentCategory(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/attachment/category',
    body,
    options,
  });
}

export async function saveAttachmentCategory(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/attachment/category/detail',
    body,
    options,
  });
}

export async function deleteAttachmentCategory(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/attachment/category/delete',
    body,
    options,
  });
}
