import React from 'react'
import Link from 'next/link'
import config from '../../config'
import './index.css'

class Header extends React.Component {
  constructor(props) {
    super(props)
  }

  render() {
    const { user } = this.props
    const {siteName} = config
    return (
      <div>
        <header className='header'>
          <div className='header-container'>
            <Link href='/'>
              <a className='header-logo'>
                <img className='header-logo-logo' src='/static/logo.png' />
                <h2 className='header-logo-title'>{siteName}</h2>
              </a>
            </Link>
            <nav className='header-nav'>
              <Link href='/'>
                <a className='nav-item'>首页</a>
              </Link>
              <Link href='/category'>
                <a className='nav-item'>分类</a>
              </Link>
              <Link href='/about'>
                <a className='nav-item'>关于</a>
              </Link>
              {user && user.isAdmin &&
                <Link href='/create'>
                  <a className='nav-item'>发布</a>
                </Link>
              }
              {user &&
                <Link href='/sign/out'>
                  <a className='nav-item'>退出</a>
                </Link>
              }
            </nav>
          </div>
        </header>
        <div className='header-fixed'></div>
      </div>
    )
  }
}

export default Header