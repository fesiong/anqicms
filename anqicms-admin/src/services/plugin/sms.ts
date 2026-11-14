import { get, post } from '../tools';

// 短信渠道配置：读端点对非空凭据返回 ******** 掩码，保存端点把它识别为「未改动」哨兵。
export async function pluginSmsSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/sms/setting',
    params,
    options,
  });
}

export async function pluginSmsSettingSave(
  params?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/sms/setting',
    body: params,
    options,
  });
}

// 后台测试发送：body 为 { phone_country, phone }，走真实发送通道且受限额约束。
export async function pluginSmsTest(
  params?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/sms/test',
    body: params,
    options,
  });
}

// 发送记录：支持 current / pageSize 分页及
// scene / provider / status / phone / start_time / end_time 筛选。
export async function pluginSmsLogs(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/sms/logs',
    params,
    options,
  });
}
