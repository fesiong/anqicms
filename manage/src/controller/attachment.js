/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */
 
layui.define(['form', 'upload', 'laypage'], function(exports){
  var $ = layui.$
  ,layer = layui.layer
  ,laytpl = layui.laytpl
  ,setter = layui.setter
  ,view = layui.view
  ,admin = layui.admin
  ,form = layui.form
  ,laypage = layui.laypage
  ,upload = layui.upload;

  getAttachment(1, true);

  function getAttachment(page, initPage = false) {
    admin.req({
      url: '/attachment/list'
      ,data: {page: page}
      ,type: 'get'
      ,done: function(res){
        $('.attachment-list').html('');
        layui.each(res.data, function(i, item) {
          laytpl($('#attachment-item').html()).render(item, function(tpl) {
            $('.attachment-list').append(tpl);
          });
        });
        form.render();
        if (initPage) {
          laypage.render({
            elem: 'attachment-page' //注意，这里的 test1 是 ID，不用加 # 号
            ,limit: res.limit || 30
            ,count: res.count //数据总数，从服务端得到
            ,jump: function(obj, first){
              getAttachment(obj.curr, false);
            }
          });
        }
      }
      ,fail: function(res){
        layer.msg(res.msg, {
          offset: '15px'
          ,icon: 2
        });
      }
    });
  }

  //上传
  upload.render({
    elem: '#attachment-upload'
    ,url: setter.baseApi + 'attachment/upload'
    ,accept: 'images'
    ,acceptMime: 'image/jpg, image/png, image/gif'
    ,multiple: true
    ,before: function(){
      layer.load(1);
    }
    ,done: function(res, index, upload){ //上传后的回调
      //
    }
    ,allDone: function(obj){
      layer.closeAll();
      layer.msg("上传成功："+obj.successful+"张，失败："+obj.aborted+"张。", {
        offset: '15px'
        ,icon: 1
      });
      getAttachment(1, true);
    }
  })
  //控制菜单操作
  let attachmentActive = {
    delete: function() {
      let formVal = form.val("attachment-list-form");
      let checkedIds = [];
      layui.each(formVal, function(i, item) {
        if(i.indexOf('attachment') != -1) {
          checkedIds.push(item);
        }
      });
      if(checkedIds.length == 0) {
        return layer.msg("请选择需要删除的图片。");
      }
      layer.confirm("确定要执行删除操作吗？", function() {
        layui.each(checkedIds, function(i, item) {
          admin.req({
            url: '/attachment/delete'
            ,data: {id: Number(item)}
            ,type: 'post'
            ,done: function(res){
              if(res.code == 0) {
                layer.msg(res.msg, {
                  offset: '15px'
                  ,icon: 1
                });
                getAttachment(1, true);
              } else {
                layer.msg(res.msg, {
                  offset: '15px'
                  ,icon: 2
                });
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
      });
    }
  };

  $('.attachment-control-btn').off('click').on('click', function(){
    var type = $(this).data('type');
    attachmentActive[type] ? attachmentActive[type].call(this) : '';
  });

  //查看大图
  $(document).off('click', '.attachment-image').on('click', '.attachment-image', function(el) {
    let src = $(this).data('src');
    layer.open({
      type: 1,
      title: "查看大图",
      area: ['800px', '600px'],
      content: '<div class="attachment-preview"><img src="'+src+'" class="attachment-preview-img"></div>',
    });
  });

  //对外暴露的接口
  exports('attachment', {});
});