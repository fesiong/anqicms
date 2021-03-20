/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL

 */

layui.define(['form', 'table', 'element'], function (exports) {
    var $ = layui.$
        , layer = layui.layer
        , laytpl = layui.laytpl
        , setter = layui.setter
        , view = layui.view
        , admin = layui.admin
        , form = layui.form
        , element = layui.element
        , table = layui.table;

    //留言管理
    let guestbookTable = table.render({
        elem: '#guestbook-manage'
        , url: setter.baseApi + 'plugin/guestbook/list'
        , cols: [[
            {checkbox: true}
            , { field: 'id', width: 60, title: 'ID' }
            , { field: 'user_name', title: '用户名', width: 100 }
            , { field: 'contact', width: 150, title: '联系方式'}
            , { field: 'content', minWidth: 200, title: '留言内容' }
            , { field: 'ip', width: 100, title: 'IP' }
            ,{field: 'created_time',width: 150, title: '时间', templet: '<div>{{layui.util.toDateString(d.created_time*1000, "yyyy-MM-dd HH:mm")}}</div>'}
            , { title: '操作', width: 220, align: 'center', fixed: 'right', toolbar: '#table-guestbook-toolbar' }
        ]]
        , page: true
        , limit: 20
        , text: '对不起，加载出现异常！'
    });

    table.on('tool(guestbook-manage)', function (obj) {
        let data = obj.data; //获得当前行数据
        let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）

        if (layEvent === 'del') { //删除
            layer.confirm('真的删除这个记录吗？', function (index) {
                admin.req({
                    url: '/plugin/guestbook/delete'
                    , data: data
                    , type: 'post'
                    , done: function (res) {
                        guestbookTable.reload();//重载表格
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
        } else if (layEvent === 'view') {
            admin.popup({
                title: '查看详情'
                , area: ['800px', '600px']
                , id: 'LAY-popup-guestbook-detail'
                , success: function (layero, index) {
                    view(this.id).render('plugin/guestbook/detail', data).done(function () {
                        form.render();
                    });
                }
            });
        }
    });

    //控制菜单操作
    let guestbookActive = {
        delete: function () {
            let checkStatus = table.checkStatus('guestbook-manage');
            if (checkStatus.data.length === 0) {
                layer.msg('请选择需要操作的数据', {
                    offset: '15px'
                    , icon: 2
                });
                return;
            }
            let ids = [];
            layui.each(checkStatus.data, function (i, item) {
                ids.push(item.id);
            });
            layer.confirm('真的删除选中的记录吗？', function (index) {
                admin.req({
                    url: '/plugin/guestbook/delete'
                    , data: { ids: ids }
                    , type: 'post'
                    , done: function (res) {
                        guestbookTable.reload();//重载表格
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
        },
        export: function () {
            //导出数据
            admin.req({
                url: '/plugin/guestbook/export'
                , data: {}
                , type: 'post'
                , done: function (res) {
                    view.exportFile(res.data.header, res.data.content, 'xls');
                }
                , fail: function (res) {
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 2
                    });
                }
            });
        },
    };

    $('.guestbook-control-btn').off('click').on('click', function () {
        var type = $(this).data('type');
        guestbookActive[type] ? guestbookActive[type].call(this) : '';
    });

    //setting
    if($('#setting-form').length) {
        let settingTable = null;
        let settingFields = [];

        admin.req({
            url: '/plugin/guestbook/setting'
            ,type: 'get'
            , done: function (res) {
                if (res.code === 0) {
                    //读取成功
                    //赋值
                    form.val('setting-form', res.data);
                    settingFields = res.data.fields;
                    //table render
                    settingTable = table.render({
                        elem: '#guestbook-setting'
                        , data: settingFields
                        , cols: [[
                            { field: 'name', title: '名称', width: 150 }
                            , { field: 'field_name', title: '字段名称', minWidth: 150 }
                            , { field: 'type', width: 150, title: '字段类型', templet: '<div>{{d.is_system ? "(内置)" : ""}}{{d.type}}</div>'}
                            , { field: 'required', width: 150, title: '是否必填', templet: '<div>{{d.required ? "是" : "否"}}</div>'}
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
        $('.setting-control-btn').off('click').on('click', function(){
            //添加一行数据
            admin.popup({
                title: '添加字段'
                , area: ['800px', '600px']
                , id: 'LAY-popup-setting-edit'
                , success: function (layero, index) {
                    view(this.id).render('plugin/guestbook/field_form', {index: -1}).done(function () {
                        form.render();
                    });
                }
            });
        });

        table.on('tool(guestbook-setting)', function (obj) {
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
            if(!data.name) {
                return layer.msg('请填写字段名称');
            }
            if (!data.field_name) {
                data.field_name = data.name;
            }
            //检查字段是否重复，重复会直接覆盖
            for (let i in settingFields) {
                if ((settingFields[i].field_name == data.field_name || settingFields[i].name == data.name) && i != data.index) {
                    return layer.msg('字段名称“'+data.field_name+'”已被占用');
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
                url: '/plugin/guestbook/setting'
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
    exports('guestbook', {});
});