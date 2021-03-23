/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */

layui.define(['form', 'table', 'element','layedit'], function (exports) {
    var $ = layui.$
        , layer = layui.layer
        , laytpl = layui.laytpl
        , setter = layui.setter
        , view = layui.view
        , admin = layui.admin
        , form = layui.form
        , element = layui.element
        ,layedit = layui.layedit
        , table = layui.table;
        var editorIndex = null;

    //内容素材库管理
    let materialTable = table.render({
        elem: '#material-manage'
        , url: setter.baseApi + 'plugin/material/list'
        , cols: [[
            { checkbox: true }
            , { field: 'id', width: 60, title: 'ID' }
            , { field: 'title', title: '素材标题', minWidth: 200 }
            , { field: 'category_title', title: '素材分类', width: 200}
            , { field: 'use_count', title: '引用数量', width: 100}
            , { title: '操作', width: 220, align: 'center', fixed: 'right', toolbar: '#table-material-toolbar' }
        ]]
        , page: true
        , limit: 20
        , text: '对不起，加载出现异常！'
    });

    //工具条操作
    form.on('submit(material-submit)', function (obj) {
        let data = obj.field;
        data.id = Number(data.id);
        data.category_id = Number(data.category_id);
        data.auto_update = Number(data.auto_update);
        if(!data.title) {
            return layer.msg("请填写文章标题");
        }
        //同步编辑器内容
        layedit.sync(editorIndex);
        data.content = $('#text-editor').val();
        admin.req({
            url: '/plugin/material/detail'
            , data: data
            , type: 'post'
            , done: function (res) {
                if (res.code === 0) {
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 1
                        , time: 1000
                    }, function () {
                        materialTable.reload(); //重载表格
                        layer.closeAll(); //执行关闭
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
    table.on('tool(material-manage)', function (obj) {
        let data = obj.data; //获得当前行数据
        let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）

        if (layEvent === 'del') { //删除
            layer.confirm('真的删除这个内容素材吗？', function (index) {
                admin.req({
                    url: '/plugin/material/delete'
                    , data: data
                    , type: 'post'
                    , done: function (res) {
                        materialTable.reload();//重载表格
                        layer.close(index);
                    }
                    , fail: function (res) {
                        layer.msg(res.msg, {
                            offset: '15px'
                            , icon: 2
                        });
                    }
                });
            });
        } else if (layEvent === 'edit') {
            //编辑
            admin.popup({
                title: '修改内容素材'
                , area: ['800px', '600px']
                , id: 'LAY-popup-material-edit'
                , success: function (layero, index) {
                    view(this.id).render('plugin/material/form', data).done(function () {
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
        } else if (layEvent === 'view') {
            //编辑
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
        }
    });

    //控制菜单操作
    let materialActive = {
        add: function () {
            admin.popup({
                title: '添加内容素材'
                , area: ['800px', '600px']
                , id: 'LAY-popup-material-add'
                , success: function (layero, index) {
                    view(this.id).render('plugin/material/form').done(function () {
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
        },
        category: function () {
            //编辑素材分类
            admin.popup({
                title: '调整素材分类'
                , area: ['800px', '600px']
                , id: 'LAY-popup-material-category'
                , success: function (layero, index) {
                    view(this.id).render('plugin/material/category').done(function () {
                        //分类操作
                        //内容素材库管理
                        let categoryTable = table.render({
                            elem: '#material-category-manage'
                            , url: setter.baseApi + 'plugin/material/category/list'
                            , cols: [[
                                { checkbox: true }
                                , { field: 'id', width: 60, title: 'ID' }
                                , { field: 'title', title: '分类名称', minWidth: 200, edit: 'text' }
                                , { field: 'material_count', title: '素材数量', width: 100 }
                                , { title: '操作', width: 200, align: 'center', fixed: 'right', toolbar: '#table-material-category-toolbar' }
                            ]]
                            , page: true
                            , limit: 20
                            , text: '对不起，加载出现异常！'
                        });
                        //修改
                        table.on('edit(material-category-manage)', function (obj) {
                            obj.data.weight = Number(obj.data.weight);
                            admin.req({
                                url: '/plugin/material/category/detail'
                                , data: obj.data
                                , type: 'post'
                                , done: function (res) {
                                    categoryTable.reload();
                                    layer.msg(res.msg, {
                                        offset: '15px'
                                        , icon: 1
                                    });
                                }
                                , fail: function (res) {
                                    layer.msg(res.msg, {
                                        offset: '15px'
                                        , icon: 2
                                    });
                                }
                            });
                        });

                        table.on('tool(material-category-manage)', function (obj) {
                            let data = obj.data; //获得当前行数据
                            let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）

                            if (layEvent === 'del') { //删除
                                layer.confirm('真的删除这个内容素材分类吗？', function (index) {
                                    admin.req({
                                        url: '/plugin/material/category/delete'
                                        , data: data
                                        , type: 'post'
                                        , done: function (res) {
                                            categoryTable.reload();//重载表格
                                            layer.close(index);
                                        }
                                        , fail: function (res) {
                                            layer.msg(res.msg, {
                                                offset: '15px'
                                                , icon: 2
                                            });
                                        }
                                    });
                                });
                            } else if (layEvent === 'edit') {
                                //编辑
                                layer.prompt({
                                    value: data.title,
                                    title: '请填写素材分类名称',
                                  }, function(value, index2, elem){
                                    //提交到后台
                                    admin.req({
                                        url: '/plugin/material/category/detail'
                                        , data: {
                                            id: data.id,
                                            title: value,
                                        }
                                        , type: 'post'
                                        , done: function (res) {
                                            categoryTable.reload();//重载表格
                                            layer.close(index2);
                                        }
                                        , fail: function (res) {
                                            layer.msg(res.msg, {
                                                offset: '15px'
                                                , icon: 2
                                            });
                                        }
                                    });
                                    layer.close(index2);
                                  });
                            }
                        });

                        //控制菜单操作
                        let materialCategoryActive = {
                            add: function () {
                                layer.prompt({
                                    value: '',
                                    title: '请填写素材分类名称',
                                  }, function(value, index2, elem){
                                    //提交到后台
                                    admin.req({
                                        url: '/plugin/material/category/detail'
                                        , data: {
                                            title: value,
                                        }
                                        , type: 'post'
                                        , done: function (res) {
                                            categoryTable.reload();//重载表格
                                            layer.close(index2);
                                        }
                                        , fail: function (res) {
                                            layer.msg(res.msg, {
                                                offset: '15px'
                                                , icon: 2
                                            });
                                        }
                                    });
                                    layer.close(index2);
                                  });
                            },
                        };

                        $('.material-category-control-btn').off('click').on('click', function () {
                            var type = $(this).data('type');
                            materialCategoryActive[type] ? materialCategoryActive[type].call(this) : '';
                        });
                    });
                }
            });
        },
    };

    $('.material-control-btn').off('click').on('click', function () {
        var type = $(this).data('type');
        materialActive[type] ? materialActive[type].call(this) : '';
    });

    //对外暴露的接口
    exports('material', {});
});