import { get, post } from './tools';

export async function getCollectorSetting(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/collector/setting',
    params,
    options,
  });
}

export async function saveCollectorSetting(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/collector/setting',
    body,
    options,
  });
}

export async function replaceCollectorArticle(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/collector/article/replace',
    body,
    options,
  });
}

export async function pseudoCollectorArticle(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/collector/article/pseudo',
    body,
    options,
  });
}

export async function digCollectorKeyword(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/collector/keyword/dig',
    body,
    options,
  });
}

export async function collectCollectorArticle(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/collector/article/collect',
    body,
    options,
  });
}
