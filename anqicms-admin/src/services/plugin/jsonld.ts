import { get, post } from '../tools';

export async function pluginGetJsonLdConfig(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/jsonld/config',
    params,
    options,
  });
}

export async function pluginSaveJsonLdConfig(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/jsonld/config',
    body,
    options,
  });
}
