import { post } from '../tools';

export async function pluginReplaceValues(body: any, options?: { [key: string]: any }) {
  return post({
    url: '/plugin/replace/values',
    body,
    options,
  });
}
