/** layuiAdmin.pro-v1.2.1 LPPL License By http://www.layui.com/admin/ */
;layui.define(["laytpl", "layer", "form"], function(e) {
    var t = layui.jquery
        , a = layui.laytpl
        , n = layui.layer
        , r = layui.setter
        , o = (layui.device(),
        layui.hint())
        , i = function(e) {
        return new d(e)
    }
        , s = "LAY_app_body"
        , d = function(e) {
        this.id = e,
            this.container = t("#" + (e || s))
    };
    i.loading = function(e) {
        e.append(this.elemLoad = t('<i class="layui-anim layui-anim-rotate layui-anim-loop layui-icon layui-icon-loading layadmin-loading"></i>'))
    }
        ,
        i.removeLoad = function() {
            this.elemLoad && this.elemLoad.remove()
        }
        ,
        i.exit = function() {
            layui.data(r.tableName, {
                key: r.request.tokenName,
                remove: !0
            }),
                location.hash = "/user/login"
        }
        ,
        i.req = function(e) {
            if(e.url.indexOf('http') !== 0) {
                if(e.url.indexOf('/') === 0) {
                    e.url = e.url.substring(1);
                }
                e.url = r.baseApi + e.url;
            }
            var a = e.success
                , n = (e.error,
                r.request)
                , o = r.response
                , s = function() {
                return r.debug ? "<br><cite>URL：</cite>" + e.url : ""
            };
            if (e.data = e.data || {},
                    e.headers = e.headers || {},
                    n.tokenName) {
                var d = "string" == typeof e.data ? JSON.parse(e.data) : e.data;
                e.data[n.tokenName] = n.tokenName in d ? e.data[n.tokenName] : layui.data(r.tableName)[n.tokenName] || "",
                    e.headers[n.tokenName] = n.tokenName in e.headers ? e.headers[n.tokenName] : layui.data(r.tableName)[n.tokenName] || ""
            }
            
            if (e.method == 'post' || e.type == 'post') {
                e.contentType = 'application/json; charset=utf-8';

                //对data值的[]改成多维数组
                let newData = {}
                layui.each(e.data, function(i, item){
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

                e.data = JSON.stringify(newData);
            }
            return delete e.success,
                delete e.error,
                t.ajax(t.extend({
                    type: "get",
                    dataType: "json",
                    success: function(t) {
                        var n = o.statusCode;
                        if (t[o.statusName] == n.ok)
                            "function" == typeof e.done && e.done(t);
                        else if (t[o.statusName] == n.logout)
                            i.exit();
                        else {
                            var r = ["<cite>Error：</cite> " + (t[o.msgName] || "返回状态码异常"), s()].join("");
                            i.error(r)
                        }
                        "function" == typeof a && a(t)
                    },
                    error: function(e, t) {
                        var a = ["请求异常，请重试<br><cite>错误信息：</cite>" + t, s()].join("");
                        i.error(a),
                        "function" == typeof a && a(res)
                    }
                }, e))
        }
        ,
        i.popup = function(e) {
            var a = e.success
                , r = e.skin;
            return delete e.success,
                delete e.skin,
                n.open(t.extend({
                    type: 1,
                    title: "提示",
                    content: "",
                    id: "LAY-system-view-popup",
                    skin: "layui-layer-admin" + (r ? " " + r : ""),
                    shadeClose: !0,
                    closeBtn: !1,
                    success: function(e, r) {
                        var o = t('<i class="layui-icon" close>&#x1006;</i>');
                        e.append(o),
                            o.on("click", function() {
                                n.close(r)
                            }),
                        "function" == typeof a && a.apply(this, arguments)
                    }
                }, e))
        }
        ,
        i.error = function(e, a) {
            return i.popup(t.extend({
                content: e,
                maxWidth: 300,
                offset: "t",
                anim: 6,
                id: "LAY_adminError"
            }, a))
        },
        i.exportFile = function(titles, data, type, fileName){
            
            type = type || 'csv';
            
            var textType = ({
              csv: 'text/csv'
              ,xls: 'application/vnd.ms-excel'
            })[type]
            ,alink = document.createElement("a");
            
            if(layui.device.ie) return n.msg('IE_NOT_SUPPORT_EXPORTS');
            
            alink.href = 'data:'+ textType +';charset=utf-8,\ufeff'+ encodeURIComponent(function(){
              let content = "";
                
                if (type == "csv") {
                  content = titles.join(',') + '\r\n' + data.join('\r\n');
                } else {
                    content += '<table border=1><thead><tr>';
                    //表头
                    layui.each(titles, function(i, item){
                        content += '<th>'+item+'</th>';
                    });
                    content += '</tr></thead>';
                    //表体
                    content += '<tbody>';
                    layui.each(data, function(i, item){
                        content += '<tr>';
                            layui.each(item, function(j, val){
                                content += '<td>'+val+'</td>';
                            });
                        content += '</tr>';
                    });
                    content += '</tbody>';
                    content += '<table>';
                }

                return content;
            }());
            
            alink.download = (fileName || 'table_'+ (new Date).getTime()) + '.' + type; 
            document.body.appendChild(alink);
            alink.click();
            document.body.removeChild(alink); 
          }
        ,
        d.prototype.render = function(e, a) {
            var n = this;
            layui.router();
            return e = r.views + e + r.engine,
                t("#" + s).children(".layadmin-loading").remove(),
                i.loading(n.container),
                t.ajax({
                    url: e,
                    type: "get",
                    dataType: "html",
                    data: {
                        v: layui.cache.version
                    },
                    success: function(e) {
                        e = "<div>" + e + "</div>";
                        var r = t(e).find("title")
                            , o = r.text() || (e.match(/\<title\>([\s\S]*)\<\/title>/) || [])[1]
                            , s = {
                            title: o,
                            body: e
                        };
                        r.remove(),
                            n.params = a || {},
                        n.then && (n.then(s),
                            delete n.then),
                            n.parse(e),
                            i.removeLoad(),
                        n.done && (n.done(s),
                            delete n.done)
                    },
                    error: function(e) {
                        return i.removeLoad(),
                            n.render.isError ? i.error("请求视图文件异常，状态：" + e.status) : (404 === e.status ? n.render("tips/404") : n.render("tips/error"),
                                void (n.render.isError = !0))
                    }
                }),
                n
        }
        ,
        d.prototype.parse = function(e, n, r) {
            var s = this
                , d = "object" == typeof e
                , l = d ? e : t(e)
                , u = d ? e : l.find("*[template]")
                , c = function(e) {
                var n = a(e.dataElem.html())
                    , o = t.extend({
                    params: y.params
                }, e.res);
                e.dataElem.after(n.render(o)),
                "function" == typeof r && r();
                try {
                    e.done && new Function("d",e.done)(o)
                } catch (i) {
                    console.error(e.dataElem[0], "\n存在错误回调脚本\n\n", i)
                }
            }
                , y = layui.router();
            l.find("title").remove(),
                s.container[n ? "after" : "html"](l.children()),
                y.params = s.params || {};
            for (var p = u.length; p > 0; p--)
                !function() {
                    var e = u.eq(p - 1)
                        , t = e.attr("lay-done") || e.attr("lay-then")
                        , n = a(e.attr("lay-url") || "").render(y)
                        , r = a(e.attr("lay-data") || "").render(y)
                        , s = a(e.attr("lay-headers") || "").render(y);
                    try {
                        r = new Function("return " + r + ";")()
                    } catch (d) {
                        o.error("lay-data: " + d.message),
                            r = {}
                    }
                    try {
                        s = new Function("return " + s + ";")()
                    } catch (d) {
                        o.error("lay-headers: " + d.message),
                            s = s || {}
                    }
                    n ? i.req({
                        type: e.attr("lay-type") || "get",
                        url: n,
                        data: r,
                        dataType: "json",
                        headers: s,
                        success: function(a) {
                            c({
                                dataElem: e,
                                res: a,
                                done: t
                            })
                        }
                    }) : c({
                        dataElem: e,
                        done: t
                    })
                }();
            return s
        }
        ,
        d.prototype.send = function(e, t) {
            var n = a(e || this.container.html()).render(t || {});
            return this.container.html(n),
                this
        }
        ,
        d.prototype.refresh = function(e) {
            var t = this
                , a = t.container.next()
                , n = a.attr("lay-templateid");
            return t.id != n ? t : (t.parse(t.container, "refresh", function() {
                t.container.siblings('[lay-templateid="' + t.id + '"]:last').remove(),
                "function" == typeof e && e()
            }),
                t)
        }
        ,
        d.prototype.then = function(e) {
            return this.then = e,
                this
        }
        ,
        d.prototype.done = function(e) {
            return this.done = e,
                this
        }
        ,
        e("view", i)
});
