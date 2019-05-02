var config = {
  apiURL: '/api/',
  backApiURL: 'http://127.0.0.1:8001/api/',
  siteName: '梁松远的博客',//网站名称后缀
  seoTitle: '看到你,梁松远的博客',//首页标题
  keywords: '梁松远的博客,看到你,一起发现,一起分享',//首页关键词
  description: '发现美好，请和我们一起分享',//首页描述
  primaryText: '看到你，梁松远的博客',//首页主要文字，不填则使用网站名称
  secondaryText: '人终其一生深觉两种东西始终不够使用，一是时间，二是空间',//首页首页第二行，不填则使用首页描述
  footerText: '看到你，分享带来快乐',//页脚次导航
  friendLinks: [
    {
      title: '粤ICP备15016830号',
      link: 'http://www.miibeian.gov.cn/',
      nofollow: !0
    },
    {
      title: '梁松远的博客',
      link: 'https://blog.kandaoni.com/',
      nofollow: !1
    }
  ],
  about: `
    这是一些介绍性文字
  `
}

module.exports = config
