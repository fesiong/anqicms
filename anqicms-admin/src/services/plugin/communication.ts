import { get, post } from '../tools';

export async function pluginGetCommunicationOrders(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/communication/order/list',
    params,
    options,
  });
}

export async function pluginGetCommunicationOrderInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/communication/order/detail',
    params,
    options,
  });
}

export async function pluginCloseCommunicationOrder(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/communication/order/close',
    body,
    options,
  });
}

export async function pluginSaveCommunicationOrderImages(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/communication/order/images/save',
    body,
    options,
  });
}

export async function pluginGetCommunicationMessages(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/communication/message/list',
    params,
    options,
  });
}

export async function pluginSendCommunicationMessage(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/communication/message/send',
    body,
    options,
  });
}

export async function pluginSendCommunicationQuote(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/communication/quote/send',
    body,
    options,
  });
}

export async function pluginDeleteCommunicationMessage(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/communication/message/delete',
    body,
    options,
  });
}

export async function pluginSaveCommunicationSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/communication/setting',
    body,
    options,
  });
}

export async function pluginGetCommunicationSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/communication/setting',
    params,
    options,
  });
}
