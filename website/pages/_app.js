import App, {Container} from 'next/app'
import React from 'react'
import Api from '../utils/api'

export default class MyApp extends App {
  static async getInitialProps ({ Component, router, ctx }) {
    let pageProps = {}
    if (Component.getInitialProps) {
      pageProps = await Component.getInitialProps(ctx)
    }
    //加载categories
    let categoriesResult = await Api.categoryList()
    pageProps.categories = categoriesResult.data
    return {pageProps}
  }
  state = {
    user: null
  }

  componentDidMount() {
    let user = localStorage.getItem('user')
    if(user){
      user = JSON.parse(user)
    }
    this.setState({
      user: user
    })
  }

  render () {
    const {Component, pageProps} = this.props
    const { user } = this.state
    pageProps.user = user
    return <Container>
      <Component {...pageProps} />
    </Container>
  }
}