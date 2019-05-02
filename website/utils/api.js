import axios from 'axios';
import serverCfg from '../config'

//默认走前端api
axios.defaults.baseURL = serverCfg.apiURL;

if (typeof window === 'undefined') {
  axios.defaults.baseURL = serverCfg.backApiURL
}

axios.defaults.timeout = 10000;
axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';
axios.defaults.headers.common['api'] = '1.0';

axios.interceptors.request.use(config => {
  //header
  if (config.method === 'post') {
    config.headers['Content-Type'] = 'application/x-www-form-urlencoded';
  }
  if (config.hasFile) {
    config.headers['Content-Type'] = 'multipart/form-data';
  }
  let token = '';
  if(typeof localStorage !== 'undefined'){
    token = localStorage.getItem('token')
  }

  if (token) {
    config.headers['token'] = token;
  }

  return config;
});

axios.interceptors.response.use(response => {
  if (response.status === 200) {
    if (response.data.code === 1001) {
      if(typeof localStorage !== 'undefined'){
        localStorage.removeItem('token')
      }
      alert('提示', '该操作需要登录');
      return;
    }

    return response.data;
  }else if (response.code || response.msg || response.data) {
    return response;
  } else {
    throw Error(response.msg || response.data.msg || '服务异常')
  }
});

const api = {
  articleList: (params => {
    return axios.get("article/list", params);
  }),
  articleDetail: ((id, params) => {
    return axios.get("article/detail/" + id, params);
  }),
  articleSave: (params => {
    return axios.post("article/save", params);
  }),
  articleDelete: ((id, params) => {
    return axios.delete("article/delete/" + id, params);
  }),
  categoryList: (params => {
    return axios.get("category/list", {params: params});
  }),
  categoryDetail: ((id, params) => {
    return axios.get("category/detail/" + id, params);
  }),
  categorySave: (params => {
    return axios.post("category/save", params);
  }),
  categoryDelete: ((id, params) => {
    return axios.delete("category/delete/" + id, params);
  }),
  commentList: ((articleID, params) => {
    return axios.get("comment/list/" + articleID, params);
  }),
  commentSave: (params => {
    return axios.post("comment/save", params);
  }),
  commentDelete: ((id, params) => {
    return axios.delete("comment/delete/" + id, params);
  }),
  attachmentUpload: (params => {
    return axios.post("attachment/upload", params, { hasFile: !0 });
  }),
  attachmentDelete: ((id, params) => {
    return axios.delete("attachment/delete/" + id, params);
  }),
  signIn: (params => {
    return axios.post("sign/in", params);
  }),
  signUp: (params => {
    return axios.post("sign/up", params);
  }),
  signOut: (params => {
    return axios.post("sign/out", params);
  }),
}

export default api;