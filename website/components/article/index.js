import React from 'react'
import Link from 'next/link'
import PropTypes from 'prop-types'
var moment = require('moment')
import './index.css'

const Article = (props) => {
  const { article, detail } = props
  return (
    <div className='article-item'>
      {!detail &&<Link href={'/article/' + article.id}>
      <a className="article-link"><time className='pull-right link-value text-muted' dateTime={moment.unix(article.addTime).format()}>{moment.unix(article.addTime).format('YYYY-MM-DD')}</time><h3 className='link-title'>{article.title}</h3></a>
      </Link>
      }
      {detail &&
        <div className='article-link-detail'>
          <h3 className='detail-title'>{article.title}</h3>
          <div className='detail-content'>{article.description}</div>
          <div>
            <div className='detail-footer'>
              <span className='detail-footer-item text-muted'>{article.views}°</span>
              <time className='detail-footer-item text-muted' dateTime={moment.unix(article.addTime).format()}>{moment.unix(article.addTime).format('YYYY-MM-DD')}</time>
            </div>
            <Link href={'/article/' + article.id}><a className='detail-link'>阅读全文</a></Link>
          </div>
        </div>
      }
    </div>
  )
}

Article.propTypes = {
  article: PropTypes.object,
  detail: PropTypes.bool,
}

export default Article