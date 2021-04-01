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
  ,element = layui.element
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
                    ,url: setter.baseApi + 'attachment/upload' //上传接口
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
                  element.render();
                  renderMaterial();

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
              ,url: setter.baseApi + 'attachment/upload' //上传接口
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
            element.render();
            renderMaterial();

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

  
  function renderMaterial(){
    let materialCategoryId = 0;
    let materialTable = null;
    //请求category
    admin.req({
      url: '/plugin/material/category/list'
      ,data: {}
      ,type: 'get'
      ,done: function(res){
        layui.each(res.data, function(i, item){
          if (materialCategoryId == 0) {
            materialCategoryId = item.id;
            //拉取一遍
            renderMaterialList();
          }
        });
        laytpl('{{# layui.each(d.list, function(i, item){ }}<a href="javascript:;" class="layui-btn {{# if(d.category_id != item.id){ }}layui-btn-primary{{# } }} material-category-item" data-id="{{item.id}}">{{item.title}}({{item.material_count}})</a>{{# }); }}')
          .render({ list: res.data, category_id: materialCategoryId }, function (html) {
            $('.material-categories').html(html);
          });

          $(document).off('click', '.material-category-item').on('click', '.material-category-item', function(){
            $(this).removeClass('layui-btn-primary').siblings().addClass('layui-btn-primary');
            materialCategoryId = $(this).data('id');
            renderMaterialList();
          });
      }
      ,fail: function(res){
        // layer.msg(res.msg, {
        //   offset: '15px'
        //   ,icon: 2
        // });
      }
    });

    function renderMaterialList(){
      if(materialTable != null) {
        materialTable.reload({
          where: {
            category_id: materialCategoryId
          }
        });
      } else {
        materialTable = table.render({
          elem: '#material-list'
          , url: setter.baseApi + 'plugin/material/list'
          , where: {
            category_id: materialCategoryId
          }
          , cols: [[
            { field: 'id', width: 60, title: 'ID' }
            , { field: 'title', title: '素材名称', minWidth: 100 }
            , { field: 'use_count', title: '引用数量', width: 100}
            , { title: '操作', width: 220, align: 'center', fixed: 'right', toolbar: '#table-material-toolbar' }
          ]]
          , page: true
          , limit: 20
          , text: '对不起，加载出现异常！'
        });
        table.on('tool(material-list)', function (obj) {
          let data = obj.data; //获得当前行数据
          let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
          if (layEvent === 'view') {
            admin.popup({
              title: '查看内容素材'
              , area: ['800px', '600px']
              , id: 'LAY-popup-material-edit'
              , success: function (layero, index) {
                  view(this.id).render('plugin/material/detail', data).done(function () {
                      form.render();
                  });
              }
            });
          } else if (layEvent === 'use') {
            content = '<div data-material="'+data.id+'">'+data.content+'</div><p><br></p>';
            layedit.setContent(editorIndex, content, true);
          }
        });
      }
    }
  }

  //setting
  if ($('#setting-form').length) {
    let settingTable = null;
    let settingFields = [];

    admin.req({
      url: '/product/setting'
      , type: 'get'
      , done: function (res) {
        if (res.code === 0) {
          //读取成功
          //赋值
          form.val('setting-form', res.data);
          settingFields = res.data.fields || [];
          //table render
          settingTable = table.render({
            elem: '#product-setting'
            , data: settingFields
            , cols: [[
              { field: 'name', title: '名称', width: 150 }
              , { field: 'field_name', title: '字段名称', minWidth: 150 }
              , { field: 'type', width: 150, title: '字段类型', templet: '<div>{{d.is_system ? "(内置)" : ""}}{{d.type}}</div>' }
              , { field: 'required', width: 150, title: '是否必填', templet: '<div>{{d.required ? "是" : "否"}}</div>' }
              , { title: '操作', width: 220, align: 'center', fixed: 'right', toolbar: '#table-setting-toolbar' }
            ]]
            , page: false
            , limit: 100
            , text: '对不起，加载出现异常！'
          });
        } else {
          layer.msg(res.msg);
        }
      }
      , fail: function (res) {
        layer.msg(res.msg, {
          offset: '15px'
          , icon: 2
        });
      }
    });

    //
    $('.setting-control-btn').off('click').on('click', function () {
      //添加一行数据
      admin.popup({
        title: '添加字段'
        , area: ['800px', '600px']
        , id: 'LAY-popup-setting-edit'
        , success: function (layero, index) {
          view(this.id).render('plugin/guestbook/field_form', { index: -1 }).done(function () {
            form.render();
          });
        }
      });
    });

    table.on('tool(product-setting)', function (obj) {
      let data = obj.data; //获得当前行数据
      let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
      let dataIndex = $(obj.tr).data('index');
      data.index = dataIndex;
      if (layEvent === 'del') { //删除
        layer.confirm('真的删除这个字段吗？', function (index) {
          layer.close(index);
          //删除的时候先不请求后端
          settingFields.splice(dataIndex, 1);
          settingTable.reload({
            data: settingFields,
          });
        });
      } else if (layEvent === 'edit') {
        //编辑
        admin.popup({
          title: '修改字段'
          , area: ['800px', '600px']
          , id: 'LAY-popup-setting-edit'
          , success: function (layero, index) {
            view(this.id).render('plugin/guestbook/field_form', data).done(function () {
              form.render();
            });
          }
        });
      }
    });

    form.on('submit(field-submit)', function (obj) {
      let data = obj.field;
      if (!data.name) {
        return layer.msg('请填写字段名称');
      }
      if (!data.field_name) {
        data.field_name = data.name;
      }
      //检查字段是否重复，重复会直接覆盖
      for (let i in settingFields) {
        if ((settingFields[i].field_name == data.field_name || settingFields[i].name == data.name) && i != data.index) {
          return layer.msg('字段名称“' + data.field_name + '”已被占用');
        }
      }
      data.required = data.required ? true : false;
      //提交的时候，写入数据到object
      if (data.index != -1) {
        settingFields[data.index] = data;
      } else {
        settingFields.push(data);
      }
      //重载表格
      settingTable.reload({
        data: settingFields,
      });
      layer.closeAll();
    });

    form.on('submit(setting-submit)', function (obj) {
      let data = obj.field;
      //增加fields
      data.fields = settingFields;
      admin.req({
        url: '/product/setting'
        , data: data
        , type: 'post'
        , done: function (res) {
          if (res.code === 0) {
            layer.msg(res.msg, {
              offset: '15px'
              , icon: 1
              , time: 1000
            });
          } else {
            layer.msg(res.msg);
          }
        }
        , fail: function (res) {
          layer.msg(res.msg, {
            offset: '15px'
            , icon: 2
          });
        }
      });
    });
  }

  //对外暴露的接口
  exports('product', {});
});