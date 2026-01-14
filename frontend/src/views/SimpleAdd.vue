<template>
  <v-container class="fill-height justify-center pa-8">
    <v-card width="100%" max-width="500" class="pa-8 py-10" border flat>
      <div class="text-center mb-8">
        <v-avatar color="primary-darken-1" size="64" class="mb-4">
          <v-icon size="32" color="white">mdi-plus</v-icon>
        </v-avatar>
        <h2 class="text-h5 font-weight-bold">Add New Site</h2>
        <div class="text-body-2 text-grey">Define a domain and its upstream port</div>
      </div>

      <v-form v-model="formValid" @submit.prevent="submit">
        <v-text-field
          v-model="form.domain"
          label="Public Domain"
          placeholder="blog.example.com"
          persistent-placeholder
          variant="outlined"
          prepend-inner-icon="mdi-web"
          :rules="[v => !!v || 'Public domain is required']"
          class="mb-4"
        ></v-text-field>

        <div class="text-subtitle-2 mb-2 text-grey">Forward To (Upstream)</div>
        <v-row dense class="mb-4">
          <v-col cols="3">
            <v-select
              v-model="form.protocol"
              :items="['http', 'https']"
              label="Proto"
              variant="outlined"
              density="comfortable"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="6">
            <v-text-field
              v-model="form.hostname"
              label="Hostname / IP"
              placeholder="127.0.0.1"
              variant="outlined"
              density="comfortable"
              prepend-inner-icon="mdi-server"
              :rules="[v => !!v || 'Required']"
              hide-details
            ></v-text-field>
          </v-col>
          <v-col cols="3">
            <v-text-field
              v-model.number="form.port"
              label="Port"
              placeholder="3000"
              variant="outlined"
              type="number"
              density="comfortable"
              :rules="[v => !!v || 'Req', v => v > 0 || 'Err']"
              hide-details
            ></v-text-field>
          </v-col>
        </v-row>

        <v-btn
          block
          color="primary"
          size="large"
          type="submit"
          :loading="loading"
          :disabled="!formValid"
        >
          Create Proxy Configuration
        </v-btn>
      </v-form>

      <v-alert
        v-if="message"
        :type="error ? 'error' : 'success'"
        variant="tonal"
        class="mt-6"
        closable
        @click:close="message = ''"
      >
        {{ message }}
      </v-alert>
    </v-card>
  </v-container>
</template>

<script setup>
import { ref } from 'vue'
import axios from 'axios'
import { useRouter } from 'vue-router'

const router = useRouter()
const form = ref({ 
  domain: '', 
  protocol: 'http', 
  hostname: '127.0.0.1', 
  port: null 
})
const formValid = ref(false)
const loading = ref(false)
const message = ref('')
const error = ref(false)

const submit = async () => {
  if (!formValid.value) return
  
  loading.value = true
  message.value = ''
  error.value = false
  
  try {
    await axios.post('/api/apps', form.value)
    message.value = 'Configuration generated and deployed!'
    setTimeout(() => router.push('/'), 1500)
  } catch (err) {
    error.value = true
    message.value = err.response?.data?.error || 'System error during deployment'
  } finally {
    loading.value = false
  }
}
</script>
