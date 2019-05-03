import React from 'react'
import Main from '../layouts/main'
import Articles from '../components/articles'
import Api from '../utils/api'
import Link from 'next/link'
import Error from 'next/error'
import './styles/article.css'
var showdown = require('showdown')
import hljs from 'highlight.js'
import 'highlight.js/styles/github.css'
var moment = require('moment')

class ArticlePage extends React.Component {
  static async getInitialProps({ query: { id }, res }) {
    let articleResult = await Api.articleDetail(id)
    let relatedResult = await Api.articleList()
    if (articleResult.code !== 0 && res) {
      res.statusCode = 404
      return { err: '分类内容不存在' }
    }

    return {
      article: articleResult.data,
      relatedArticles: relatedResult.data.articles,
      id: id,
    }
  }

  componentDidMount() {
    this.updateCodeSyntaxHighlighting()
  }

  componentDidUpdate() {
    this.updateCodeSyntaxHighlighting()
  }

  updateCodeSyntaxHighlighting = () => {
    document.querySelectorAll("pre code").forEach(block => {
      hljs.highlightBlock(block);
    });
  };

  renderMarkdown(message) {
    let converter = new showdown.Converter();
    return converter.makeHtml(message)
  }

  render() {
    const { article, relatedArticles, user, err } = this.props

    if(err){
      return <Error statusCode='404' />
    }

    let messageHtml = this.renderMarkdown(article.message)
    return (
      <Main user={user} title={article.seoTitle || article.title} keywords={article.keywords} description={article.description}>
        <div className='article-main'>
          <h1 className='article-title'>{article.title}</h1>
          <div className='article-meta-list'>
            {article.categories && article.categories.map((item, index) => {
              return (
                <span className='article-meta' key={item.id}>
                  <Link href={'/category/' + item.id}>
                    <a>{item.title}</a>
                  </Link>
                </span>
              )
            })}
            <span className='article-meta'>{article.author || 'Fesion'}</span>
            <time className='article-meta' dateTime={moment.unix(article.addTime).format()}>{moment.unix(article.addTime).format('YYYY-MM-DD')}</time>
            {user && user.isAdmin &&
              <span className='article-meta'>
                <Link href={'/create/' + article.id}>
                  <a>编辑</a>
                </Link>
              </span>
            }
            <span className='article-meta pull-right'>{article.views}°</span>
          </div>
          <div className='article-content' dangerouslySetInnerHTML={{ __html: messageHtml }}>
          </div>
          <div className='article-footer'>
            {article.PrevArticle && <Link href={'/article/' + article.PrevArticle.id}><a className='article-footer-item'>上一篇</a></Link>}
            {article.NextArticle && <Link href={'/article/' + article.NextArticle.id}><a className='article-footer-item text-right'>下一篇</a></Link>}
          </div>
          <Articles title='相关阅读' detail={false} articles={relatedArticles} nopadding />
        </div>
      </Main>
    )
  }
}

export default ArticlePage