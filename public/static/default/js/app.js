layui.use(['element', 'layedit', 'form', 'layer', 'carousel', 'code'], function(){
    var $ = layui.$;
    var element = layui.element;
    var layedit = layui.layedit;
    var form = layui.form;
    let layer = layui.layer;
    var editorIndex = null;
    var carousel = layui.carousel;

    layui.code({
        elem: 'pre'
    }); //引用code方法

    if($('#text-editor').length) {
        editorIndex = layedit.build('text-editor', {
            height: 450,
            uploadImage: {
                url: '/attachment/upload',
                type: 'post'
            }
        });
    }
    //监听提交
    form.on('submit(install)', function(data){
        let index = layer.load();
        $.ajax({
            url: "/install",
            method: "post",
            data: data.field,
            dataType: "json",
            success: function (res) {
                layer.close(index);
                if(res.code === 0) {
                    layer.open({
                        content: res.msg
                        ,btn: ['访问管理后台', '访问首页']
                        ,yes: function(index, layero){
                            window.location.href = "/system/";
                        }
                        ,btn2: function(index, layero){
                            window.location.href = "/";
                        }
                        ,cancel: function(){
                            window.location.href = "/";
                        }
                    });
                } else {
                    layer.msg(res.msg);
                }
            },
            error: function (err) {
                layer.close(index);
                layer.msg(err);
            }
        });
        return false;
    });

    //评论
    $('.comment-control .item').click(function(e) {
        let that = $(this);
        let parentId = $(this).parent().data('id');
        let parentUser = $(this).parent().data('user');
        let eventType = $(this).data('id');
        if (eventType === 'praise') {
            //赞
            $.ajax({
                url: '/comment/praise',
                method: "post",
                data: {id: parentId},
                dataType: "json",
                success: function (res) {
                    if(res.code === 0) {
                        layer.msg(res.msg, {
                            offset: '15px'
                            ,icon: 1
                            ,time: 1000
                        }, function(){
                            that.find('.vote-count').text(res.data.vote_count)
                            if (res.data.active) {
                                that.addClass('active');
                            } else {
                                that.removeClass('active');
                            }
                        });
                    } else {
                        layer.msg(res.msg);
                    }
                },
                error: function (err) {
                    layer.msg(err);
                }
            });
        } else if (eventType === 'reply') {
            //回复
            $('#parent-id-field').val(parentId);
            $('#comment-content-field').prop('placeholder', '回复：' + parentUser).focus();
        }
    });
    form.on('submit(comment-submit)', function(data){
        if(!data.field.content) {
            return layer.msg("请填写评论内容");
        }
        data.form.submit();
        return false;
    });

    //产品图片轮播
    carousel.render({
        elem: '#product-photos'
        ,width: '100%'
    });

    //展示电话
    $('#show-cellphone').click(function(){
        let cellphone = $(this).data('id')
        layer.alert('电话：'+cellphone);
    });

    //留言提交
    form.on('submit(guestbook-submit)', function(data){
        if(!data.field.user_name && !data.field.contact) {
            return layer.msg("请填写联系方式");
        }
        
        data.form.submit();
        return false;
    })
});