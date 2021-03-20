/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */

layui.define(['form', 'upload', 'table', 'element'], function (exports) {
    var $ = layui.$
        , layer = layui.layer
        , laytpl = layui.laytpl
        , setter = layui.setter
        , view = layui.view
        , admin = layui.admin
        , form = layui.form
        , element = layui.element
        , upload = layui.upload
        , table = layui.table;

    //关键词库管理
    let keywordTable = table.render({
        elem: '#keyword-manage'
        , url: setter.baseApi + 'plugin/keyword/list'
        , cols: [[
            {checkbox: true}
            , { field: 'id', width: 60, title: 'ID' }
            , { field: 'title', title: '关键词', minWidth: 200, edit: 'text' }
            , { title: '操作', width: 220, align: 'center', fixed: 'right', toolbar: '#table-keyword-toolbar' }
        ]]
        , page: true
        , limit: 20
        , text: '对不起，加载出现异常！'
    });
    //修改
    table.on('edit(keyword-manage)', function (obj) {
        obj.data.weight = Number(obj.data.weight);
        admin.req({
            url: '/plugin/keyword/detail'
            , data: obj.data
            , type: 'post'
            , done: function (res) {
                keywordTable.reload();
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

    //工具条操作
    form.on('submit(keyword-submit)', function (obj) {
        let data = obj.field;
        data.id = Number(data.id);
        data.weight = Number(data.weight);
        admin.req({
            url: '/plugin/keyword/detail'
            , data: data
            , type: 'post'
            , done: function (res) {
                if (res.code === 0) {
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 1
                        , time: 1000
                    }, function () {
                        keywordTable.reload(); //重载表格
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
    table.on('tool(keyword-manage)', function (obj) {
        let data = obj.data; //获得当前行数据
        let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）

        if (layEvent === 'del') { //删除
            layer.confirm('真的删除这个关键词吗？', function (index) {
                admin.req({
                    url: '/plugin/keyword/delete'
                    , data: data
                    , type: 'post'
                    , done: function (res) {
                        keywordTable.reload();//重载表格
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
                title: '修改关键词'
                , area: ['600px', '400px']
                , id: 'LAY-popup-keyword-edit'
                , success: function (layero, index) {
                    view(this.id).render('plugin/keyword/form', data).done(function () {
                        form.render();
                    });
                }
            });
        }
    });

    //控制菜单操作
    let keywordActive = {
        add: function () {
            admin.popup({
                title: '添加关键词'
                , area: ['600px', '400px']
                , id: 'LAY-popup-keyword-add'
                , success: function (layero, index) {
                    view(this.id).render('plugin/keyword/form').done(function () {
                        form.render();
                    });
                }
            });
        },
        delete: function () {
            let checkStatus = table.checkStatus('keyword-manage');
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
            layer.confirm('真的删除选中的关键词吗？', function (index) {
                admin.req({
                    url: '/plugin/keyword/delete'
                    , data: { ids: ids }
                    , type: 'post'
                    , done: function (res) {
                        keywordTable.reload();//重载表格
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
                url: '/plugin/keyword/export'
                , data: {}
                , type: 'post'
                , done: function (res) {
                    table.exportFile(res.data.header, res.data.content, 'csv');
                }
                , fail: function (res) {
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 2
                    });
                }
            });
        },
        import: function () {
            //导入数据
            admin.popup({
                title: '导入关键词'
                , area: ['600px', '400px']
                , id: 'LAY-popup-keyword-import'
                , success: function (layero, index) {
                    view(this.id).render('plugin/keyword/import').done(function () {
                        form.render();
                        //下载
                        $('#download-keyword-template').off('click').on('click', function () {
                            table.exportFile(['title'], [['SEO']], 'csv');
                        });
                        //导入
                        let uploadInst = upload.render({
                            elem: '#upload-keyword' //绑定元素
                            ,url: setter.baseApi + 'plugin/keyword/import' //上传接口
                            ,accept: 'file'
                            ,acceptMime: 'text/csv'
                            ,exts: 'csv'
                            ,done: function(res){
                            if(res.code === 0) {
                                keywordTable.reload();//重载表格
                                layer.alert(res.msg, function() {
                                    layer.closeAll();
                                });
                            } else {
                                layer.msg(res.msg, {
                                    offset: '15px'
                                    , icon: 2
                                });
                            }
                            }
                            ,error: function(){
                            //请求异常回调
                            layer.msg("上传出错");
                            }
                        });
                    });
                    
                }
            });
        },
    };

    $('.keyword-control-btn').off('click').on('click', function () {
        var type = $(this).data('type');
        keywordActive[type] ? keywordActive[type].call(this) : '';
    });

    //对外暴露的接口
    exports('keyword', {});
});