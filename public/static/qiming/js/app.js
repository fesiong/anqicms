layui.use(['layer', 'form', 'element'], function(){
    let $ = layui.$, layer = layui.layer, form = layui.form, laydate = layui.laydate, element = layui.element;

    form.val("named-form", today);
    form.render();
    $('#show-more-item').click(function(){
        $('#more-item').toggleClass("layui-hide");
    })
    //calendar
    form.on('radio(calendar)', function(data){
        if(data.value == "lunar") {
            $('#lunar-leap').removeClass('layui-hide');
        } else {
            $('#lunar-leap').addClass('layui-hide');
        }
    });

    form.on('submit(named-form)', function(data){
        layer.load(1);
        $.ajax({
            type: "POST",
            url: "/name/create",
            contentType: "application/json; charset=utf-8",
            data: JSON.stringify(data.field),
            dataType: "json",
            success: function (res) {
                if (res.code === 0) {
                    window.location.href = "/naming?id=" + res.data
                } else {
                    layer.closeAll();
                    layer.msg(res.msg);
                }
            },
            error: function (err) {
                layer.closeAll();
                layer.msg("提交失败");
            }
        });
        return false;
    });
    form.on('submit(checked-form)', function(data){
        layer.load(1);
        $.ajax({
            type: "POST",
            url: "/name/checkout",
            contentType: "application/json; charset=utf-8",
            data: JSON.stringify(data.field),
            dataType: "json",
            success: function (res) {
                layer.closeAll();
                if (res.code === 0) {
                    window.location.href = "/detail/" + res.data
                } else {
                    layer.msg(res.msg);
                }
            },
            error: function (err) {
                layer.closeAll();
                layer.msg("提交失败");
            }
        });
        return false;
    });
    form.on('submit(horoscope-form)', function(data){
        layer.load(1);
        $.ajax({
            type: "POST",
            url: "/name/horoscope",
            contentType: "application/json; charset=utf-8",
            data: JSON.stringify(data.field),
            dataType: "json",
            success: function (res) {
                layer.closeAll();
                if (res.code === 0) {
                    window.location.href = "/horoscope/" + res.data
                } else {
                    layer.msg(res.msg);
                }
            },
            error: function (err) {
                layer.closeAll();
                layer.msg("提交失败");
            }
        });
        return false;
    });
    form.on('submit(article-form)', function(data){
        layer.load(1);
        $.ajax({
            type: "POST",
            url: "/article/publish",
            contentType: "application/json; charset=utf-8",
            data: JSON.stringify(data.field),
            dataType: "json",
            success: function (res) {
                layer.closeAll();
                layer.msg(res.msg, function(){
                    window.location.reload()
                });
            },
            error: function (err) {
                layer.closeAll();
                layer.msg("提交失败");
            }
        });
        return false;
    });
    //choose-load-more
    $('#choose-load-more').click(function(){
        layer.load(1);
        var that = this;
        var page = $(this).data('page');
        page++
        $(this).data('page', page);
        $.get(window.location.href, {page: page, ajax: true}, function(res){
            layer.closeAll();
            if(!res) {
                $(that).remove();
                layer.msg("没有更多了。");
                return;
            }
            $('#name-list').append(res);
        });
    })
});