import React from 'react'
import Main from '../layouts/main'
import Articles from '../components/articles'
import Banner from '../components/banner'
import Aside from '../components/aside'
import Api from '../utils/api'
import Categories from '../components/categories'
import Link from 'next/link'
import Error from 'next/error'
import './styles/category.css'

class CagetoryPage extends React.Component {
  static async getInitialProps({ query: { id, page }, res }) {
    let categoryResult = {}, articlesResult = { data: {} }
    if (id) {
      categoryResult = await Api.categoryDetail(id)
      articlesResult = await Api.articleList({
        category_id: id,
        pageSize: 2,
      })
      if (categoryResult.code !== 0 && res) {
        res.statusCode = 404
        return { err: '分类内容不存在' }
      }
    }

    return {
      articles: articlesResult.data.articles,
      category: categoryResult.data,
      id: id,
      page: page,
      totalPage: articlesResult.data.totalPage,
      totalCount: articlesResult.data.totalCount
    }
  }

  componentDidMount() {

  }

  render() {
    const { id, page, category, categories, articles, user ,err } = this.props
    if(err){
      return <Error statusCode='404' />
    }
    
    return (
      <Main user={user} title={category.title} keywords={category.title} description={category.description}>
        {id && !category &&
          <div>404</div>
        }
        {category && <Banner primary={category.title} secondary={category.description} />}
        <div className='container'>
          {category &&
            <div className='main-content'>
              <Articles detail articles={articles} />
              <Link href='/category'><a className='paginator'>查看更多</a></Link>
            </div>
          }
          {!category &&
            <div className='main-content'>
              <Categories detail categories={categories} />
            </div>
          }

          {category && <Aside categories={categories} />}
        </div>
      </Main>
    )
  }
}

export default CagetoryPage