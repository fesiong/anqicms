/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */
 
layui.define(['form', 'upload', 'table', 'element'], function(exports){
  var $ = layui.$
  ,layer = layui.layer
  ,laytpl = layui.laytpl
  ,setter = layui.setter
  ,view = layui.view
  ,admin = layui.admin
  ,form = layui.form
  ,element = layui.element
  ,table = layui.table;

  //友情链接管理
  let linkTable = table.render({
    elem: '#link-manage'
    ,url: setter.baseApi + 'plugin/link/list'
    ,cols: [[
      {field: 'sort', title: '排序',width:80, edit: 'text'}
      ,{field: 'title', title: '对方关键词/链接',minWidth:200, templet: '<div>{{d.title}} / {{d.link}}</div>'}
      ,{field: 'contact', width: 200,title: '对方联系方式/备注', templet: '<div>{{d.contact}} / {{d.remark}}</div>'}
      ,{field: 'status', title: '状态/检查时间', width: 220,templet: '<div><div class="link-status" data-id="{{d.id}}">{{# if(d.status == 0){ }}待检测{{# } else if(d.status == 1) { }}正常{{# }else if(d.status == 2){ }}NOFOLLOW{{# }else if(d.status == 3){ }}关键词不一致{{# }else if(d.status == 4){ }}对方无反链{{# } }} / {{layui.util.toDateString(d.checked_time*1000, "yyyy-MM-dd HH:mm")}}</div></div>'}
      ,{field: 'created_time', width: 150,title: '添加时间', templet: '<div>{{layui.util.toDateString(d.created_time*1000, "yyyy-MM-dd HH:mm")}}</div>'}
      ,{title: '操作', width: 220, align:'center', fixed: 'right', toolbar: '#table-link-toolbar'}
    ]]
    ,page: false
    ,limit: 100
    ,text: '对不起，加载出现异常！'
    ,done: function() {
      $('.link-status').each(function(i, item) {
        checkLink($(item).data('id'));
      });
    }
  });
  //修改排序
  table.on('edit(link-manage)', function(obj){
    obj.data.sort = Number(obj.data.sort);
    admin.req({
      url: '/plugin/link/detail'
      ,data: obj.data
      ,type: 'post'
      ,done: function(res){
        linkTable.reload();
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
  form.on('submit(link-submit)', function(obj){
    let data = obj.field;
    data.id = Number(data.id);
    data.nofollow = Number(data.nofollow);
    data.sort = Number(data.sort);
    admin.req({
        url: '/plugin/link/detail'
        ,data: data
        ,type: 'post'
        ,done: function(res){
            if (res.code == 0) {
              layer.msg(res.msg, {
                offset: '15px'
                ,icon: 1
                ,time: 1000
              }, function(){
                linkTable.reload(); //重载表格
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
  table.on('tool(link-manage)', function(obj){
    let data = obj.data; //获得当前行数据
    let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
  
    if(layEvent === 'del'){ //删除
      layer.confirm('真的删除这个链接吗？', function(index){
        admin.req({
          url: '/plugin/link/delete'
          ,data: data
          ,type: 'post'
          ,done: function(res){
            linkTable.reload();//重载表格
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
        title: '修改友情链接'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-link-edit'
        ,success: function(layero, index){
          view(this.id).render('plugin/link/link_form', data).done(function(){
            //
            element.render('collapse');
            form.render();
          });
        }
      });
    } else if(layEvent === 'check'){
      checkLink(data.id)
    }
  });
  //check link
  function checkLink(id) {
    let originTxt = $('.link-status[data-id='+id+']').html();
    $('.link-status[data-id='+id+']').html('<i class="layui-icon layui-icon-refresh layui-anim layui-anim-rotate layui-anim-loop"></i>');
    admin.req({
      url: '/plugin/link/check'
      ,data: {id: id}
      ,type: 'post'
      ,done: function(res){
        let statusText = '';
        if(res.data.status == 0) {
          statusText += "待检测";
        } else if(res.data.status == 1) {
          statusText += "正常";
        } else if(res.data.status == 2) {
          statusText += "NOFOLLOW";
        } else if(res.data.status == 3) {
          statusText += "关键词不一致";
        } else if(res.data.status == 4) {
          statusText += "对方无反链";
        }
        statusText += " / ";
        statusText += layui.util.toDateString(res.data.checked_time*1000, "yyyy-MM-dd HH:mm");
        $('.link-status[data-id='+id+']').html(statusText);
      }
      ,fail: function(res){
        $('.link-status[data-id='+id+']').html(originTxt);
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 2
        });
      }
    });
  }
  //控制菜单操作
  let linkActive = {
    add: function(){
      admin.popup({
        title: '添加友情链接'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-link-add'
        ,success: function(layero, index){
          view(this.id).render('plugin/link/link_form').done(function(){
            //
            element.render('collapse');
            form.render();
          });
        }
      });
    }
  };

  $('.link-control-btn').off('click').on('click', function(){
    var type = $(this).data('type');
    linkActive[type] ? linkActive[type].call(this) : '';
  });

  //对外暴露的接口
  exports('links', {});
});