import { get, post } from '../tools';

export async function pluginGetFulltextConfig(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/fulltext/config',
    params,
    options,
  });
}

export async function pluginSaveFulltextConfig(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/fulltext/config',
    body,
    options,
  });
}

export async function pluginRebuildFulltextIndex(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/fulltext/rebuild',
    body,
    options,
  });
}

export async function pluginGetFulltextStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/fulltext/status',
    params,
    options,
  });
}
