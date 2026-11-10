import { get, post } from '../tools';

export async function pluginGetActivities(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/activity/list',
    params,
    options,
  });
}

export async function pluginGetActivityDetail(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/activity/detail',
    params,
    options,
  });
}

export async function pluginSaveActivity(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/activity/detail',
    body,
    options,
  });
}

export async function pluginDeleteActivity(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/activity/delete',
    body,
    options,
  });
}
