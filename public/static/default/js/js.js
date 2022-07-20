$win = $(window);
$('.m-menu-open').on('click', function (e) {
    e.stopPropagation();
    $('.header').addClass('m-nav-show');
});
$('.m-menu-close').on('click', function (e) {
    e.stopPropagation();
    $('.header').removeClass('m-nav-show');
});

$('.menu-headermenu-container').on('click', '.menu-item-has-children', function (e) {
    e.stopPropagation();
    $(this).toggleClass('sub-menu-show');
});

var $singleTable = $('.arc table');
if ($singleTable.length > 0) {
    $singleTable.each(function () {
        $(this).wrap('<div class="single-table">');
    });
}


$('body').on('click', function(){
    $('.header').removeClass('m-nav-show');
});

$win.on('scroll', function () {
    fnScroll();
});

$('.js-to-top').on('click', function () {
    $win.scrollTop(0);
});


$('#menu-sidebarmenu > .menu-item').on('click', function () {
    $(this).toggleClass('side-nav-show').siblings().removeClass('side-nav-show');
});
$('#menu-sidebarmenu > .menu-item > a').on('click', function (e) {
    e.preventDefault();
});