import { get, post } from '../tools';

export async function pluginGetTitleImageConfig(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/titleimage/config',
    params,
    options,
  });
}

export async function pluginSaveTitleImageConfig(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/titleimage/config',
    body,
    options,
  });
}

export async function pluginTitleImagePreview(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/titleimage/preview',
    params,
    options,
  });
}

export async function pluginTitleImageUploadFile(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/titleimage/upload',
    body,
    options,
  });
}

export async function pluginTitleImageGenerate(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/titleimage/generate',
    body,
    options,
  });
}
