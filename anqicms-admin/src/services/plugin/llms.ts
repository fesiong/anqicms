import { get, post } from '../tools';

export async function pluginGetLLMs(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/llms/setting',
    params,
    options,
  });
}

export async function pluginGetLLMsBuildStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/llms/status',
    params,
    options,
  });
}

export async function pluginSaveLLMs(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/llms/setting',
    body,
    options,
  });
}

export async function pluginBuildLLMs(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/llms/build',
    body,
    options,
  });
}
