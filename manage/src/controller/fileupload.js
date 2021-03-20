/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */

layui.define(['form', 'upload', 'table'], function (exports) {
  var $ = layui.$
    , layer = layui.layer
    , laytpl = layui.laytpl
    , setter = layui.setter
    , view = layui.view
    , admin = layui.admin
    , form = layui.form
    , upload = layui.upload
    , table = layui.table;

  //验证文件上传管理
  let fileuploadTable = table.render({
    elem: '#fileupload-manage'
    , url: setter.baseApi + 'plugin/fileupload/list'
    , cols: [[
      { field: 'file_name', title: '文件名', minWidth: 100 }
      , { field: 'created_time', width: 150, title: '添加时间', templet: '<div>{{layui.util.toDateString(d.created_time*1000, "yyyy-MM-dd HH:mm")}}</div>' }
      , { title: '操作', width: 220, align: 'center', fixed: 'right', toolbar: '#table-fileupload-toolbar' }
    ]]
    , page: true
    , limit: 20
    , text: '对不起，加载出现异常！'
  });

  table.on('tool(fileupload-manage)', function (obj) {
    let data = obj.data; //获得当前行数据
    let layEvent = obj.event; //获得 lay-event 对应的值（也可以是表头的 event 参数对应的值）
    if (layEvent === 'del') { //删除
      layer.confirm('真的删除这个验证文件吗？', function (index) {
        admin.req({
          url: '/plugin/fileupload/delete'
          , data: data
          , type: 'post'
          , done: function (res) {
            fileuploadTable.reload();//重载表格
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
    }
  });

  let uploadInst = upload.render({
    elem: '#upload-file' //绑定元素
    , url: setter.baseApi + 'plugin/fileupload/upload' //上传接口
    , accept: 'file'
    , acceptMime: 'text/*'
    , done: function (res) {
      if (res.code === 0) {
        fileuploadTable.reload();//重载表格
        layer.alert(res.msg, function () {
          layer.closeAll();
        });
      } else {
        layer.msg(res.msg, {
          offset: '15px'
          , icon: 2
        });
      }
    }
    , error: function () {
      //请求异常回调
      layer.msg("上传出错");
    }
  });

  //对外暴露的接口
  exports('fileupload', {});
});