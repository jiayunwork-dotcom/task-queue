// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  devtools: { enabled: true },
  modules: ['@nuxt/ui'],
  ssr: false,
  app: {
    head: {
      title: 'Task Queue Admin',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' }
      ],
      link: [
        { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' }
      ]
    }
  },
  runtimeConfig: {
    public: {
      apiBase: process.env.API_BASE_URL || 'http://localhost:8080/api/v1'
    }
  },
  colorMode: {
    preference: 'light'
  },
  ui: {
    primary: 'blue',
    gray: 'slate'
  },
  nitro: {
    routeRules: {
      '/api/**': { proxy: { to: process.env.API_BASE_URL || 'http://localhost:8080/api/v1' } }
    }
  }
})
