import { get } from './tools';

export async function getStatistics(params?: any, options?: { [key: string]: any }) {
  return get({
    url: '/statistics',
    params,
    options,
  });
}

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
