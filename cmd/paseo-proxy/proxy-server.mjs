import http from 'node:http'
import https from 'node:https'
import { URL } from 'node:url'

const listenAddr = process.env.PASEO_LISTEN || '127.0.0.1:8787'
const proxyRaw = process.env.PASEO_PROXY || ''
const failThreshold = Number(process.env.PASEO_FAIL_THRESHOLD || '3')

const upstreamURLs = (process.env.PASEO_URLS || '')
  .split(',')
  .map((s) => s.trim())
  .filter(Boolean)
const upstreamKeys = (process.env.PASEO_KEYS || '')
  .split(',')
  .map((s) => s.trim())
  .filter(Boolean)

const [host, port] = listenAddr.split(':')
const listenPort = Number(port || '8787')

const agentOptions = {
  keepAlive: true,
  timeout: 60000,
}

const httpAgent = new http.Agent(agentOptions)
const httpsAgent = new https.Agent(agentOptions)
httpsAgent.options.rejectUnauthorized = false

if (upstreamURLs.length === 0 || upstreamKeys.length === 0) {
  throw new Error('PASEO_URLS and PASEO_KEYS are required')
}
if (upstreamKeys.length !== 1 && upstreamKeys.length !== upstreamURLs.length) {
  throw new Error('PASEO_KEYS must have 1 item or match PASEO_URLS count')
}

const upstreams = upstreamURLs.map((raw, index) => ({
  raw,
  url: new URL(raw),
  key: upstreamKeys.length === 1 ? upstreamKeys[0] : upstreamKeys[index],
  failCount: 0,
}))
let activeIndex = 0

const server = http.createServer((req, res) => {
  const upstream = selectUpstream()
  const target = new URL(req.url || '/', upstream.url)
  const isHttps = target.protocol === 'https:'
  const client = isHttps ? https : http
  const agent = isHttps ? httpsAgent : httpAgent

  const headers = { ...req.headers }
  headers.host = target.host
  headers.authorization = `Bearer ${upstream.key}`

  const upstreamReq = client.request(
    {
      protocol: target.protocol,
      hostname: target.hostname,
      port: target.port || (isHttps ? 443 : 80),
      method: req.method,
      path: target.pathname + target.search,
      headers,
      agent,
      timeout: 60000,
    },
    (upstreamRes) => {
      res.writeHead(upstreamRes.statusCode || 502, upstreamRes.headers)
      upstreamRes.pipe(res)
    }
  )

  upstreamReq.on('timeout', () => upstreamReq.destroy(new Error('upstream timeout')))
  upstreamReq.on('error', (err) => {
    recordFailure(upstream)
    res.writeHead(502, { 'content-type': 'text/plain; charset=utf-8' })
    res.end(`upstream error: ${err.message}`)
  })

  upstreamReq.on('response', (upstreamRes) => {
    if (upstreamRes.statusCode && upstreamRes.statusCode >= 500) {
      recordFailure(upstream)
    } else {
      recordSuccess(upstream)
    }
  })

  req.pipe(upstreamReq)
})

server.listen(listenPort, host, () => {
  console.log(
    `paseo proxy listening on http://${host}:${listenPort} -> ${upstreams
      .map((u) => u.url.href)
      .join(', ')}`
  )
  if (proxyRaw) console.log(`proxy env: ${proxyRaw}`)
})

function selectUpstream() {
  for (let i = 0; i < upstreams.length; i++) {
    const idx = (activeIndex + i) % upstreams.length
    if (upstreams[idx].failCount < failThreshold) {
      activeIndex = idx
      return upstreams[idx]
    }
  }
  activeIndex = (activeIndex + 1) % upstreams.length
  return upstreams[activeIndex]
}

function recordFailure(upstream) {
  upstream.failCount += 1
  if (upstream.failCount >= failThreshold) {
    upstream.failCount = 0
    activeIndex = (activeIndex + 1) % upstreams.length
  }
}

function recordSuccess(upstream) {
  upstream.failCount = 0
}
