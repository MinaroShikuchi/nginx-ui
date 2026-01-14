<template>
  <v-app>
    <v-navigation-drawer v-model="drawer" permanent>
      <v-list-item
        prepend-icon="mdi-nginx"
        title="Nginx Manager"
        subtitle="Native Edition"
        class="py-4"
      ></v-list-item>

      <v-divider></v-divider>

      <v-list density="compact" nav>
        <v-list-item
          prepend-icon="mdi-view-dashboard"
          title="Dashboard"
          to="/"
          exact
        ></v-list-item>
      </v-list>
    </v-navigation-drawer>

    <v-app-bar flat border>
      <v-app-bar-title>Infrastructure Overview</v-app-bar-title>
      <v-spacer></v-spacer>
      <v-chip
        :color="systemOnline ? 'success' : 'error'"
        size="small"
        variant="flat"
        class="mr-4"
      >
        {{ systemOnline ? 'System Online' : 'System Offline' }}
      </v-chip>
    </v-app-bar>

    <v-main class="bg-grey-darken-4">
      <router-view v-slot="{ Component }">
        <v-fade-transition mode="out-in">
          <component :is="Component" />
        </v-fade-transition>
      </router-view>
    </v-main>
  </v-app>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import axios from 'axios'

const drawer = ref(true)
const systemOnline = ref(false)
let healthPoll = null

const checkHealth = async () => {
  try {
    await axios.get('/api/health')
    systemOnline.value = true
  } catch (e) {
    systemOnline.value = false
  }
}

onMounted(() => {
  checkHealth()
  healthPoll = setInterval(checkHealth, 5000)
})

onUnmounted(() => {
  if (healthPoll) clearInterval(healthPoll)
})
</script>

<style>
/* Remove custom styles as Vuetify handles them */
</style>
