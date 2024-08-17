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
$(".m-menu-open").on("click", function () {
  $(".header").addClass("m-nav-show");
});
$(".m-menu-close").on("click", function () {
  $(".header").removeClass("m-nav-show");
});

$(".js-to-top").on("click", function () {
  $win.scrollTop(0);
});

var $singleTable = $(".single-arc table");
if ($singleTable.length > 0) {
  $singleTable.each(function () {
    $(this).wrap('<div class="single-table">');
  });
}
$("button.accordion-item").click(function () {
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
$("button.accordion-item").first().click();
$('.tool-item').click(function() {
  $(this).addClass('active').siblings().removeClass('active');
})