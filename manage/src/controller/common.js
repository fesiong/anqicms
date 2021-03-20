/**

 @Name：layuiAdmin 公共业务
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License：LPPL
    
 */

layui.define(['laypage'], function (exports) {
  var $ = layui.$
    , layer = layui.layer
    , laytpl = layui.laytpl
    , setter = layui.setter
    , view = layui.view
    , admin = layui.admin
    , form = layui.form
    , laypage = layui.laypage

  //公共业务的逻辑处理可以写在此处，切换任何页面都会执行
  //……
  $(document).off('click', '#select-keywords').on('click', '#select-keywords', function () {
    let targetField = $('input#' + $(this).data('for'));
    admin.popup({
      title: '选择关键词'
      , area: ['650px', '450px']
      , id: 'LAY-popup-select-keyword'
      , success: function (layero, index2) {
        view(this.id).render('plugin/keyword/list', {}).done(function () {
          let keyword = '';
          //默认拉取一次
          getKeywords(1);

          //拉取数据
          function getKeywords(page) {
            admin.req({
              url: '/plugin/keyword/list'
              , type: 'get'
              , data: {
                keyword: keyword,
                page: page,
              }
              , done: function (res) {
                laytpl('{{# layui.each(d.list, function(i, item){ }}<span class="keyword-item"><input type="checkbox" name="keywords[]" value="{{item.title}}" title="{{item.title}}"  /></span>{{# }); }}')
                  .render({ list: res.data }, function (html) {
                    $('.keyword-result').html(html);
                    form.render();
                  });
                if (page == 1) {
                  renderPage(res.count);
                }
              }
            });
          }

          //分页处理
          function renderPage(total) {
            laypage.render({
              elem: 'keyword-page'
              , count: total
              , limit: 20
              , jump: function (obj, first) {
                //首次不执行
                if (!first) {
                  getKeywords(obj.curr);
                }
              }
            });
          }


          form.on('submit(search-keyword)', function (d) {
            keyword = d.field.keyword;
            getKeywords(1);
          })

          form.on('submit(selected-keyword-submit)', function (d) {
            let results = [];
            //已存在的关键词
            let values = $(targetField).val().split(",")
            for (let i in values) {
              results.push(values[i]);
            }
            for (let i in d.field) {
              //检查是否重复
              let exists = false;
              for (let j in results) {
                if (results[j] == d.field[i]) {
                  exists = true;
                }
              }
              if (!exists) {
                results.push(d.field[i]);
              }
            }
            $(targetField).val(results.join(','));
            layer.close(index2);
          })
        });
      }
    });
  });


  //退出
  admin.events.logout = function () {
    //执行退出接口
    admin.req({
      url: '/user/logout'
      , type: 'post'
      , data: {}
      , done: function (res) { //这里要说明一下：done 是只有 response 的 code 正常才会执行。而 succese 则是只要 http 为 200 就会执行

        //清空本地记录的 token，并跳转到登入页
        admin.exit();
      }
    });
  };


  //对外暴露的接口
  exports('common', {});
});