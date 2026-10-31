import { get, post } from '../tools';

export async function getAiGenerateSetting(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/aigenerate/setting',
    params,
    options,
  });
}

export async function saveAiGenerateSetting(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/aigenerate/setting',
    body,
    options,
  });
}

export async function collectAiGenerateArticle(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/aigenerate/article/collect',
    body,
    options,
  });
}

export async function startAiGenerateArticle(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/aigenerate/article/start',
    body,
    options,
  });
}

export async function checkOpenAIApi(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/aigenerate/checkapi',
    params,
    options,
  });
}

export async function getAiArticlePlans(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/aigenerate/plans',
    params,
    options,
  });
}
