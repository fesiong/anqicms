import { get, post } from '../tools';

export async function pluginGetSubscribers(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/subscriber/list',
    params,
    options,
  });
}

export async function pluginSaveSubscriber(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/subscriber/save',
    body,
    options,
  });
}
export async function pluginDeleteSubscriber(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/subscriber/delete',
    body,
    options,
  });
}

export async function pluginSubscriberSendMail(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/subscriber/send',
    body,
    options,
  });
}

export async function pluginGetSubscriberCategories(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/subscriber/category/list',
    params,
    options,
  });
}

export async function pluginSaveSubscriberCategory(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/subscriber/category/save',
    body,
    options,
  });
}
export async function pluginDeleteSubscriberCategory(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/subscriber/category/delete',
    body,
    options,
  });
}
