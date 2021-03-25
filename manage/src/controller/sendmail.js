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

    //留言邮件提醒管理
    let sendmailTable = table.render({
        elem: '#sendmail-manage'
        , url: setter.baseApi + 'plugin/sendmail/list'
        , cols: [[
            { field: 'created_time', width: 200, title: '发送时间', templet: '<div>{{layui.util.toDateString(d.created_time*1000, "yyyy-MM-dd HH:mm")}}</div>' }
            , { field: 'subject', title: '邮件标题', minWidth: 200 }
            , { field: 'status', title: '发送状态', minWidth: 200 }
        ]]
        , page: false
        , limit: 20
        , text: '对不起，加载出现异常！'
    });

    //读取配置信息
    admin.req({
        url: '/plugin/sendmail/setting'
        , data: {}
        , type: 'get'
        , done: function (res) {
            if (res.data.recipient || res.data.account) {
                if (res.data.recipient) {
                    $('#recipient').html(res.data.recipient);
                } else {
                    $('#recipient').html(res.data.account);
                }
            } else {
                $('#sendmail-test').remove();
            }
        }
        , fail: function (res) {
            layer.msg(res.msg, {
                offset: '15px'
                , icon: 2
            });
        }
    });

    //发送测试邮件
    $('#sendmail-test').click(function () {
        layer.confirm('确定要发送测试邮件吗？', function (index) {
            admin.req({
                url: '/plugin/sendmail/test'
                , data: {}
                , type: 'post'
                , done: function (res) {
                    sendmailTable.reload();//重载表格
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
    });

    //setting
    form.on('submit(sendmail-setting-submit)', function (obj) {
        let data = obj.field;
        data.use_ssl = Number(data.use_ssl);
        data.port = Number(data.port);
        admin.req({
            url: '/plugin/sendmail/setting'
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
    exports('sendmail', {});
});