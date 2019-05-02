import React from 'react'
import PropTypes from 'prop-types'
import './index.css'

class Banner extends React.Component {
  constructor(props) {
    super(props)
  }

  render() {
    const { primary, secondary } = this.props
    return (
      <div className='branding'>
        <h1 className='primary-text'>{primary}</h1>
        <div className='scrondary-text'>{secondary}</div>
      </div>
    )
  }
}

Banner.propTypes = {
  primary: PropTypes.string,
  secondary: PropTypes.string,
}

export default Banner