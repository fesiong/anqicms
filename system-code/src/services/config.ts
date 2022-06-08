// 配置文件

import { history } from 'umi';

const basePath = history.location.pathname.split('/')[1] || '';


//const host = '/' + basePath;
var host = 'http://127.0.0.1:8001/system/api';
host = "/system/api";

const config = {
  baseUrl: host,
};

export default config;
