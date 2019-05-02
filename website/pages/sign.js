import React from 'react'
import Main from '../layouts/main'
import Api from '../utils/api'
import Link from 'next/link'
import Router from 'next/router'
import { InputItem, WhiteSpace, Button } from 'antd-mobile';
import qs from 'qs';
import './styles/sign.css'

class SignPage extends React.Component {
  static getInitialProps({ query: { type } }) {
    return { type: type }
  }

  state = {
    userData: {}
  }

  componentDidMount() {
    let { type } = this.props

    if(type === 'out'){
      //注销本地cookie
      localStorage.removeItem('token')
    }
  }

  handleChange = (name, value) => {
    let { userData } = this.state
    userData[name] = value
    this.setState({
      userData: userData
    })
  }

  handleSubmit = (e) => {
    let { userData } = this.state
    let { type } = this.props
    userData = qs.stringify(userData);
    if(type === 'in'){
      Api.signIn(userData).then(res => {
        if (res.code === 0) {
          localStorage.setItem('token', res.data.token)
          localStorage.setItem('user', JSON.stringify(res.data.user))
          alert('登录成功')
          Router.push('/')
        } else {
          alert(res.msg)
        }
      })
    }else{
      Api.signUp(userData).then(res => {
        if(res.code === 0){
          localStorage.setItem('token', res.data.token)
          localStorage.setItem('user', JSON.stringify(res.data.user))
          alert('注册成功，并且已经为您登录')
          Router.push('/');
        }else{
          alert(res.msg)
        }
      })
    }
    
  }

  render() {
    const { type } = this.props
    return (
      <Main title='注册登录'>
        <div className='sign-container'>
          {type === 'in' &&
            <form>
              <div className='sign-header'>
                <h1 className='sign-title'>登录账号</h1>
                <Link href='/sign/up'><a className='sign-tips'>我没有账号，点击注册</a></Link>
              </div>
              <div className='fields-list'>
                <InputItem
                  placeholder="请输入用户名"
                  type='text'
                  onChange={this.handleChange.bind(this, 'userName')}
                  name='userName'
                >用户名</InputItem>
                <WhiteSpace size='xl' />
                <InputItem
                  placeholder="请输入密码"
                  type='password'
                  onChange={this.handleChange.bind(this, 'password')}
                  name='password'
                >密码</InputItem>
                <WhiteSpace size='xl' />
                <Button onClick={this.handleSubmit}>提交</Button>
              </div>
            </form>
          }
          {type === 'up' &&
            <form>
              <div className='sign-header'>
                <h1 className='sign-title'>注册账号</h1>
                <Link href='/sign/in'><a className='sign-tips'>我有账号，点击登录</a></Link>
              </div>
              <div className='fields-list'>
                <InputItem
                  placeholder="请输入用户名"
                  type='text'
                  onChange={this.handleChange.bind(this, 'userName')}
                  name='userName'
                >用户名</InputItem>
                <WhiteSpace size='xl' />
                <InputItem
                  placeholder="请输入密码"
                  type='password'
                  onChange={this.handleChange.bind(this, 'password')}
                  name='password'
                >密码</InputItem>
                <WhiteSpace size='xl' />
                <Button onClick={this.handleSubmit}>提交</Button>
              </div>
            </form>
          }
          {type === 'out' &&
            <form>
              <div className='sign-header'>
                <h1 className='sign-title'>您已退出登录</h1>
                <Link href='/'><a className='sign-tips'>点我返回首页</a></Link>
              </div>
            </form>
          }
        </div>
      </Main>
    )
  }
}

export default SignPage