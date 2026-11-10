import { get, post } from '../tools';

export async function pluginGetBackupList(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/backup/list',
    params,
    options,
  });
}

export async function pluginBackupData(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/backup/dump',
    body,
    options,
  });
}

export async function pluginGetBackupStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/backup/status',
    params,
    options,
  });
}

export async function pluginBackupRestore(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/backup/restore',
    body,
    options,
  });
}
export async function pluginBackupDelete(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/backup/delete',
    body,
    options,
  });
}

export async function pluginBackupImport(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/backup/import',
    body,
    options,
  });
}

export async function pluginBackupCleanup(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/backup/cleanup',
    body,
    options,
  });
}

export async function pluginBackupRemark(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/backup/remark',
    body,
    options,
  });
}
