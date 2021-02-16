/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */
 
layui.define(['form', 'table', 'layedit','upload'], function(exports){
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

  //产品管理
  let productTable = table.render({
    elem: '#product-manage'
    ,url: setter.baseApi + 'product/list'
    ,cols: [[
      {field: 'id', width: 60,title: 'ID'}
      ,{field: 'title', title: '产品标题',minWidth:200, templet: '<div>{{d.title}}{{# if(d.thumb){ }}<span class="layui-badge">[图]</span>{{# } }}</div>'}
      ,{field: 'category_id',width: 150, title: '所属分类', templet: '<div>{{# if(d.category){ }}{{d.category.title}}{{# } }}</div>'}
      ,{field: 'views',width: 80, title: '浏览'}
      ,{field: 'created_time',width: 150, title: '发布时间', templet: '<div>{{layui.util.toDateString(d.created_time*1000, "yyyy-MM-dd HH:mm")}}</div>'}
      ,{field: 'updated_time',width: 150, title: '更新时间', templet: '<div>{{layui.util.toDateString(d.updated_time*1000, "yyyy-MM-dd HH:mm")}}</div>'}
      ,{title: '操作', width: 150, align:'center', fixed: 'right', toolbar: '#table-product-toolbar'}
    ]]
    ,page: true
    ,limit: 20
    ,text: '对不起，加载出现异常！'
  });
  //修改排序
  table.on('edit(product-manage)', function(obj){
    obj.data.sort = Number(obj.data.sort);
    admin.req({
      url: '/product/detail'
      ,data: obj.data
      ,type: 'post'
      ,done: function(res){
        productTable.reload();
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
  form.on('submit(product-submit)', function(obj){
    let data = obj.field;
    data.id = Number(data.id);
    data.category_id = Number(data.category_id);
    data.price = Number(data.price);
    data.stock = Number(data.stock);
    if(!data.title) {
        return layer.msg("请填写产品标题");
    }
    //同步编辑器内容
    layedit.sync(editorIndex);
    data.content = $('#text-editor').val();
    admin.req({
        url: '/product/detail'
        ,data: data
        ,type: 'post'
        ,done: function(res){
            if (res.code === 0) {
              layer.msg(res.msg, {
                offset: '15px'
                ,icon: 1
                ,time: 1000
              }, function(){
                productTable.reload(); //重载表格
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
  table.on('tool(product-manage)', function(obj){
    let data = obj.data; //获得当前行数据
    let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
  
    if(layEvent === 'del'){ //删除
      layer.confirm('真的删除这个产品吗？', function(index){
        admin.req({
          url: '/product/delete'
          ,data: obj.data
          ,type: 'post'
          ,done: function(res){
            productTable.reload();//重载表格
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
        title: '修改产品'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-nav-edit'
        ,success: function(layero, index){
          let viewId = this.id;
          admin.req({
            url: '/product/detail'
            ,data: {id: data.id}
            ,type: 'get'
            ,done: function(res){
              if (res.code === 0) {
                view(viewId).render('content/product/product_form', res.data).done(function(){
                  
                  //图片上传
                  var uploadInst = upload.render({
                    elem: '#thumb-upload' //绑定元素
                    ,url: '/attachment/upload' //上传接口
                    ,done: function(res){
                      if(res.code == 0) {
                        //上传完毕回调
                        $('.thumb-images').append(laytpl('<div class="thumb-item">\
                        <input type="hidden" name="images[]" value="{{d.src}}">\
                        <img src="{{d.src}}" />\
                        <a href="javascript:;" class="remove-item" data-id="0"><i class="layui-icon layui-icon-close"></i></a>\
                      </div>').render({
                        src: res.data.src,
                    }));
                      } else {
                        layer.msg("上传出错");
                      }
                    }
                    ,error: function(){
                      //请求异常回调
                      layer.msg("上传出错");
                    }
                  });
                  
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
  let productActive = {
    add: function(){
      admin.popup({
        title: '添加产品'
        ,area: ['800px', '600px']
        ,id: 'LAY-popup-product-add'
        ,success: function(layero, index){
          view(this.id).render('content/product/product_form').done(function(){
            //
            //图片上传
            var uploadInst = upload.render({
              elem: '#thumb-upload' //绑定元素
              ,url: '/attachment/upload' //上传接口
              ,done: function(res){
                //上传完毕回调
                $('.thumb-images').append(laytpl('<div class="thumb-item">\
                <input type="hidden" name="images[]" value="{{d.src}}">\
                <img src="{{d.src}}" />\
                <a href="javascript:;" class="remove-item" data-id="0"><i class="layui-icon layui-icon-close"></i></a>\
              </div>').render({
                src: res.data.src,
              }));
              }
              ,error: function(){
                //请求异常回调
                layer.msg("上传出错");
              }
            });

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

  $('.product-control-btn').off('click').on('click', function(){
    var type = $(this).data('type');
    productActive[type] ? productActive[type].call(this) : '';
  });

  $(document).off('click','.remove-item').on('click','.remove-item', function(e) {
    let that = this;
    $(that).parents('.thumb-item').remove();
  })

  //对外暴露的接口
  exports('product', {});
});