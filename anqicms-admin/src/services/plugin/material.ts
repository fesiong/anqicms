import { get, post } from '../tools';

export async function pluginGetMaterials(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/material/list',
    params,
    options,
  });
}

export async function pluginSaveMaterial(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/material/detail',
    body,
    options,
  });
}

export async function pluginDeleteMaterial(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/material/delete',
    body,
    options,
  });
}

export async function pluginGetMaterialCategories(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/plugin/material/category/list',
    params,
    options,
  });
}

export async function pluginSaveMaterialCategory(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/material/category/detail',
    body,
    options,
  });
}

export async function pluginDeleteMaterialCategory(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/material/category/delete',
    body,
    options,
  });
}

export async function pluginMaterialImport(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/material/import',
    body,
    options,
  });
}

export async function pluginMaterialConvertFile(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/material/convert/file',
    body,
    options,
  });
}
