import { get, post } from '../tools';

export async function pluginGetOrders(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/order/list',
    params,
    options,
  });
}

export async function pluginGetOrderInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/order/detail',
    params,
    options,
  });
}

export async function pluginSetOrderDelivery(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/deliver',
    body,
    options,
  });
}

export async function pluginSetOrderFinished(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/finished',
    body,
    options,
  });
}

export async function pluginSetOrderCanceled(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/canceled',
    body,
    options,
  });
}

export async function pluginSetOrderPay(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/pay',
    body,
    options,
  });
}

export async function pluginSetOrderRefund(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/refund',
    body,
    options,
  });
}

export async function pluginOrderApplyRefund(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/refund/apply',
    body,
    options,
  });
}

export async function pluginExportOrder(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/export',
    body,
    options,
  });
}

export async function pluginGetOrderSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/order/config',
    params,
    options,
  });
}

export async function pluginSaveOrderSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/config',
    body,
    options,
  });
}

export async function pluginGetOrderServices(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/order/services',
    params,
    options,
  });
}

export async function pluginGetOrderServiceInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/order/service',
    params,
    options,
  });
}

export async function pluginSaveOrderService(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/service/save',
    body,
    options,
  });
}

export async function pluginDeleteOrderService(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/service/delete',
    body,
    options,
  });
}
