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

  //分类管理
  let categoryTable = table.render({
    elem: '#category-manage'
    ,url: setter.baseApi + 'category/list'
    ,cols: [[
      {field: 'id', width: 60,title: 'ID'}
      ,{field: 'sort', title: '排序',width:80, edit: 'text'}
      ,{field: 'title', title: '分类名称',width:200, edit: 'text',templet: '<div>{{d.spacer}}{{d.title}}</div>'}
      ,{field: 'type', title: '类型', width: 60,templet: '<div>{{# if(d.type == 1){ }}文章{{# } else if(d.type == 2) { }}产品{{# }else if(d.type == 3){ }}页面{{# } }} </div>'}
      ,{field: 'description',minWidth: 100, title: '描述', edit: 'text'}
      ,{title: '操作', width: 150, align:'center', fixed: 'right', toolbar: '#table-category-toolbar'}
    ]]
    ,page: false
    ,limit: 100
    ,text: '对不起，加载出现异常！'
  });
  //修改排序
  table.on('edit(category-manage)', function(obj){
    obj.data.sort = Number(obj.data.sort);
    admin.req({
      url: '/category/detail'
      ,data: obj.data
      ,type: 'post'
      ,done: function(res){
        categoryTable.reload();
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
  form.on('submit(category-submit)', function(obj){
    let data = obj.field;
    data.id = Number(data.id);
    data.parent_id = Number(data.parent_id);
    data.type = Number(data.type);
    data.sort = Number(data.sort);
    if(!data.title) {
        return layer.msg("请填写分类名称");
    }
    //同步编辑器内容
    layedit.sync(editorIndex);
		data.content = $('#text-editor').val();
    admin.req({
        url: '/category/detail'
        ,data: data
        ,type: 'post'
        ,done: function(res){
            if (res.code == 0) {
              layer.msg(res.msg, {
                offset: '15px'
                ,icon: 1
                ,time: 1000
              }, function(){
                categoryTable.reload(); //重载表格
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
  table.on('tool(category-manage)', function(obj){
    let data = obj.data; //获得当前行数据
    let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
  
    if(layEvent === 'del'){ //删除
      layer.confirm('真的删除这个导航吗？', function(index){
        admin.req({
          url: '/category/delete'
          ,data: data
          ,type: 'post'
          ,done: function(res){
            categoryTable.reload();//重载表格
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
        title: '修改分类'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-nav-edit'
        ,success: function(layero, index){
          view(this.id).render('content/category/category_form', data).done(function(){
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
  });
  //控制菜单操作
  let categoryActive = {
    add: function(){
      admin.popup({
        title: '添加分类'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-category-add'
        ,success: function(layero, index){
          view(this.id).render('content/category/category_form').done(function(){
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

  $('.category-control-btn').off('click').on('click', function(){
    var type = $(this).data('type');
    categoryActive[type] ? categoryActive[type].call(this) : '';
  });

  //对外暴露的接口
  exports('category', {});
});