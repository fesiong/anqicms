import { get, post } from './tools';

export async function getArchives(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/archive/list',
    params,
    options,
  });
}

export async function getArchiveInfo(
  params: any,
  options?: { [key: string]: any },
) {
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

export async function deleteArchiveImage(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/delete/image',
    body,
    options,
  });
}

export async function deleteArchive(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/delete',
    body,
    options,
  });
}

export async function recoverArchive(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/recover',
    body,
    options,
  });
}

export async function releaseArchive(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/release',
    body,
    options,
  });
}

export async function updateArchivesFlag(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/recommend',
    body,
    options,
  });
}

export async function updateArchivesStatus(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/status',
    body,
    options,
  });
}

export async function updateArchivesTime(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/time',
    body,
    options,
  });
}

export async function updateArchivesCategory(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/category',
    body,
    options,
  });
}

export async function updateArchivesTags(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/tags',
    body,
    options,
  });
}

export async function updateArchivesSort(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/sort',
    body,
    options,
  });
}

export async function updateArchiveParent(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/parent',
    body,
    options,
  });
}

export async function updateArchivesReleasePlan(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/plan',
    body,
    options,
  });
}

export async function getQuickImportArchiveStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/archive/import/status',
    params,
    options,
  });
}

export async function archiveQuickImport(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/archive/import',
    body,
    options,
  });
}
