import React from 'react'
import PropTypes from 'prop-types'
import Categories from '../categories'
import './index.css'

class Aside extends React.Component {
  constructor(props) {
    super(props)
  }

  render() {
    const { categories } = this.props
    return (
      <div className='aside'>
        <Categories detail={false} title='文章分类' categories={categories} />
      </div>
    )
  }
}

Aside.propTypes = {
  categories: PropTypes.array,
}

export default Aside