/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */

layui.define(['form', 'table', 'layedit'], function (exports) {
  var $ = layui.$
    , layer = layui.layer
    , laytpl = layui.laytpl
    , setter = layui.setter
    , view = layui.view
    , admin = layui.admin
    , form = layui.form
    , table = layui.table
    , element = layui.element
    , upload = layui.upload;

  //setting
  form.on('submit(setting-submit)', function (obj) {
    let data = obj.field;
    data.title_min_length = Number(data.title_min_length);
    data.content_min_length = Number(data.content_min_length);
    data.category_id = Number(data.category_id);
    data.start_hour = Number(data.start_hour);
    data.end_hour = Number(data.end_hour);
    data.daily_limit = Number(data.daily_limit);

    data.auto_pseudo = data.auto_pseudo == '1' ? true : false;
    data.auto_dig_keyword = data.auto_dig_keyword == '1' ? true : false;

    data.title_exclude = data.title_exclude.trim().split("\n");
    data.title_exclude_prefix = data.title_exclude_prefix.trim().split("\n");
    data.title_exclude_suffix = data.title_exclude_suffix.trim().split("\n");
    data.content_exclude_line = data.content_exclude_line.trim().split("\n");
    data.content_exclude = data.content_exclude.trim().split("\n");
    data.link_exclude = data.link_exclude.trim().split("\n");
    data.content_replace = data.content_replace.trim().split("\n");

    admin.req({
      url: '/collector/setting'
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
  exports('collector', {});
});