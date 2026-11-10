import { get, post } from '../tools';

export async function pluginGetSendmails(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/sendmail/list',
    params,
    options,
  });
}

export async function pluginTestSendmail(
  body?: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/sendmail/test',
    body,
    options,
  });
}

export async function pluginGetSendmailSetting(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/sendmail/setting',
    params,
    options,
  });
}

export async function pluginSaveSendmailSetting(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/sendmail/setting',
    body,
    options,
  });
}

export async function pluginGetSendmailTemplates(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/sendmail/templates',
    params,
    options,
  });
}

export async function pluginGetSendmailTemplateDetail(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/sendmail/template',
    params,
    options,
  });
}

export async function pluginSaveSendmailTemplate(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/sendmail/template',
    body,
    options,
  });
}

export async function pluginSendmailTemplatePreview(
  body: any,
  options?: { [key: string]: any },
) {
  return post({
    url: '/plugin/sendmail/template/preview',
    body,
    options,
  });
}
