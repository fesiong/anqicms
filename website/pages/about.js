import React from 'react'
import Main from '../layouts/main'
import './styles/article.css'

class About extends React.Component {
  
  componentDidMount() {
    
  }

  render() {
    const { user } = this.props
    return (
      <Main user={user} title='关于我'>
        <div className='article-main'>
          <h1 className='article-title'>关于我</h1>
          <div className='article-meta-list'>
            <span className='article-meta'>By Fesion</span>
            <span className='article-meta pull-right'>2019-05-01</span>
          </div>
          <div className='article-content'>
            <p>一个会点前端的后端工程师，使用过的开发语言有php、java、golang、lua、javascript。</p>
            <p>这个博客使用的是golang + next.js 搭建，如果您感兴趣，可以在我的github下载本博客源码：<a href='https://github.com/fesiong/goblog' target='blank' rel='nofollow'>https://github.com/fesiong/goblog</a></p>
            <p>我的github：<a href='https://github.com/fesiong' target='blank' rel='nofollow'>https://github.com/fesiong</a></p>
          </div>
        </div>
      </Main>
    )
  }
}

export default About