layui.define(function (exports) {
  var $ = layui.$;
  var layer = layui.layer;
  var form = layui.form;
  var laytpl = layui.laytpl;

  //扩展jquery的格式化方法
  $.fn.parseForm = function () {
    var serializeObj = {};
    var array = this.serializeArray();
    var str = this.serialize();
    $(array).each(function () {
      if (serializeObj[this.name]) {
        if ($.isArray(serializeObj[this.name])) {
          serializeObj[this.name].push(this.value);
        } else {
          serializeObj[this.name] = [serializeObj[this.name], this.value];
        }
      } else {
        serializeObj[this.name] = this.value;
      }
    });
    return serializeObj;
  };
  // can parseData,
  /**
   *
   * @param {data: {}, action: string, callback: ():void => {}} obj
   */
  $.fn.jsonPost = function (obj = {}) {
    let action = $(this).parents("form,.layui-form").attr("action");
    if (obj.action) {
      action = obj.action;
    }
    $.ajax({
      url: action,
      contentType: "application/json;charset=utf-8",
      data: JSON.stringify(obj.data),
      type: "post",
      dataType: "json",
      async: false,
      success: function (res) {
        if (obj.callback) {
          obj.callback(res);
        } else {
          alert(res.msg);
          if (res.code === 0) {
            window.location.reload();
          }
        }
      },
    }).fail(function (err) {
      alert("提交出错了");
    });
  };

  $(".m-menu-open").on("click", function (e) {
    e.stopPropagation();
    $(".header").addClass("m-nav-show");
  });
  $(".m-menu-close").on("click", function (e) {
    e.stopPropagation();
    $(".header").removeClass("m-nav-show");
  });

  $(".menu-headermenu-container").on(
    "click",
    ".menu-item-has-children",
    function (e) {
      e.stopPropagation();
      $(this).toggleClass("sub-menu-show");
    }
  );

  var $singleTable = $(".arc table");
  if ($singleTable.length > 0) {
    $singleTable.each(function () {
      $(this).wrap('<div class="single-table">');
    });
  }

  $("body").on("click", function () {
    $(".header").removeClass("m-nav-show");
  });

  $(".js-to-top").on("click", function () {
    $(window).scrollTop(0);
  });

  $("#menu-sidebarmenu > .menu-item").on("click", function () {
    $(this)
      .toggleClass("side-nav-show")
      .siblings()
      .removeClass("side-nav-show");
  });
  $("#menu-sidebarmenu > .menu-item > a").on("click", function (e) {
    e.preventDefault();
  });

  

  // $("#login-container")
  //   .find("form")
  //   .on("submit", function () {
  //     $(this).jsonPost({
  //       parseData: function (data) {
  //         data.remember = data.remember ? true : false;
  //         data.invite_id = Number(data.invite_id);
  //         return data;
  //       },
  //       callback: function (res) {
  //         if (res.code === 0) {
  //           // login or register success
  //           window.location.href = "/";
  //         } else {
  //           alert(res.msg);
  //         }
  //       },
  //     });
  //   });

  exports('index', {});
});
