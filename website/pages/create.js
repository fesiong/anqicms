import React from 'react'
import Main from '../layouts/main'
import Api from '../utils/api'
import Router from 'next/router'
import Error from 'next/error'
import { List, InputItem, TextareaItem, Tag, SearchBar, ImagePicker, Button } from 'antd-mobile'
import './styles/create.css'
import 'simplemde/dist/simplemde.min.css'
var simple
class CreatePage extends React.Component {
  static getInitialProps({ query: { id } }) {
    return { id: id }
  }

  state = {
    article: {
      categories: []
    },
    searchCategories: [],
    inputCategory: '',
    attachments: [],
  }

  componentDidMount() {
    const { id = 0 } = this.props
    if (id) {
      Api.articleDetail(id).then((res) => {
        this.setState({
          article: res.data
        })
        simple.value(res.data.message)
      })
    }
    //初始化编辑器
    var SimpleMDE = require('simplemde')
    simple = new SimpleMDE({
      element: document.getElementById("article-editor"),
    })

    simple.codemirror.on("change", () => {
      let { article } = this.state
      article.message = simple.value()
      this.setState({
        article: article
      })
    })
  }

  removeCategory = (e) => {
    let { article } = this.state
    article.categories.splice(e, 1)
    this.setState({
      article: article
    })
  }

  submitArticle = (e) => {
    let { article } = this.state
    Api.articleSave(article).then(res => {
      if (res.code == 0) {
        alert('发布成功')
        Router.push('/article/' + res.data.id)
      } else {
        alert(res.msg)
      }
    })
  }

  addCategory = (e) => {
    let category = this.state.searchCategories[e]
    let { article } = this.state
    let exists = false
    article.categories.forEach(item => {
      if (category.title == item.title) {
        exists = true
      }
    });
    if (!exists) {
      article.categories.push(category)
      this.setState({
        article: article,
        searchCategories: [],
        inputCategory: ''
      })
    }
  }

  changeInputCategory = (text) => {
    this.setState({
      inputCategory: text
    })

    Api.categoryList({ title: text }).then(res => {
      this.setState({
        searchCategories: res.data
      })
    })
  }

  checkAddCategory = (text) => {
    let category = {
      title: text
    }
    let { article } = this.state
    let exists = false
    article.categories.forEach(item => {
      if (category.title == item.title) {
        exists = true
      }
    });
    if (!exists) {
      article.categories.push(category)
      this.setState({
        article: article,
        searchCategories: [],
        inputCategory: ''
      })
    }
  }

  changeField = (name, value) => {
    let { article } = this.state
    article[name] = value
    this.setState({
      article: article
    })
  }

  attachmentChange = (files, type, index) => {
    console.log(files, type, index)
    let { attachments } = this.state
    if (type === 'remove') {
      let attachment = attachments[index]
      Api.attachmentDelete(attachment.id).then(res => {
        if (res.code === 0) {
          attachments.splice(index, 1)
          this.setState({
            attachments: attachments
          })
        } else {
          alert(res.msg)
        }
      })
    } else {
      files.forEach((item, i) => {
        let formData = new FormData()
        formData.append("upFile", item.file)
        Api.attachmentUpload(formData).then(res => {
          if (res.code === 0) {
            let attachment = res.data
            attachment.url = attachment.location
            attachments.push(attachment)
            this.setState({
              attachments: attachments
            })
          } else {
            alert(res.msg)
          }
        })
      });
    }
  }

  addToEditor = (e) => {
    let currentFile = this.state.attachments[e]
    let value = simple.value()
    //附加到最后
    value += "\n!["+currentFile.title+"]("+currentFile.url+")"
    simple.value(value)
  }

  render() {
    const { article, searchCategories, inputCategory, attachments } = this.state
    const {user} = this.props

    return (
      <Main user={user} title='发布文章'>
        <div className='create-container'>
          <List renderHeader='发布文章'>
            <InputItem
              placeholder="填写文章标题"
              type='text'
              onChange={this.changeField.bind(this, 'title')}
              name='title'
              value={article.title || ''}
            >标题</InputItem>
            <InputItem
              placeholder="填写文章SEO标题"
              type='text'
              onChange={this.changeField.bind(this, 'seoTitle')}
              name='seoTitle'
              value={article.seoTitle || ''}
            >SEO标题</InputItem>
            <InputItem
              placeholder="填写文章关键词"
              type='text'
              onChange={this.changeField.bind(this, 'keywords')}
              name='keywords'
              value={article.keywords || ''}
            >关键词</InputItem>
            <List.Item>SEO描述</List.Item>
            <TextareaItem
              placeholder="填写SEO描述"
              rows='2'
              autoHeight
              onChange={this.changeField.bind(this, 'description')}
              name='description'
              value={article.description || ''}
            ></TextareaItem>
            <List.Item>文章内容</List.Item>
            <div className='article-editor'>
              <textarea name='message' id='article-editor' value={article.message || ''}
                onChange={this.changeField.bind(this, 'message')} />
            </div>
          </List>
          <List.Item>
            文章分类
          {article.categories.map((item, index) => {
              return (
                <Tag closable
                  key={index}
                  onClose={this.removeCategory.bind(this, index)}
                  className='category-tag'
                >
                  {item.title}
                </Tag>)
            })}
          </List.Item>
          <SearchBar
            value={inputCategory}
            placeholder="输入分类名称"
            onSubmit={this.checkAddCategory}
            onChange={this.changeInputCategory}
          />
          {searchCategories.length > 0 &&
            <div className='category-dropdown' id='category-dropdown'>
              {searchCategories.map((item, index) => {
                return (
                  <div className='dropdown-item' key={index} onClick={this.addCategory.bind(this, index)}>{item.title}</div>
                )
              })}
            </div>
          }

          <List.Item>上传图片
            <ImagePicker
              length="4"
              files={attachments}
              onChange={this.attachmentChange}
              selectable={attachments.length < 10}
              onImageClick={this.addToEditor}
              multiple
            />
          </List.Item>

          <Button onClick={this.submitArticle}>提交</Button>
        </div>
      </Main>
    )
  }
}

export default CreatePage