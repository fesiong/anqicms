import React from 'react'
import Main from '../layouts/main'
import Articles from '../components/articles'
import Banner from '../components/banner'
import Aside from '../components/aside'
import Link from 'next/link'
import Api from '../utils/api'
import './styles/index.css'
import config from '../config'

class App extends React.Component {
  static async getInitialProps(props) {
    let result = await Api.articleList();

    return {
      articles: result.data.articles,
      primary: config.primaryText,
      secondary: config.secondaryText,
    }
  }

  componentDidMount() {
    
  }

  render() {
    const { articles, categories, primary, secondary, user } = this.props
    const {seoTitle, keywords, description} = config
    return (
      <Main user={user} title={seoTitle} keywords={keywords} description={description}>
        <Banner primary={primary} secondary={secondary} />
        <div className='container'>
          <div className='main-content'>
            <Articles detail articles={articles} />
            <Link href='/category'><a className='paginator'>查看更多</a></Link>
          </div>
          <Aside categories={categories} />
        </div>
      </Main>
    )
  }
}

export default App