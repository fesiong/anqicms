import React from 'react'
import Link from 'next/link'
import PropTypes from 'prop-types'
import './index.css'

const Category = (props) => {
  const { category, detail } = props

  return (
    <div className='category-item'>
      {!detail && <Link href={'/category/' + category.id}>
        <a className="category-link">{category.title}</a>
      </Link>
      }
      {detail &&
      <Link href={'/category/' + category.id}>
        <a className='category-detail'>
            {category.logo && <div className='category-logo'><img src={category.logo} /></div>}
            <div className='category-info'>
              <h3 className='detail-title'>{category.title}</h3>
              <div className='category-desc text-muted'>{category.description}</div>
            </div>
        </a>
        </Link>
      }
    </div>
  )
}

Category.propTypes = {
  category: PropTypes.object,
  detail: PropTypes.bool,
}

export default Category