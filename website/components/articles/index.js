import React from 'react'
import Article from '../article'
import PropTypes from 'prop-types'
import './index.css'

class Articles extends React.Component {
  constructor(props) {
    super(props)
  }

  render() {
    const { title, articles, detail, nopadding } = this.props

    return (
      <div className={'article-list-box' + (nopadding ? 'nopadding' : '')}>
        {title && <h3 className='article-header'>{title}</h3>}
        <div className='article-list'>
        {articles && articles.map((item, index) => {
          return (
            <Article key={index} article={item} detail={detail} />
          )
        })}
        </div>
      </div>
    )
  }
}

Articles.propTypes = {
  title: PropTypes.string,
  detail: PropTypes.bool,
  articles: PropTypes.array,
  nopadding: PropTypes.bool
}

export default Articles