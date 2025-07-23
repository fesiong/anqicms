getRem(375, 100);
window.onresize = function () {
  getRem(375, 100);
};
function getRem(pwidth, prem) {
  var html = document.getElementsByTagName("html")[0];
  var oWidth =
    document.body.clientWidth || document.documentElement.clientWidth;
  if (oWidth < 768) {
    html.style.fontSize = (oWidth / pwidth) * prem + "px";
  } else {
    html.style && html.style.fontSize && (html.style.fontSize = "");
  }
}
$win = $(window);
$(document).on("click", ".m-menu-open", function () {
  $(".header").addClass("m-nav-show");
});
$(document).on("click", ".m-menu-close", function () {
  $(".header").removeClass("m-nav-show");
});

$(document).on("click", ".js-to-top", function () {
  $win.scrollTop(0);
});

function renderTable() {
  var $singleTable = $(".single-arc table");
  if ($singleTable.length > 0) {
    $singleTable.each(function () {
      $(this).wrap('<div class="single-table">');
    });
  }
}

$(document).on("click", "button.accordion-item", function () {
  var contentEl = $(this).next(".accordion-item-content");
  if ($(this).hasClass("active")) {
    $(this).removeClass("active");
    contentEl.css("max-height", 0);
  } else {
    $(this).addClass("active").siblings().removeClass("active");
    contentEl
      .css("max-height", contentEl[0].scrollHeight)
      .siblings(".accordion-item-content")
      .css("max-height", 0);
  }
});

function initial() {
  renderTable();
  $("button.accordion-item").first().click();
}

initial();

$(document).on('click', '.tool-item', function() {
  $(this).addClass('active').siblings().removeClass('active');
})
// pjax
$(document).pjax('a', '#pjax-container');
$(document).on('pjax:beforeSend', function(xhr) {
  if ($(xhr.relatedTarget).data("pjax") === false) {
    window.location.href = $(xhr.relatedTarget).prop('href');
    return false
  }
})
$(document).on('pjax:send', function() {
  NProgress.start();
})
$(document).on('pjax:complete', function() {
  NProgress.done();
  initial();
})