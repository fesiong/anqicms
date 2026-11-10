import { get, post } from '../tools';

export async function pluginGetCoupons(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/coupon/list',
    params,
    options,
  });
}

export async function pluginGetCouponDetail(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/coupon/detail',
    params,
    options,
  });
}

export async function pluginSaveCoupon(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/coupon/detail',
    body,
    options,
  });
}

export async function pluginDeleteCoupon(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/coupon/delete',
    body,
    options,
  });
}

export async function pluginGetCouponCodes(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/coupon/code/list',
    params,
    options,
  });
}

export async function pluginMakeCouponCode(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/coupon/code/make',
    body,
    options,
  });
}

export async function pluginDeleteCouponCode(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/coupon/code/delete',
    body,
    options,
  });
}

export async function pluginSaveCouponCode(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/coupon/code/detail',
    body,
    options,
  });
}

export async function pluginGetCouponArchives(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/coupon/archives',
    params,
    options,
  });
}

export async function pluginAddCouponArchives(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/coupon/archives/add',
    body,
    options,
  });
}

export async function pluginDeleteCouponArchives(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/coupon/archives/delete',
    body,
    options,
  });
}
