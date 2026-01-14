import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import SimpleAdd from '../views/SimpleAdd.vue'
import AdvancedEdit from '../views/AdvancedEdit.vue'

const routes = [
  { path: '/', component: Dashboard },
  { path: '/simple', component: SimpleAdd },
  { path: '/advanced', component: AdvancedEdit },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
