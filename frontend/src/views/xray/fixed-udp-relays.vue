<template>
  <el-card shadow="never" class="hui-panel">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h2 class="hui-section-title">{{ $t("xray.relayTitle") }}</h2>
        <el-button type="primary" :disabled="!inventory?.supported || loading || saving" @click="openEditor()">
          {{ $t("xray.relayAdd") }}
        </el-button>
      </div>
    </template>
    <el-alert v-if="loadFailed" type="error" :closable="false" :title="$t('xray.relayLoadFailed')">
      <el-button link type="primary" @click="load">{{ $t("common.refresh") }}</el-button>
    </el-alert>
    <el-alert v-else-if="inventory && !inventory.supported" type="info" :closable="false" :title="$t('xray.relayUnsupported')" />
    <el-alert v-if="!visible && saveError" type="error" :closable="false" :title="saveError" />
    <el-table v-loading="loading" :data="inventory?.relays ?? []" class="hui-table">
      <el-table-column prop="name" :label="$t('xray.name')" min-width="120" show-overflow-tooltip />
      <el-table-column prop="addr" :label="$t('hysteria.relayEndpoint')" min-width="160" show-overflow-tooltip />
      <el-table-column :label="$t('common.operate')" min-width="136">
        <template #default="{ row }">
          <el-button link type="primary" :disabled="!inventory?.supported || loading || saving" @click="openEditor(row)">
            {{ $t("common.update") }}
          </el-button>
          <el-button link type="danger" :disabled="loading || saving" @click="removeRelay(row)">
            {{ $t("common.delete") }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-drawer v-model="visible" :title="$t(previousName ? 'xray.editOutbound' : 'xray.relayAdd')" :size="drawerSize"
    class="hui-editor-drawer" append-to-body :close-on-click-modal="false" :close-on-press-escape="!saving" :show-close="!saving">
    <el-form label-position="top" :disabled="saving" @submit.prevent="save">
      <p class="hui-section-description mb-4">{{ $t("xray.relayHelp") }}</p>
      <el-form-item :label="$t('xray.name')" required>
        <el-input v-model="draft.name" maxlength="64" />
      </el-form-item>
      <el-form-item :label="$t('hysteria.relayEndpoint')" required>
        <el-input v-model="draft.addr" placeholder="192.0.2.10:51820" />
      </el-form-item>
      <el-alert v-if="saveError" type="error" :closable="false" :title="saveError" />
    </el-form>
    <template #footer>
      <el-button :disabled="saving" @click="visible = false">{{ $t("common.cancel") }}</el-button>
      <el-button type="primary" :loading="saving" :disabled="saving || !draft.name.trim() || !draft.addr.trim()" @click="save">
        {{ $t("common.save") }}
      </el-button>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { ElMessage, ElMessageBox } from "element-plus";
import { useI18n } from "vue-i18n";
import {
  addFixedUDPRelayApi,
  deleteFixedUDPRelayApi,
  listFixedUDPRelaysApi,
  updateFixedUDPRelayApi,
  waitForHysteria2Job,
  type FixedUDPRelay,
  type Hysteria2ApplyResult,
} from "@/api/config";

defineProps<{ drawerSize: string }>();

const { t } = useI18n();
const previousName = ref("");
const inventory = ref<{ supported: boolean; relays: FixedUDPRelay[] }>();
const loading = ref(false);
const loadFailed = ref(false);
const visible = ref(false);
const saving = ref(false);
const saveError = ref("");
const draft = reactive<FixedUDPRelay>({ name: "", addr: "" });
let pageClosed = false;

onBeforeUnmount(() => { pageClosed = true; });

const load = async () => {
  loading.value = true;
  loadFailed.value = false;
  try {
    ({ data: inventory.value } = await listFixedUDPRelaysApi());
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
};

const openEditor = (relay?: FixedUDPRelay) => {
  previousName.value = relay?.name ?? "";
  Object.assign(draft, relay ?? { name: "", addr: "" });
  saveError.value = "";
  visible.value = true;
};

const save = async () => {
  const relay = { name: draft.name.trim(), addr: draft.addr.trim() };
  if (saving.value || !inventory.value?.supported || !relay.name || !relay.addr) return;
  if (!/^(?:\d{1,3}\.){3}\d{1,3}:[1-9]\d{0,4}$/.test(relay.addr)) {
    saveError.value = t("xray.relayEndpointRequired");
    return;
  }
  await applyRelayChange(() => previousName.value
    ? updateFixedUDPRelayApi(previousName.value, relay)
    : addFixedUDPRelayApi(relay));
};

const removeRelay = async (relay: FixedUDPRelay) => {
  try {
    await ElMessageBox.confirm(t("xray.deleteConfirm", { name: relay.name }), t("common.confirm"), { type: "warning" });
  } catch {
    return;
  }
  if (pageClosed || saving.value) return;
  await applyRelayChange(() => deleteFixedUDPRelayApi(relay.name));
};

const applyRelayChange = async (request: () => Promise<{ data: Hysteria2ApplyResult }>) => {
  saving.value = true;
  saveError.value = "";
  let jobStarted = false;
  try {
    const { data: job } = await request();
    jobStarted = true;
    const result = await waitForHysteria2Job(job, () => pageClosed);
    if (pageClosed) return;
    if (result.state === "failed") {
      saveError.value = result.error || t("hysteria.configNotConfirmed");
    } else if (result.state === "pending") {
      saveError.value = t("hysteria.configResultUnknown");
    } else {
      visible.value = false;
      ElMessage.success(t("hysteria.configApplied"));
      await load();
    }
  } catch (error) {
    if (pageClosed) return;
    saveError.value = jobStarted ? t("hysteria.configResultUnknown") :
      (error instanceof Error ? error.message : t("hysteria.configNotConfirmed"));
  } finally {
    saving.value = false;
  }
};

onMounted(load);
</script>
