import { useStorage } from '@vueuse/core'
import axios from 'axios'
import NProgress from 'nprogress'
import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { menuRoutes } from './menu'
import 'nprogress/nprogress.css'

NProgress.configure({ showSpinner: false })

const adminToken = useStorage('admin_token', '')
let validatedToken = ''
let validatingPromise: Promise<boolean> | null = null

async function ensureTokenValid() {
  const token = String(adminToken.value || '').trim()
  if (!token)
    return false

  if (validatedToken && validatedToken === token)
    return true

  if (validatingPromise)
    return validatingPromise

  validatingPromise = axios.get('/api/auth/validate', {
    headers: { 'x-admin-token': token },
    timeout: 6000,
  }).then(async (res) => {
    const ok = !!(res.data && res.data.ok)
    if (ok) {
      validatedToken = token
      const userStore = useUserStore()
      await userStore.fetchUserInfo()
    }
    return ok
  }).catch(() => false).finally(() => {
    validatingPromise = null
  })

  return validatingPromise
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      component: () => import('@/components/wangui/UserLayout.vue'),
      children: menuRoutes.filter(route => !route.adminOnly).map(route => ({
        path: route.path,
        name: route.name,
        component: route.component,
      })),
    },
    {
      path: '/admin',
      component: () => import('@/components/wangui/AdminLayout.vue'),
      children: [
        {
          path: '',
          name: 'admin',
          component: () => import('@/views/AdminPanel.vue'),
        },
      ],
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/Login.vue'),
    },
    {
      path: '/renewal',
      name: 'renewal',
      component: () => import('@/views/Renewal.vue'),
    },
  ],
})

router.beforeEach(async (to) => {
  NProgress.start()

  if (to.name === 'renewal') {
    if (!adminToken.value) {
      validatedToken = ''
      return true
    }
    const valid = await ensureTokenValid()
    if (!valid) {
      adminToken.value = ''
      validatedToken = ''
    }
    return true
  }

  if (to.name === 'login') {
    if (!adminToken.value) {
      validatedToken = ''
      return true
    }
    const valid = await ensureTokenValid()
    if (valid)
      return { name: 'dashboard' }
    adminToken.value = ''
    validatedToken = ''
    return true
  }

  if (!adminToken.value) {
    validatedToken = ''
    return { name: 'login' }
  }

  const valid = await ensureTokenValid()
  if (!valid) {
    adminToken.value = ''
    validatedToken = ''
    return { name: 'login' }
  }

  const userStore = useUserStore()
  if (to.path.startsWith('/admin') && !userStore.isAdmin) {
    return { name: 'dashboard' }
  }

  return true
})

router.afterEach(() => {
  NProgress.done()
})

export default router
