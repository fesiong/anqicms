/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */
 
layui.define(['form', 'table', 'layedit'], function(exports){
  var $ = layui.$
  ,layer = layui.layer
  ,laytpl = layui.laytpl
  ,setter = layui.setter
  ,view = layui.view
  ,admin = layui.admin
  ,form = layui.form
  ,table = layui.table
  ,layedit = layui.layedit
  ,upload = layui.upload;
  var editorIndex = null;

  //文章管理
  let articleTable = table.render({
    elem: '#article-manage'
    ,url: setter.baseApi + 'article/list'
    ,cols: [[
      {field: 'id', width: 60,title: 'ID'}
      ,{field: 'title', title: '文章标题',minWidth:200, templet: '<div>{{d.title}}{{# if(d.thumb){ }}<span class="layui-badge">[图]</span>{{# } }}</div>'}
      ,{field: 'category_id',width: 150, title: '所属分类', templet: '<div>{{# if(d.category){ }}{{d.category.title}}{{# } }}</div>'}
      ,{field: 'views',width: 80, title: '浏览'}
      ,{field: 'created_time',width: 150, title: '发布时间', templet: '<div>{{layui.util.toDateString(d.created_time*1000, "yyyy-MM-dd HH:mm")}}</div>'}
      ,{field: 'updated_time',width: 150, title: '更新时间', templet: '<div>{{layui.util.toDateString(d.updated_time*1000, "yyyy-MM-dd HH:mm")}}</div>'}
      ,{title: '操作', width: 150, align:'center', fixed: 'right', toolbar: '#table-article-toolbar'}
    ]]
    ,page: true
    ,limit: 20
    ,text: '对不起，加载出现异常！'
  });
  //修改排序
  table.on('edit(article-manage)', function(obj){
    obj.data.sort = Number(obj.data.sort);
    admin.req({
      url: '/article/detail'
      ,data: obj.data
      ,type: 'post'
      ,done: function(res){
        articleTable.reload();
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
  
  //工具条操作
  form.on('submit(article-submit)', function(obj){
    let data = obj.field;
    data.id = Number(data.id);
    data.category_id = Number(data.category_id);
    if(!data.title) {
        return layer.msg("请填写文章标题");
    }
    //同步编辑器内容
    layedit.sync(editorIndex);
		data.content = $('#text-editor').val();
    admin.req({
        url: '/article/detail'
        ,data: data
        ,type: 'post'
        ,done: function(res){
            if (res.code == 0) {
              layer.msg(res.msg, {
                offset: '15px'
                ,icon: 1
                ,time: 1000
              }, function(){
                articleTable.reload(); //重载表格
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
  table.on('tool(article-manage)', function(obj){
    let data = obj.data; //获得当前行数据
    let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
  
    if(layEvent === 'del'){ //删除
      layer.confirm('真的删除这个文章吗？', function(index){
        admin.req({
          url: '/article/delete'
          ,data: obj.data
          ,type: 'post'
          ,done: function(res){
            articleTable.reload();//重载表格
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
        title: '修改文章'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-nav-edit'
        ,success: function(layero, index){
          let viewId = this.id;
          admin.req({
            url: '/article/detail'
            ,data: {id: data.id}
            ,type: 'get'
            ,done: function(res){
              if (res.code == 0) {
                view(viewId).render('content/article/article_form', res.data).done(function(){
                  //
                  form.render();
                  editorIndex = layedit.build('text-editor', {
                    height: 450,
                    uploadImage: {
                        url: '/attachment/upload',
                        type: 'post'
                    }
                  });
                });
              }else{
                layer.close(index);
                layer.msg(res.msg);
              }
            }
            ,fail: function(res){
              layer.close(index);
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
  //控制菜单操作
  let articleActive = {
    add: function(){
      admin.popup({
        title: '添加文章'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-article-add'
        ,success: function(layero, index){
          view(this.id).render('content/article/article_form').done(function(){
            //
            form.render();
            editorIndex = layedit.build('text-editor', {
              height: 450,
              uploadImage: {
                  url: '/attachment/upload',
                  type: 'post'
              }
            });
          });
        }
      });
    }
  };

  $('.article-control-btn').off('click').on('click', function(){
    var type = $(this).data('type');
    articleActive[type] ? articleActive[type].call(this) : '';
  });

  //对外暴露的接口
  exports('article', {});
});