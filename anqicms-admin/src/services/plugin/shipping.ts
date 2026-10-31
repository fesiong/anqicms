import { get, post } from '../tools';

// ==================== 配送区域 ====================

// 获取配送区域列表
export async function pluginGetShippingZones(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/shipping/zone/list',
    params,
    options,
  });
}

// 获取配送区域详情
export async function pluginGetShippingZone(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/shipping/zone',
    params,
    options,
  });
}

// 保存配送区域
export async function pluginSaveShippingZone(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/shipping/zone/save',
    body,
    options,
  });
}

// 删除配送区域
export async function pluginDeleteShippingZone(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/shipping/zone/delete',
    body,
    options,
  });
}

// ==================== 区域覆盖地址 ====================

// 获取区域覆盖地址列表
export async function pluginGetShippingZoneRegions(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/shipping/zone/region/list',
    params,
    options,
  });
}

// 保存区域覆盖地址
export async function pluginSaveShippingZoneRegion(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/shipping/zone/region/save',
    body,
    options,
  });
}

// 删除区域覆盖地址
export async function pluginDeleteShippingZoneRegion(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/shipping/zone/region/delete',
    body,
    options,
  });
}

// ==================== 运费规则 ====================

// 获取运费规则列表
export async function pluginGetShippingRules(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/shipping/rule/list',
    params,
    options,
  });
}

// 获取运费规则详情
export async function pluginGetShippingRule(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/shipping/rule',
    params,
    options,
  });
}

// 保存运费规则
export async function pluginSaveShippingRule(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/shipping/rule/save',
    body,
    options,
  });
}

// 删除运费规则
export async function pluginDeleteShippingRule(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/shipping/rule/delete',
    body,
    options,
  });
}

// ==================== 设置 ====================

// 获取设置
export async function pluginGetShippingSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/shipping/setting',
    params,
    options,
  });
}

// 保存设置
export async function pluginSaveShippingSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/shipping/setting',
    body,
    options,
  });
}
