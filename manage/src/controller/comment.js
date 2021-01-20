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
  ,table = layui.table;

  //评论管理
  let commentTable = table.render({
    elem: '#comment-manage'
    ,url: setter.baseApi + 'plugin/comment/list'
    ,cols: [[
      {field: 'item_type', title: '类型',width:100, templet: '<div>{{# if(d.item_type == "article"){ }}文章{{# } else if(d.item_type == "product"){ }}产品{{# } }}</div>'}
      ,{field: 'item_title', title: '标题',width:200, templet: '<div>{{d.item_title}}</div>'}
      ,{field: 'user_name', title: '用户名',width:100}
      ,{field: 'content', minWidth: 200,title: '评论内容'}
      ,{field: 'ip', width: 150,title: 'IP'}
      ,{field: 'created_time', width: 150,title: '添加时间', templet: '<div>{{layui.util.toDateString(d.created_time*1000, "yyyy-MM-dd HH:mm")}}</div>'}
      ,{field: 'status', width: 80,title: '状态', templet: '<div>{{# if(d.status == 0){ }}审核中{{# } else if(d.status == 1){ }}正常{{# } }}</div>'}
      ,{title: '操作', width: 220, align:'center', fixed: 'right', toolbar: '#table-comment-toolbar'}
    ]]
    ,page: true
    ,limit: 20
    ,text: '对不起，加载出现异常！'
  });
  //审核
  form.on('submit(comment-check-submit)', function(obj){
    let data = obj.field;
    data.id = Number(data.id);
    admin.req({
        url: '/plugin/comment/check'
        ,data: {id: data.id}
        ,type: 'post'
        ,done: function(res){
            if (res.code == 0) {
              layer.msg(res.msg, {
                offset: '15px'
                ,icon: 1
                ,time: 1000
              }, function(){
                commentTable.reload(); //重载表格
                layer.closeAll(); //执行关闭
              });
          }else{
              layer.msg(res.msg);
          }
        }
        ,fail: function(res){
            layer.msg(res.msg, {
                offset: '15px'
                ,icon: 2
            });
        }
    });
  });
  //工具条操作
  form.on('submit(comment-submit)', function(obj){
    let data = obj.field;
    data.id = Number(data.id);
    admin.req({
        url: '/plugin/comment/detail'
        ,data: data
        ,type: 'post'
        ,done: function(res){
            if (res.code == 0) {
              layer.msg(res.msg, {
                offset: '15px'
                ,icon: 1
                ,time: 1000
              }, function(){
                commentTable.reload(); //重载表格
                layer.closeAll(); //执行关闭
              });
          }else{
              layer.msg(res.msg);
          }
        }
        ,fail: function(res){
            layer.msg(res.msg, {
                offset: '15px'
                ,icon: 2
            });
        }
    });
  });
  table.on('tool(comment-manage)', function(obj){
    let data = obj.data; //获得当前行数据
    let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
    if(layEvent === 'del'){ //删除
      layer.confirm('真的删除这个评论吗？', function(index){
        admin.req({
          url: '/plugin/comment/delete'
          ,data: data
          ,type: 'post'
          ,done: function(res){
            commentTable.reload();//重载表格
            layer.close(index);
          }
          ,fail: function(res){
            layer.msg(res.msg, {
              offset: '15px'
              ,icon: 2
            });
          }
        });
      });
    } else if(layEvent === 'edit'){
      //编辑
      admin.popup({
        title: '修改评论'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-comment-edit'
        ,success: function(layero, index){
          view(this.id).render('plugin/comment/comment_form', data).done(function(){
            //
          });
        }
      });
    } else if(layEvent === 'view'){
      //编辑
      admin.popup({
        title: '查看评论'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-comment-view'
        ,success: function(layero, index){
          let viewId = this.id;
          admin.req({
            url: '/plugin/comment/detail'
            ,data: {id: data.id}
            ,type: 'get'
            ,done: function(res){
              if(res.code == 0) {
                view(viewId).render('plugin/comment/comment_detail', res.data).done(function(){
                  //
                });
              } else {
                layer.close(index)
                layer.msg(res.msg, {
                  offset: '15px'
                  ,icon: 2
                });
              }
            }
            ,fail: function(res){
              layer.close(index)
              layer.msg(res.msg, {
                offset: '15px'
                ,icon: 2
              });
            }
          });
        }
      });
    }
  });

  //对外暴露的接口
  exports('comment', {});
});