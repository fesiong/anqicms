import React from 'react'
import Head from 'next/head'
import Header from '../components/header'
import Footer from '../components/footer'
import config from '../config'
import './main.css'

export default class Main extends React.Component {

  render() {
    const { children, title = '', keywords = '', description = '', user } = this.props
    const {siteName} = config
    return (
      <div>
        <Head>
          <title>{title} - {siteName}</title>
          {keywords && <meta name="keywords" content={keywords}></meta>}
          {description && <meta name="description" content={description}></meta>}
          <meta charSet='utf-8' />
          <meta name='viewport' content='initial-scale=1.0, width=device-width' />
          <link rel='shortcut icon' href='/static/favicon.ico' type='image/ico' />
        </Head>
        <Header user={user} />
        {children}
        <Footer />
      </div>
    )
  }
}