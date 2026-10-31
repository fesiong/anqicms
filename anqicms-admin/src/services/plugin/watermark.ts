import { get, post } from '../tools';

export async function pluginGetWatermarkConfig(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/watermark/config',
    params,
    options,
  });
}

export async function pluginSaveWatermarkConfig(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/watermark/config',
    body,
    options,
  });
}

export async function pluginWatermarkPreview(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/watermark/preview',
    params,
    options,
  });
}

export async function pluginWatermarkUploadFile(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/watermark/upload',
    body,
    options,
  });
}

export async function pluginWatermarkGenerate(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/watermark/generate',
    body,
    options,
  });
}
