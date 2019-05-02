import React from 'react'
import Link from 'next/link'
import config from '../../config'
import './index.css'

class Footer extends React.Component {
  constructor(props) {
    super(props)
  }

  render() {
    const {footerText, friendLinks} = config
    const year = (new Date()).getFullYear()
    return (
      <footer className='footer'>
        <span className='footer-item'>Copyright © {year}</span>
        <span className='footer-item'>{footerText}</span>
        {friendLinks.map((item, index) => {
          return(
            <span key={index} className='footer-item'><Link href={item.link}><a rel={item.nofollow ? 'nofollow' : 'friend'}>{item.title}</a></Link></span>
          )
        })}
      </footer>
    )
  }
}

export default Footer