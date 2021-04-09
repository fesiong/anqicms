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

  //logo上传
  var uploadInst = upload.render({
    elem: '#site-logo' //绑定元素
    ,url: setter.baseApi + 'attachment/upload' //上传接口
    ,done: function(res){
      //上传完毕回调
      $('#site-logo-input').val(res.data.src);
      $('#site-logo-img').prop('src', res.data.src);
    }
    ,error: function(){
      //请求异常回调
      layer.msg("上传出错");
    }
  });

  //网站设置
  form.on('submit(system-submit)', function(obj){
    delete obj.field.file;
    obj.field.site_close = Number(obj.field.site_close);
    obj.field.template_type = Number(obj.field.template_type);
    admin.req({
      url: '/setting/system'
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

  //首页TDK设置
  form.on('submit(index-submit)', function(obj){
    admin.req({
      url: '/setting/index'
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
  //内容设置
  form.on('submit(content-submit)', function(obj){
    obj.field.remote_download = Number(obj.field.remote_download);
    obj.field.filter_outlink = Number(obj.field.filter_outlink);
    obj.field.resize_image = Number(obj.field.resize_image);
    obj.field.resize_width = Number(obj.field.resize_width);
    obj.field.thumb_crop = Number(obj.field.thumb_crop);
    obj.field.thumb_width = Number(obj.field.thumb_width);
    obj.field.thumb_height = Number(obj.field.thumb_height);
    admin.req({
      url: '/setting/content'
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
  //重建缩略图
  form.on('submit(rebuild-thumb)', function(obj){
    obj.field.remote_download = Number(obj.field.remote_download);
    obj.field.filter_outlink = Number(obj.field.filter_outlink);
    obj.field.resize_image = Number(obj.field.resize_image);
    obj.field.resize_width = Number(obj.field.resize_width);
    obj.field.thumb_crop = Number(obj.field.thumb_crop);
    obj.field.thumb_width = Number(obj.field.thumb_width);
    obj.field.thumb_height = Number(obj.field.thumb_height);
    admin.req({
      url: '/setting/content'
      ,data: obj.field
      ,type: 'post'
      ,done: function(res){
        admin.req({
          url: '/setting/thumb/rebuild'
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
      }
      ,fail: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 2
        });
      }
    });
  });
  //默认缩略图
  var uploadInst = upload.render({
    elem: '#default-thumb' //绑定元素
    ,url: setter.baseApi + 'attachment/upload' //上传接口
    ,done: function(res){
      //上传完毕回调
      $('#default-thumb-input').val(res.data.thumb);
      $('#default-thumb-img').prop('src', res.data.thumb);
    }
    ,error: function(){
      //请求异常回调
      layer.msg("上传出错");
    }
  });
  //清除默认图片
  $('#clean-default-thumb').click(function(){
    layer.confirm("确定要清除默认图片缩略图吗？", function(index){
      layer.close(index);
      $('#default-thumb-input').val("");
      $('#default-thumb-img').prop('src', '');
    });
  });
  
  //contact
  form.on('submit(contact-submit)', function(obj){
    admin.req({
      url: '/setting/contact'
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
  //二维码上传
  var uploadInst = upload.render({
    elem: '#contact-qrcode' //绑定元素
    ,url: setter.baseApi + 'attachment/upload' //上传接口
    ,done: function(res){
      //上传完毕回调
      $('#contact-qrcode-input').val(res.data.src);
      $('#contact-qrcode-img').prop('src', res.data.src);
    }
    ,error: function(){
      //请求异常回调
      layer.msg("上传出错");
    }
  });

  //对外暴露的接口
  exports('setting', {});
});