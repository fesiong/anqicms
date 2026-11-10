import { get, post } from '../tools';

export async function pluginGetGoogleMerchantSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/googlemerchant/config',
    params,
    options,
  });
}

export async function pluginSaveGoogleMerchantSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/googlemerchant/config',
    body,
    options,
  });
}

export async function pluginGetGoogleMerchantAuthUrl(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/googlemerchant/auth/url',
    params,
    options,
  });
}

export async function pluginGoogleMerchantAuthCallback(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/googlemerchant/auth/callback',
    body,
    options,
  });
}

export async function pluginGoogleMerchantAccountSelect(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/googlemerchant/account/select',
    body,
    options,
  });
}

export async function pluginGoogleMerchantSync(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/googlemerchant/sync',
    body,
    options,
  });
}

export async function pluginGoogleMerchantSyncDelete(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/googlemerchant/sync/delete',
    body,
    options,
  });
}

export async function pluginGoogleMerchantSyncStatus(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/googlemerchant/sync/statuses',
    params,
    options,
  });
}

export async function pluginGoogleMerchantSynclogs(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/googlemerchant/sync/logs',
    params,
    options,
  });
}
