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

  //导航设置
  let navTable = table.render({
    elem: '#nav-manage'
    ,url: setter.baseApi + 'setting/nav'
    ,cols: [[
      {field: 'id', width: 60,title: 'ID'}
      ,{field: 'sort', title: '排序',width:80, edit: 'text'}
      ,{field: 'title', title: '显示名称',width:150, edit: 'text',templet: '<div>{{# if(d.parent_id != 0){ }}└&nbsp;&nbsp;{{# } }}{{d.title}}</div>'}
      ,{field: 'sub_title', minWidth: 100,title: '子标题', edit: 'text'}
      ,{field: 'description',minWidth: 100, title: '描述', edit: 'text'}
      ,{field: 'nav_type', title: '类型', width: 60,templet: '<div>{{# if(d.nav_type == 0){ }}内置{{# } else if(d.nav_type == 1) { }}分类{{# }else if(d.nav_type == 2){ }}外链{{# } }} </div>'}
      ,{field: 'link', minWidth: 100,title: '链接'}
      ,{title: '操作', width: 150, align:'center', fixed: 'right', toolbar: '#table-nav-toolbar'}
    ]]
    ,page: false
    ,limit: 100
    ,text: '对不起，加载出现异常！'
  });
  //修改排序
  table.on('edit(nav-manage)', function(obj){
    obj.data.sort = Number(obj.data.sort);
    admin.req({
      url: '/setting/nav'
      ,data: obj.data
      ,type: 'post'
      ,done: function(res){
        navTable.reload();
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
  form.on('radio(nav-type)', function(obj) {
    let val = obj.value;
    $('.nav-type-item').hide();
    $('.nav-type-item[data-id='+val+']').show();
  });
  form.on('submit(nav-submit)', function(obj){
    let data = obj.field;
    data.id = Number(data.id);
    data.parent_id = Number(data.parent_id);
    data.nav_type = Number(data.nav_type);
    data.page_id = Number(data.page_id);
    data.sort = Number(data.sort);
    admin.req({
        url: '/setting/nav'
        ,data: data
        ,type: 'post'
        ,done: function(res){
            if (res.code == 0) {
              layer.msg(res.msg, {
                offset: '15px'
                ,icon: 1
                ,time: 1000
              }, function(){
                navTable.reload(); //重载表格
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
  table.on('tool(nav-manage)', function(obj){
    let data = obj.data; //获得当前行数据
    let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
  
    if(layEvent === 'del'){ //删除
      layer.confirm('真的删除这个导航吗？', function(index){
        admin.req({
          url: '/setting/nav/delete'
          ,data: data
          ,type: 'post'
          ,done: function(res){
            navTable.reload();//重载表格
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
        title: '修改导航'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-nav-edit'
        ,success: function(layero, index){
          view(this.id).render('setting/nav_form', data).done(function(){
            //
            $('.nav-type-item').hide();
            $('.nav-type-item[data-id='+data.nav_type+']').show();
            form.render();
          });
        }
      });
    }
  });
  //控制菜单操作
  let navActive = {
    add: function(){
      admin.popup({
        title: '添加导航'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-nav-add'
        ,success: function(layero, index){
          view(this.id).render('setting/nav_form').done(function(){
            //
            $('.nav-type-item').hide();
            $('.nav-type-item[data-id=0]').show();
            form.render();
          });
        }
      });
    }
  };

  $('.nav-control-btn').off('click').on('click', function(){
    var type = $(this).data('type');
    navActive[type] ? navActive[type].call(this) : '';
  });

  //对外暴露的接口
  exports('nav', {});
});