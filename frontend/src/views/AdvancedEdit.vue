<template>
  <v-container fluid class="pa-8 d-flex flex-column">
    <div class="d-flex justify-space-between align-center mb-6">
      <div>
        <h2 class="text-h4 font-weight-bold">Raw Configuration</h2>
        <div class="text-subtitle-1 text-grey">Directly edit Nginx configuration files</div>
      </div>
      <div class="d-flex gap-x-4">
        <v-text-field
          v-model="filename"
          label="Filename"
          placeholder="filename.conf"
          variant="outlined"
          density="compact"
          hide-details
          style="width: 280px"
          class="mr-4"
        >
          <template v-slot:prepend-inner>
            <v-icon size="small">mdi-file-document-outline</v-icon>
          </template>
        </v-text-field>
        <v-btn
          color="primary"
          height="40"
          :loading="loading"
          @click="save"
        >
          Save & Reload
        </v-btn>
      </div>
    </div>

    <v-card border flat class="flex-grow-1 d-flex flex-column overflow-hidden rounded-lg">
      <div class="px-4 py-2 bg-grey-darken-3 text-caption font-mono d-flex align-center">
        <v-icon size="x-small" color="primary" class="mr-2">mdi-circle-medium</v-icon>
        {{ filename || 'new-config.conf' }}
      </div>
      <v-textarea
        v-model="content"
        auto-grow
        variant="plain"
        class="font-mono px-4"
        hide-details
        spellcheck="false"
        placeholder="# Custom Nginx config here...
server {
    listen 8080;
    server_name example.local;

    location / {
        proxy_pass http://localhost:3000;
    }
}"
      ></v-textarea>
    </v-card>

    <v-snackbar
      v-model="showSnackbar"
      :color="error ? 'error' : 'success'"
      timeout="3000"
    >
      {{ message }}
    </v-snackbar>
  </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { useRoute } from 'vue-router'

const route = useRoute()
const filename = ref('')
const content = ref('')
const loading = ref(false)
const showSnackbar = ref(false)
const message = ref('')
const error = ref(false)

onMounted(async () => {
  if (route.query.site) {
    filename.value = route.query.site
    try {
      loading.value = true
      const res = await axios.get(`/api/sites/${route.query.site}`)
      content.value = res.data.content
    } catch (err) {
      console.error('Failed to fetch config:', err)
      error.value = true
      message.value = "Failed to load configuration content"
      showSnackbar.value = true
    } finally {
      loading.value = false
    }
  }
})

const save = async () => {
  if (!filename.value) {
    error.value = true
    message.value = "Filename is required"
    showSnackbar.value = true
    return
  }

  loading.value = true
  message.value = ''
  error.value = false

  const name = filename.value.endsWith('.conf') ? filename.value : filename.value + '.conf'

  try {
    await axios.post('/api/sites', {
      name: name,
      content: content.value
    })
    message.value = "Configuration deployed successfully"
    error.value = false
  } catch (err) {
    error.value = true
    message.value = err.response?.data?.error || "Syntax error or system failure"
  } finally {
    loading.value = false
    showSnackbar.value = true
  }
}
</script>

<style scoped>
.font-mono {
  font-family: 'Fira Code', 'Courier New', monospace !important;
  font-size: 13px !important;
}
</style>
