/**

 @Name：layuiAdmin 用户登入和注册等
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */
 
layui.define('form', function(exports){
  var $ = layui.$
  ,layer = layui.layer
  ,laytpl = layui.laytpl
  ,setter = layui.setter
  ,view = layui.view
  ,admin = layui.admin
  ,form = layui.form
  ,router = layui.router()
  ,search = router.search;

  //自定义验证
  form.verify({
    username: function(value, item){ //value：表单的值、item：表单的DOM对象
      if(!new RegExp("^[a-zA-Z0-9_\u4e00-\u9fa5\\s·]+$").test(value)){
        return '用户名不能有特殊字符';
      }
      if(/(^\_)|(\__)|(\_+$)/.test(value)){
        return '用户名首尾不能出现下划线\'_\'';
      }
      if(/^\d+\d+\d$/.test(value)){
        return '用户名不能全为数字';
      }
    }
    
    //我们既支持上述函数式的方式，也支持下述数组的形式
    //数组的两个值分别代表：[正则匹配、匹配不符时的提示文字]
    ,pass: [
      /^[\S]{6,20}$/
      ,'密码必须6到20位，且不能出现空格'
    ] 
  });

  //提交
  form.on('submit(admin-login)', function(obj){
    //请求登入接口
    admin.req({
      url: '/user/login'
      ,type: 'post'
      ,data: obj.field
      ,done: function(res){

        //登入成功的提示与跳转
        layer.msg('登入成功', {
          offset: '15px'
          ,icon: 1
          ,time: 1000
        }, function(){
          location.hash = search.redirect ? decodeURIComponent(search.redirect) : '/';
        });
      }
    });
  });

  // 修改管理员
  form.on('submit(change-admin)', function(obj){
    if(obj.field.password != obj.field.re_password) {
      return layer.msg("两次输入的新密码不一致。请重新输入");
    }

    admin.req({
      url: '/user/detail'
      ,type: 'post'
      ,data: obj.field
      ,done: function(res){
        if(res.code === 0) {
          layer.msg(res.msg, {
            offset: '15px'
            ,icon: 1
            ,time: 1000
          });
        } else {
          layer.msg(res.msg);
        }
      }
    });
    
  });
  
  //对外暴露的接口
  exports('user', {});
});