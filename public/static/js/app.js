layui.use(['element', 'layedit', 'form', 'layer', 'carousel'], function(){
    var $ = layui.$;
    var element = layui.element;
    var layedit = layui.layedit;
    var form = layui.form;
    let layer = layui.layer;
    var editorIndex = null;
    var carousel = layui.carousel;

    function convertFormDataToObject(data) {
        //对data值的[]改成多维数组
        let newData = {}
        layui.each(data, function(i, item){
            if(/^.*\[\d*\]$/.test(i)){
                let ii = i.replace(/\[\d*\]/, '', i);
                if(typeof newData[ii] === 'undefined') {
                    newData[ii] = [];
                }
                newData[ii].push(item);
            } else if(n = i.match(/^(.*)\[(\s*)\]$/gi), n){
                if(typeof newData[n[0]] === 'undefined') {
                    newData[n[0]] = {};
                }
                newData[n[0]][n[1]] = item;
            } else {
                newData[i] = item
            }
        });

        return newData;
    }

    if($('#text-editor').length) {
        editorIndex = layedit.build('text-editor', {
            height: 450,
            uploadImage: {
                url: '/attachment/upload',
                type: 'post'
            }
        });
    }
    form.on('submit(login-submit)', function(obj){
        $.ajax({
            url: '/admin/login',
            method: "post",
            data: JSON.stringify(convertFormDataToObject(obj.field)),
            contentType: "application/json; charset=utf-8",
            dataType: "json",
            success: function (res) {
                if(res.code === 0) {
                    layer.msg('登录成功', {
                        offset: '15px'
                        ,icon: 1
                        ,time: 1000
                    }, function(){
                        window.location.href = '/';
                    });
                } else {
                    layer.msg(res.msg);
                }
            },
            error: function (err) {
                layer.msg(err);
            }
        });
    });
    //监听提交
    form.on('submit(install)', function(data){
        let index = layer.load();
        let postData = convertFormDataToObject(data.field);
        postData.port = Number(postData.port)
        $.ajax({
            url: "/install",
            method: "post",
            data: JSON.stringify(postData),
            contentType: "application/json; charset=utf-8",
            dataType: "json",
            success: function (res) {
                layer.close(index);
                if(res.code === 0) {
                    layer.alert(res.msg, function(){
                        window.location.href = "/";
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
    //发布文章
    form.on('submit(article-publish)', function(data){
        let postData = convertFormDataToObject(data.field);
        postData.id = Number(postData.id)
        if(!postData.title) {
            return layer.msg("请填写文章标题");
        }
        //同步编辑器内容
        layedit.sync(editorIndex);
		postData.content = $('#text-editor').val();
        $.ajax({
            url: "/article/publish",
            method: "post",
            data: JSON.stringify(postData),
            contentType: "application/json; charset=utf-8",
            dataType: "json",
            success: function (res) {
                if(res.code === 0) {
                    layer.alert(res.msg, function(){
                        window.location.href = "/article/" + res.data.id;
                    });
                } else {
                    layer.msg(res.msg);
                }
            },
            error: function (err) {
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
        if (eventType == 'praise') {
            //赞
            $.ajax({
                url: '/comment/praise',
                method: "post",
                data: JSON.stringify({id: parentId}),
                contentType: "application/json; charset=utf-8",
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
        } else if (eventType == 'reply') {
            //回复
            $('#parent-id-field').val(parentId);
            $('#comment-content-field').prop('placeholder', '回复：' + parentUser).focus();
        }
    });
    form.on('submit(comment-submit)', function(data){
        let postData = convertFormDataToObject(data.field);
        postData.id = Number(postData.id)
        postData.item_id = Number(postData.item_id)
        postData.parent_id = Number(postData.parent_id)
        if(!postData.content) {
            return layer.msg("请填写评论内容");
        }
        $.ajax({
            url: "/comment/publish",
            method: "post",
            data: JSON.stringify(postData),
            contentType: "application/json; charset=utf-8",
            dataType: "json",
            success: function (res) {
                if(res.code === 0) {
                    window.location.reload();
                } else {
                    layer.msg(res.msg);
                }
            },
            error: function (err) {
                layer.msg(err);
            }
        });
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
        let postData = convertFormDataToObject(data.field);
        if(!postData.user_name || !postData.contact) {
            return layer.msg("请填写联系方式");
        }
        let index = layer.load();
        $.ajax({
            url: "/guestbook.html",
            method: "post",
            data: JSON.stringify(postData),
            contentType: "application/json; charset=utf-8",
            dataType: "json",
            success: function (res) {
                layer.close(index);
                if(res.code === 0) {
                    layer.alert(res.msg, function(){
                        window.location.reload();
                    })
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
    })
});