<template>
  <v-container fluid class="pa-8">
    <div class="d-flex justify-space-between align-center mb-6">
      <div>
        <h2 class="text-h4 font-weight-bold">Active Sites</h2>
        <div class="text-subtitle-1 text-grey">Managing active proxy configurations</div>
      </div>
      <v-btn
        prepend-icon="mdi-plus"
        color="primary"
        size="large"
        to="/simple"
      >
        New Site
      </v-btn>
    </div>

    <v-card border flat>
      <v-tabs v-model="tab" color="primary">
        <v-tab value="active">Active Sites</v-tab>
        <v-tab value="archived">Archived Sites</v-tab>
      </v-tabs>
      <v-divider></v-divider>

      <v-text-field
        v-model="search"
        prepend-inner-icon="mdi-magnify"
        label="Search Sites"
        single-line
        hide-details
        density="compact"
        class="pa-4"
        style="max-width: 400px"
      ></v-text-field>

      <v-data-table
        :headers="headers"
        :items="filteredSites"
        :loading="loading"
        :search="search"
        :custom-filter="customFilter"
        hover
      >
        <template v-slot:item.url="{ item }">
          <a v-if="item.url !== 'N/A'" :href="item.url" target="_blank" class="text-caption text-primary text-decoration-none">
            {{ item.url }}
            <v-icon size="x-small" icon="mdi-open-in-new" class="ml-1"></v-icon>
          </a>
          <span v-else class="text-caption text-grey">N/A</span>
        </template>

        <template v-slot:item.isEnabled="{ item }">
          <v-switch
            v-model="item.isEnabled"
            hide-details
            density="compact"
            color="success"
            :disabled="item.name === 'nginx.conf' || item.isArchived || loading"
            @change="toggleSite(item)"
          ></v-switch>
        </template>

        <template v-slot:item.hasSsl="{ item }">
          <v-icon
            :color="item.hasSsl ? 'success' : 'grey-lighten-1'"
            :icon="item.hasSsl ? 'mdi-lock' : 'mdi-lock-open-outline'"
            size="small"
            :title="item.hasSsl ? 'SSL Enabled' : 'No SSL'"
          ></v-icon>
        </template>

        <template v-slot:item.isActive="{ item }">
          <v-chip
            v-if="item.isArchived"
            color="warning"
            size="small"
            variant="flat"
            border
          >
            Archived
          </v-chip>
          <v-chip
            v-else
            :color="item.isActive === true ? 'success' : (item.isActive === false ? 'error' : 'grey')"
            size="small"
            variant="flat"
            border
          >
            {{ item.isActive === true ? 'Active' : (item.isActive === false ? 'Offline' : 'Unknown') }}
          </v-chip>
        </template>

        <template v-slot:item.actions="{ item }">
          <div class="d-flex justify-end">
             <v-btn
              v-if="!item.isArchived"
              icon="mdi-archive"
              size="small"
              variant="text"
              color="warning"
              @click="archiveSite(item)"
              title="Archive Site"
              :disabled="item.name === 'nginx.conf'"
            ></v-btn>
             <v-btn
              v-if="item.isArchived"
              icon="mdi-restore"
              size="small"
              variant="text"
              color="success"
              @click="restoreSite(item)"
              title="Restore Site"
            ></v-btn>
             <v-btn
              icon="mdi-pencil"
              size="small"
              variant="text"
              color="primary"
              :to="'/advanced?site=' + item.name"
              title="Edit Configuration"
            ></v-btn>
          </div>
        </template>

        <template v-slot:no-data>
          <div class="pa-8 text-center">
            <v-icon size="64" color="grey" class="mb-4">mdi-folder-open</v-icon>
            <div class="text-h6 text-grey">No sites found</div>
            <p class="text-caption text-grey mb-4">Add a new proxy to get started</p>
            <v-btn variant="tonal" color="primary" to="/simple">
              Create First Site
            </v-btn>
          </div>
        </template>
      </v-data-table>
    </v-card>
  </v-container>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import axios from 'axios'

const sites = ref([])
const loading = ref(true)
const search = ref('')
const tab = ref('active')
let pollInterval = null

const filteredSites = computed(() => {
  if (tab.value === 'active') {
    return sites.value.filter(s => !s.isArchived)
  }
  return sites.value.filter(s => s.isArchived)
})

const headers = [
  { title: 'Site Name', key: 'name', align: 'start' },
  { title: 'URL', key: 'url', align: 'start' },
  { title: 'Enabled', key: 'isEnabled', align: 'center', width: '100px' },
  { title: 'SSL', key: 'hasSsl', align: 'center', width: '80px' },
  { title: 'Status', key: 'isActive', align: 'start', width: '120px' },
  { title: 'Actions', key: 'actions', align: 'end', sortable: false },
]

const fetchSites = async () => {
  try {
    const res = await axios.get('/api/sites')
    sites.value = res.data.sites || []
  } catch (err) {
    console.error(err)
    // If backend is unreachable, mark existing sites as unknown status
    sites.value.forEach(site => site.isActive = null)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchSites()
  // Poll every 5 seconds
  pollInterval = setInterval(fetchSites, 5000)
})

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval)
})

const toggleSite = async (item) => {
  try {
    await axios.post(`/api/sites/${item.name}/toggle`, { enabled: item.isEnabled })
    // Refresh to update status chips
    fetchSites()
  } catch (err) {
    console.error(err)
    // Revert if failed
    item.isEnabled = !item.isEnabled
  }
}

const archiveSite = async (item) => {
  if (!confirm(`Are you sure you want to archive ${item.name}? This will disable the site.`)) return
  try {
    await axios.post(`/api/sites/${item.name}/archive`)
    fetchSites()
  } catch (err) {
    console.error(err)
    alert('Failed to archive site')
  }
}

const restoreSite = async (item) => {
  try {
    await axios.post(`/api/sites/${item.name}/restore`)
    fetchSites()
  } catch (err) {
    console.error(err)
    alert('Failed to restore site')
  }
}

const customFilter = (value, query, item) => {
  if (!query) return true
  const q = query.toLowerCase()
  const name = (item.raw.name || '').toLowerCase()
  const url = (item.raw.url || '').toLowerCase()
  return name.includes(q) || url.includes(q)
}
</script>

<style scoped>
.font-mono {
  font-family: monospace;
}
</style>
