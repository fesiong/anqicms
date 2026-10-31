import { get, post } from '../tools';

export async function pluginGetOrderReviews(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/order/reviews',
    params,
    options,
  });
}

export async function pluginGetOrderReviewInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/order/review',
    params,
    options,
  });
}

export async function pluginSaveOrderReview(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/review/save',
    body,
    options,
  });
}

export async function pluginDeleteOrderReview(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/order/review/delete',
    body,
    options,
  });
}
