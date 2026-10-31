import { request } from '@umijs/max';

/**
 * 公用get请求
 * @param url       接口地址
 * @param msg       接口异常提示
 * @param headers   接口所需header配置
 */
export const get = ({ url = '', params = {}, options = {} }) => {
  return request(url, {
    method: 'get',
    params: params,
    ...options,
  });
};

/**
 * 公用post请求
 * @param url       接口地址
 * @param data      接口参数
 * @param msg       接口异常提示
 * @param headers   接口所需header配置
 */
export const post = ({ url = '', body = {}, options = {} }) => {
  return request(url, {
    method: 'post',
    data: body,
    ...options,
  })
};

export const put = ({ url = '', body = {}, options = {} }) => {
  return request(url, {
    method: 'put',
    data: body,
    ...options,
  })
};
