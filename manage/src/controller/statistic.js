/**

 @Name：layuiAdmin 设置
 @Author：贤心
 @Site：http://www.layui.com/admin/
 @License: LPPL
    
 */

layui.define(['form', 'table', 'element', 'echarts'], function (exports) {
    var $ = layui.$
        , layer = layui.layer
        , laytpl = layui.laytpl
        , setter = layui.setter
        , view = layui.view
        , admin = layui.admin
        , form = layui.form
        , element = layui.element
        , router = layui.router()
        , echarts = layui.echarts
        , table = layui.table;

    let statisticTable = table.render({
        elem: '#statistic-manage'
        , url: setter.baseApi + 'statistic/detail?is_spider=' + router.search.is_spider
        , cols: [[
            { field: 'created_time', width: 150, title: '时间', templet: '<div>{{layui.util.toDateString(d.created_time*1000, "yyyy-MM-dd HH:mm")}}</div>' }
            , { field: 'host', title: '域名', width: 150 }
            , { field: 'url', title: '访问地址', minWidth: 150 }
            , { field: 'ip', title: 'IP', width: 150 }
            , { field: 'device', title: '设备', width: 150 }
            , { field: 'http_code', title: '状态码', width: 150 }
            , { field: 'user_agent', title: '请求UA', width: 150 }
        ]]
        , page: true
        , limit: 20
        , text: '对不起，加载出现异常！'
    });

    if ($('#spider-echarts').length) {
        admin.req({
            url: '/statistic/spider'
            , data: { separate: 'day' }//显示天统计
            , type: 'get'
            , done: function (res) {
                let categories = [];
                let contents = [];
                for (let i in res.data) {
                    categories.push(res.data[i].statistic_date)
                    contents.push(res.data[i].total)
                }

                let options = {
                    title: {
                        text: '蜘蛛爬行',
                        x: 'center',
                        textStyle: {
                            fontSize: 14
                        }
                    },
                    tooltip: {
                        trigger: 'axis'
                    },
                    legend: {
                        data: ['', '']
                    },
                    xAxis: [{
                        type: 'category',
                        boundaryGap: false,
                        data: categories
                    }],
                    yAxis: [{
                        type: 'value'
                    }],
                    series: [{
                        name: 'PV',
                        type: 'line',
                        smooth: true,
                        itemStyle: { normal: { areaStyle: { type: 'default' } } },
                        data: contents
                    }]
                }

                let echartsSpider = echarts.init($('#spider-echarts')[0], layui.echartsTheme);
                echartsSpider.setOption(options);
                window.onresize = echartsSpider.resize;
            }
            , fail: function (res) {
                layer.msg(res.msg, {
                    offset: '15px'
                    , icon: 2
                });
            }
        });
    }

    if ($('#traffic-echarts').length) {
        admin.req({
            url: '/statistic/traffic'
            , data: { separate: 'day' }//显示天统计
            , type: 'get'
            , done: function (res) {
                let categories = [];
                let contents = [];
                for (let i in res.data) {
                    categories.push(res.data[i].statistic_date)
                    contents.push(res.data[i].total)
                }

                let options = {
                    title: {
                        text: '流量趋势',
                        x: 'center',
                        textStyle: {
                            fontSize: 14
                        }
                    },
                    tooltip: {
                        trigger: 'axis'
                    },
                    legend: {
                        data: ['', '']
                    },
                    xAxis: [{
                        type: 'category',
                        boundaryGap: false,
                        data: categories
                    }],
                    yAxis: [{
                        type: 'value'
                    }],
                    series: [{
                        name: 'PV',
                        type: 'line',
                        smooth: true,
                        itemStyle: { normal: { areaStyle: { type: 'default' } } },
                        data: contents
                    }]
                }

                let echartsSpider = echarts.init($('#traffic-echarts')[0], layui.echartsTheme);
                echartsSpider.setOption(options);
                window.onresize = echartsSpider.resize;
            }
            , fail: function (res) {
                layer.msg(res.msg, {
                    offset: '15px'
                    , icon: 2
                });
            }
        });
    }

    //对外暴露的接口
    exports('statistic', {});
});