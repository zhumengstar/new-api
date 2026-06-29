import path from 'path'
import { createRequire } from 'module'
import { fileURLToPath } from 'url'
import { defineConfig, loadEnv } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)
const workspaceNodeModules = path.resolve(__dirname, '../node_modules')
const semiUiDir = path.resolve(
  path.dirname(
    require.resolve('@douyinfe/semi-ui', {
      paths: [workspaceNodeModules],
    }),
  ),
  '../..',
)
const classicNodeModules = path.resolve(__dirname, 'node_modules')
const vchartNodeModules = path.resolve(
  path.dirname(
    require.resolve('@visactor/vchart/package.json', {
      paths: [classicNodeModules],
    }),
  ),
  'node_modules',
)

export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ['VITE_'] })
  const clientServerUrl =
    process.env.VITE_REACT_APP_SERVER_URL ||
    env.rawPublicVars.VITE_REACT_APP_SERVER_URL ||
    ''
  const proxyServerUrl =
    clientServerUrl ||
    'http://localhost:3000'
  const isProd = envMode === 'production'
  const devProxy = Object.fromEntries(
    (['/api', '/mj', '/pg'] as const).map((key) => [
      key,
      { target: proxyServerUrl, changeOrigin: true },
    ]),
  ) as Record<string, { target: string; changeOrigin: boolean }>

  return {
    plugins: [pluginReact()],
    source: {
      entry: {
        index: './src/index.jsx',
      },
      define: {
        'import.meta.env.VITE_REACT_APP_SERVER_URL': JSON.stringify(
          clientServerUrl,
        ),
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        react: path.resolve(workspaceNodeModules, 'react'),
        'react-dom': path.resolve(workspaceNodeModules, 'react-dom'),
        'react/jsx-runtime': path.resolve(
          workspaceNodeModules,
          'react/jsx-runtime.js',
        ),
        'react/jsx-dev-runtime': path.resolve(
          workspaceNodeModules,
          'react/jsx-dev-runtime.js',
        ),
        '@douyinfe/semi-ui': path.resolve(
          workspaceNodeModules,
          '@douyinfe/semi-ui',
        ),
        '@douyinfe/semi-ui/lib/es': path.resolve(
          workspaceNodeModules,
          '@douyinfe/semi-ui/lib/es',
        ),
        '@douyinfe/semi-ui/lib/cjs': path.resolve(
          workspaceNodeModules,
          '@douyinfe/semi-ui/lib/cjs',
        ),
        '@douyinfe/semi-ui/react19-adapter': path.resolve(
          workspaceNodeModules,
          '@douyinfe/semi-ui/lib/es/react19-adapter.js',
        ),
        '@douyinfe/semi-icons': path.resolve(
          workspaceNodeModules,
          '@douyinfe/semi-icons',
        ),
        '@douyinfe/semi-illustrations': path.resolve(
          workspaceNodeModules,
          '@douyinfe/semi-illustrations',
        ),
        '@douyinfe/semi-ui/dist/css/semi.css': path.resolve(
          semiUiDir,
          'dist/css/semi.css',
        ),
        'date-fns': path.resolve(
          workspaceNodeModules,
          '@douyinfe/semi-ui/node_modules/date-fns',
        ),
        '@visactor/vrender-core': path.resolve(
          vchartNodeModules,
          '@visactor/vrender-core',
        ),
        '@visactor/vrender-kits': path.resolve(
          vchartNodeModules,
          '@visactor/vrender-kits',
        ),
        '@visactor/vutils': path.resolve(vchartNodeModules, '@visactor/vutils'),
      },
    },
    html: {
      template: './index.html',
    },
    server: {
      host: '0.0.0.0',
      strictPort: true,
      proxy: devProxy,
    },
    output: {
      minify: isProd,
      target: 'web',
      distPath: {
        root: 'dist',
      },
    },
    performance: {
      removeConsole: isProd ? ['log'] : false,
      buildCache: {
        cacheDigest: [process.env.VITE_REACT_APP_VERSION],
      },
    },
    tools: {
      rspack: {
        module: {
          rules: [
            {
              test: /src[\\/].*\.js$/,
              type: 'javascript/auto',
              use: [
                {
                  loader: 'builtin:swc-loader',
                  options: {
                    jsc: {
                      parser: {
                        syntax: 'ecmascript',
                        jsx: true,
                      },
                      transform: {
                        react: {
                          runtime: 'automatic',
                          development: !isProd,
                          refresh: !isProd,
                        },
                      },
                    },
                  },
                },
              ],
            },
          ],
        },
      },
    },
  }
})
