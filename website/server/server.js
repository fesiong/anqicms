const express = require('express')
const next = require('next')
const compression = require('compression')
const LRUCache = require('lru-cache')

const port = parseInt(process.env.PORT, 10) || 3001
const dev = process.env.NODE_ENV !== 'production'
const app = next({ dev })
const handle = app.getRequestHandler()

// 缓存设置
const ssrCache = new LRUCache({
  max: 100,
  maxAge: 1000 * 60 * 60 // 1hour
})

app.prepare()
  .then(() => {
    const server = express()
    if (!dev) {
      server.use(compression()) //gzip
    }

    server.get('/', (req, res) => {
      renderAndCache(req, res, '/')
    })

    server.get('/category/:id', (req, res) => {
      req.query.id = req.params.id
      return app.render(req, res, '/category', req.query)
    })

    server.get('/article/:id', (req, res) => {
      const queryParams = { id: req.params.id }
      return app.render(req, res, '/article', queryParams)
    })

    server.get('/create/:id', (req, res) => {
      const queryParams = { id: req.params.id }
      return app.render(req, res, '/create', queryParams)
    })

    server.get('/about', (req, res) => {
      return app.render(req, res, '/about')
    })

    server.get('/sign/in', (req, res) => {
      return app.render(req, res, '/sign', {type: 'in'})
    })

    server.get('/sign/up', (req, res) => {
      return app.render(req, res, '/sign', {type: 'up'})
    })

    server.get('/sign/out', (req, res) => {
      return app.render(req, res, '/sign', {type: 'out'})
    })

    server.get('*', (req, res) => {
      res.setHeader('Access-Control-Allow-Origin', '*')
      return handle(req, res)
    })

    server.listen(port, (err) => {
      if (err) throw err
      console.log(`> Ready on http://localhost:${port}`)
    })
  })
function getCacheKey(req) {
  return `${req.url}`
}
function renderAndCache(req, res, pagePath, queryParams) {
  const key = getCacheKey(req)
  // 存在缓存
  if (ssrCache.has(key)) {
    console.log(`CACHE HIT: ${key}`)
    res.send(ssrCache.get(key))
    return
  }
  // 无缓存，重新渲染
  app.renderToHTML(req, res, pagePath, queryParams)
    .then((html) => {
      // 缓存页面
      console.log(`CACHE MISS: ${key}`)
      ssrCache.set(key, html)
      res.send(html)
    })
    .catch((err) => {
      app.renderError(err, req, res, pagePath, queryParams)
    })
}