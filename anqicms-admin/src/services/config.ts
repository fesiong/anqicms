// 配置文件
declare global {
  interface Window {
    APP_CONFIG?: {
      base: string;
      publicPath: string;
      apiBaseUrl: string;
    };
  }
}

let host = '/system/api';
if (typeof window !== 'undefined' && window.APP_CONFIG) {
  host = window.APP_CONFIG.apiBaseUrl;
}
if (process.env.NODE_ENV === 'development') {
  host = 'http://127.0.0.1:8001/system/api';
}

const config = {
  baseUrl: host,
};

export default config;
