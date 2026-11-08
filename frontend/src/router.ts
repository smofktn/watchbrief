import { createRouter, createWebHistory, type RouterHistory } from 'vue-router'
import LoginPage from './pages/LoginPage.vue'
import MePage from './pages/MePage.vue'
import CollectionsPage from './pages/CollectionsPage.vue'
import CollectionDetailPage from './pages/CollectionDetailPage.vue'
import ReportsPage from './pages/ReportsPage.vue'
import ReportDetailPage from './pages/ReportDetailPage.vue'
import NotFoundPage from './pages/NotFoundPage.vue'
export const createAppRouter = (history: RouterHistory = createWebHistory()) => createRouter({
  history,
  strict: true,
  sensitive: true,
  stringifyQuery: (query) => {
    const parameters = new URLSearchParams()
    for (const [key, raw] of Object.entries(query ?? {})) {
      for (const value of Array.isArray(raw) ? raw : [raw]) {
        if (value !== undefined) {
          parameters.append(key, value === null ? '' : String(value))
        }
      }
    }
    return parameters.toString()
  },
  routes: [
    { path: '/', component: MePage },
    { path: '/login', component: LoginPage },
    { path: '/me', component: MePage },
    { path: '/collections', component: CollectionsPage },
    { path: '/collections/:id([0-7][0-9A-HJKMNP-TV-Z]{25})', component: CollectionDetailPage },
    { path: '/reports', component: ReportsPage },
    { path: '/reports/:id([0-7][0-9A-HJKMNP-TV-Z]{25})', component: ReportDetailPage },
    { path: '/:pathMatch(.*)*', component: NotFoundPage },
  ],
})
