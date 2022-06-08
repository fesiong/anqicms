import { extend } from 'umi-request';
import config from './config';
import { getStore } from '../utils/store';
import { message } from 'antd';
import { history } from 'umi';
const request = extend({
  prefix: config.baseUrl,
  timeout: 60000,
  requestType: 'json',
});

request.use(async (ctx, next) => {
  const { req } = ctx;
  const { url, options } = req;

  let headers: any = {};

  let adminToken = getStore('adminToken');
  if (adminToken) {
    headers['admin'] = adminToken;
  }
  ctx.req.options = {
    ...options,
    headers: headers,
  };

  await next();

  const { res } = ctx;

  if (res.code === 1001) {
    //需要登录
    message.warning({
      content: res.msg,
      key: 'error',
      style: { marginTop: '50px' },
    });
    history.push('/login');
  }
});

/**
 * 公用get请求
 * @param url       接口地址
 * @param msg       接口异常提示
 * @param headers   接口所需header配置
 */
export const get = ({ url = '', params = {}, options = {} }) => {
  return request
    .get(url, { params: params, ...options })
    .then((res: any) => {
      return res;
    })
    .catch((err: any) => {
      return Promise.reject(err);
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
  return request
    .post(url, { data: body, ...options })
    .then((res: any) => {
      return res;
    })
    .catch((err: any) => {
      return Promise.reject(err);
    });
};

export const put = ({ url = '', body = {}, options = {} }) => {
  return request
    .put(url, { data: body, ...options })
    .then((res: any) => {
      return res;
    })
    .catch((err: any) => {
      return Promise.reject(err);
    });
};
