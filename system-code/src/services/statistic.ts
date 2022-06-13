import { get } from './tools';

export async function getStatisticSpider(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/statistic/spider',
    params,
    options,
  });
}

export async function getStatisticTraffic(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/statistic/traffic',
    params,
    options,
  });
}

export async function getStatisticInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/statistic/detail',
    params,
    options,
  });
}

export async function getStatisticInclude(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/statistic/include',
    params,
    options,
  });
}

export async function getStatisticIncludeInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/statistic/include/detail',
    params,
    options,
  });
}

export async function getStatisticSummary(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/statistic/summary',
    params,
    options,
  });
}

export async function getDashboardInfo(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/statistic/dashboard',
    params,
    options,
  });
}
