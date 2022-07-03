import { get, post } from './tools';

export async function getArchives(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/archive/list',
    params,
    options,
  });
}

export async function getArchiveInfo(params: any, options?: { [key: string]: any }) {
  return get({
    url: '/archive/detail',
    params,
    options,
  });
}

export async function saveArchive(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/archive/detail',
    body,
    options,
  });
}

export async function deleteArchive(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/archive/delete',
    body,
    options,
  });
}

export async function recoverArchive(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/archive/recover',
    body,
    options,
  });
}

export async function releaseArchive(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/archive/release',
    body,
    options,
  });
}
