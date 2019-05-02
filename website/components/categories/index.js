import React from 'react'
import PropTypes from 'prop-types'
import Category from '../category'
import './index.css'

class Categories extends React.Component {
  constructor(props) {
    super(props)
  }

  componentWillMount() {
    
  }

  render() {
    const { categories, detail, title } = this.props

    return (
      <div className='category-list-box'>
        {title && 
          <div className='category-header'>{title}</div>
        }
        <div className='category-list'>
        {categories && categories.map((item, index) => {
          return (
            <Category key={index} category={item} detail={detail} />
          )
        })}
        </div>
      </div>
    )
  }
}

Categories.propTypes = {
  categories: PropTypes.array,
  detail: PropTypes.bool,
  title: PropTypes.string
}

export default Categories