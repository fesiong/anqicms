/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */
 
layui.define(['form', 'upload', 'table'], function(exports){
  var $ = layui.$
  ,layer = layui.layer
  ,laytpl = layui.laytpl
  ,setter = layui.setter
  ,view = layui.view
  ,admin = layui.admin
  ,form = layui.form
  ,table = layui.table
  ,upload = layui.upload;

  //搜索引擎推送
  form.on('submit(push-submit)', function(obj){
    admin.req({
      url: '/plugin/push'
      ,data: obj.field
      ,type: 'post'
      ,done: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 1
        });
      }
      ,fail: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 2
        });
      }
    });
  });
  //Robots管理
  form.on('submit(robots-submit)', function(obj){
    admin.req({
      url: '/plugin/robots'
      ,data: obj.field
      ,type: 'post'
      ,done: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 1
        });
      }
      ,fail: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 2
        });
      }
    });
  });
  //sitemap管理
  form.on('submit(sitemap-submit)', function(obj){
    obj.field.auto_build = Number(obj.field.auto_build)
    admin.req({
      url: '/plugin/sitemap'
      ,data: obj.field
      ,type: 'post'
      ,done: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 1
        });
      }
      ,fail: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 2
        });
      }
    });
  });
  //手动生成sitemap
  form.on('submit(sitemap-build-submit)', function(obj){
    obj.field.auto_build = Number(obj.field.auto_build)
    admin.req({
      url: '/plugin/sitemap/build'
      ,data: obj.field
      ,type: 'post'
      ,done: function(res){
        $('#updated-time-field').val(layui.util.toDateString(res.data.updated_time*1000, "yyyy-MM-dd HH:mm"));
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 1
        });
      }
      ,fail: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 2
        });
      }
    });
  });
  //rewrite管理
  form.on('submit(rewrite-submit)', function(obj){
    obj.field.mode = Number(obj.field.mode)
    admin.req({
      url: '/plugin/rewrite'
      ,data: obj.field
      ,type: 'post'
      ,done: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 1
        });
      }
      ,fail: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 2
        });
      }
    });
  });
  //rewrite-radio监听
  form.on('radio(rewrite-radio)', function(d){
    if(d.value == 3) {
      //自定义模式
      $('#rewrite-patten-field').removeClass('layui-hide');
    } else {
      $('#rewrite-patten-field').addClass('layui-hide');
    }
  });

  //对外暴露的接口
  exports('plugin', {});
});