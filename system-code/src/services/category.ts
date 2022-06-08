import { get, post } from './tools';

export async function getCategories(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/category/list',
    params,
    options,
  });
}

export async function getCategoryInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/category/detail',
    params,
    options,
  });
}

export async function saveCategory(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/category/detail',
    body,
    options,
  });
}

export async function deleteCategory(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/category/delete',
    body,
    options,
  });
}
