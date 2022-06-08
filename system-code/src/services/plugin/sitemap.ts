import { get, post } from '../tools';

export async function pluginGetSitemap(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/sitemap',
    params,
    options,
  });
}

export async function pluginSaveSitemap(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/sitemap',
    body,
    options,
  });
}

export async function pluginBuildSitemap(body?: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/sitemap/build',
    body,
    options,
  });
}
