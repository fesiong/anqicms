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
        , table = layui.table;

    //锚文本管理
    let anchorTable = table.render({
        elem: '#anchor-manage'
        , url: setter.baseApi + 'plugin/anchor/list'
        , cols: [[
            {checkbox: true}
            , { field: 'id', width: 60, title: 'ID' }
            , { field: 'title', title: '锚文本', width: 150, edit: 'text' }
            , { field: 'link', minWidth: 200, title: '锚文本连接', edit: 'text' }
            , { field: 'weight', width: 100, title: '权重', edit: 'text' }
            , { field: 'replace_count', width: 100, title: '替换次数' }
            , { title: '操作', width: 220, align: 'center', fixed: 'right', toolbar: '#table-anchor-toolbar' }
        ]]
        , page: false
        , limit: 100
        , text: '对不起，加载出现异常！'
    });
    //修改
    table.on('edit(anchor-manage)', function (obj) {
        obj.data.weight = Number(obj.data.weight);
        admin.req({
            url: '/plugin/anchor/detail'
            , data: obj.data
            , type: 'post'
            , done: function (res) {
                anchorTable.reload();
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
    form.on('submit(anchor-submit)', function (obj) {
        let data = obj.field;
        data.id = Number(data.id);
        data.weight = Number(data.weight);
        admin.req({
            url: '/plugin/anchor/detail'
            , data: data
            , type: 'post'
            , done: function (res) {
                if (res.code === 0) {
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 1
                        , time: 1000
                    }, function () {
                        anchorTable.reload(); //重载表格
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
    table.on('tool(anchor-manage)', function (obj) {
        let data = obj.data; //获得当前行数据
        let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）

        if (layEvent === 'del') { //删除
            layer.confirm('真的删除这个锚文本吗？', function (index) {
                admin.req({
                    url: '/plugin/anchor/delete'
                    , data: data
                    , type: 'post'
                    , done: function (res) {
                        anchorTable.reload();//重载表格
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
                title: '修改锚文本'
                , area: ['600px', '400px']
                , id: 'LAY-popup-anchor-edit'
                , success: function (layero, index) {
                    view(this.id).render('plugin/anchor/form', data).done(function () {
                        form.render();
                    });
                }
            });
        } else if (layEvent === 'replace') {
            admin.req({
                url: '/plugin/anchor/replace'
                , data: { id: data.id }
                , type: 'post'
                , done: function (res) {
                    anchorTable.reload();//重载表格
                    layer.close(index);
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 2
                    });
                }
                , fail: function (res) {
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 2
                    });
                }
            });
        }
    });

    //控制菜单操作
    let anchorActive = {
        add: function () {
            admin.popup({
                title: '添加锚文本'
                , area: ['600px', '400px']
                , id: 'LAY-popup-anchor-add'
                , success: function (layero, index) {
                    view(this.id).render('plugin/anchor/form').done(function () {
                        form.render();
                    });
                }
            });
        },
        delete: function () {
            let checkStatus = table.checkStatus('anchor-manage');
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
            layer.confirm('真的删除选中的锚文本吗？', function (index) {
                admin.req({
                    url: '/plugin/anchor/delete'
                    , data: { ids: ids }
                    , type: 'post'
                    , done: function (res) {
                        anchorTable.reload();//重载表格
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
        replace: function () {
            //替换操作, 批量执行
            admin.req({
                url: '/plugin/anchor/replace'
                , data: {}
                , type: 'post'
                , done: function (res) {
                    anchorTable.reload();//重载表格
                    layer.close(index);
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 2
                    });
                }
                , fail: function (res) {
                    layer.msg(res.msg, {
                        offset: '15px'
                        , icon: 2
                    });
                }
            });
        },
        export: function () {
            //导出数据
            admin.req({
                url: '/plugin/anchor/export'
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
                title: '导入锚文本'
                , area: ['600px', '400px']
                , id: 'LAY-popup-anchor-import'
                , success: function (layero, index) {
                    view(this.id).render('plugin/anchor/import').done(function () {
                        form.render();
                    });
                    //下载
                    $('.download-anchor-template').off('click').on('click', function () {
                        table.exportFile(['title', 'link', 'weight'], [['SEO','/a/123.html', 9]], 'csv');
                    });
                    //导入
                    let uploadInst = upload.render({
                        elem: '#upload-anchor' //绑定元素
                        ,url: '/plugin/anchor/import' //上传接口
                        ,done: function(res){
                          if(res.code === 0) {
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
                }
            });
        },
    };

    $('.anchor-control-btn').off('click').on('click', function () {
        var type = $(this).data('type');
        anchorActive[type] ? anchorActive[type].call(this) : '';
    });

    //setting
    form.on('submit(anchor-setting-submit)', function (obj) {
        let data = obj.field;
        data.anchor_density = Number(data.anchor_density);
        data.replace_way = Number(data.replace_way);
        data.keyword_way = Number(data.keyword_way);
        admin.req({
            url: '/plugin/anchor/setting'
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

    //对外暴露的接口
    exports('anchor', {});
});