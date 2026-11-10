import { get } from './tools';

export async function getStatisticSpider(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/spider',
    params,
    options,
  });
}

export async function getStatisticTraffic(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/traffic',
    params,
    options,
  });
}

export async function getStatisticDates(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/dates',
    params,
    options,
  });
}

export async function getStatisticInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/detail',
    params,
    options,
  });
}

export async function getStatisticInclude(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/include',
    params,
    options,
  });
}

export async function getStatisticIncludeInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/include/detail',
    params,
    options,
  });
}

export async function getStatisticSummary(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/summary',
    params,
    options,
  });
}

export async function getDashboardInfo(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/dashboard',
    params,
    options,
  });
}

export async function getStatisticConversion(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/conversion',
    params,
    options,
  });
}

export async function getStatisticCoupon(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/coupon',
    params,
    options,
  });
}

export async function getStatisticCustomer(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/customer',
    params,
    options,
  });
}

export async function getStatisticLiveView(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/live',
    params,
    options,
  });
}

export async function getStatisticOrder(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/order',
    params,
    options,
  });
}

export async function getStatisticProduct(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/product',
    params,
    options,
  });
}

export async function getStatisticSearch(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/search',
    params,
    options,
  });
}

export async function getStatisticSearchDetail(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/statistic/search/detail',
    params,
    options,
  });
}
