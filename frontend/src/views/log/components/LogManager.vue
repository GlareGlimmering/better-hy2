<template>
  <div class="app-container">
    <el-card shadow="never" class="usage-card">
      <div class="usage-grid" v-loading="usageLoading">
        <div v-for="item in usageItems" :key="item.type" class="usage-item">
          <span>{{ typeLabel(item.type) }}</span>
          <strong v-if="!item.error">{{ item.human }}</strong>
          <el-text v-else type="danger">{{ item.error }}</el-text>
        </div>
        <div class="usage-item total">
          <span>{{ $t("log.totalUsage") }}</span>
          <strong>{{ usage.human }}</strong>
          <el-text v-if="!usage.complete" type="warning">
            {{ $t("log.partialUsage") }}
          </el-text>
        </div>
      </div>
    </el-card>

    <el-card shadow="never">
      <div class="toolbar">
        <el-radio-group v-model="logType" @change="handleTypeChange">
          <el-radio-button label="system">{{ $t("log.system") }}</el-radio-button>
          <el-radio-button label="hysteria2">{{ $t("log.hysteria2") }}</el-radio-button>
          <el-radio-button label="access">{{ $t("log.access") }}</el-radio-button>
        </el-radio-group>

        <div class="toolbar-actions">
          <span>{{ $t("log.autoRefresh") }}</span>
          <el-switch v-model="autoRefresh" @change="syncTimer" />
          <el-button @click="togglePaused">
            {{ paused ? $t("log.resume") : $t("log.pause") }}
          </el-button>
          <el-button :loading="loading" @click="refreshNow">{{ $t("common.refresh") }}</el-button>
          <span>{{ $t("log.autoScroll") }}</span>
          <el-switch v-model="autoScroll" />
          <el-button @click="handleExport">{{ $t("common.export") }}</el-button>
          <el-button type="danger" plain @click="confirmClear(logType)">
            {{ $t("log.clearCurrent") }}
          </el-button>
          <el-button type="danger" @click="confirmClear('all')">
            {{ $t("log.clearAll") }}
          </el-button>
        </div>
      </div>

      <el-alert
        v-if="!exists"
        :title="$t('log.notCreated')"
        type="info"
        :closable="false"
        show-icon
        class="log-alert"
      />
      <div ref="logViewport" class="log-viewport" v-loading="loading">
        <pre>{{ renderedLines }}</pre>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { clearLogApi, exportLogApi, logStreamApi, logUsageApi } from "@/api/log";
import { LogType, LogUsageSummaryVo, LogUsageVo } from "@/api/log/types";
import { useI18n } from "vue-i18n";

const props = withDefaults(defineProps<{ initialType?: LogType }>(), {
  initialType: "system",
});
const { t } = useI18n();
const emptyUsage = (): LogUsageSummaryVo => ({
  logs: [],
  bytes: 0,
  human: "0 B",
  complete: true,
});

const logType = ref<LogType>(props.initialType);
const lines = ref<string[]>([]);
const cursor = ref("");
const exists = ref(true);
const loading = ref(false);
const usageLoading = ref(false);
const autoRefresh = ref(true);
const paused = ref(false);
const autoScroll = ref(true);
const usage = ref<LogUsageSummaryVo>(emptyUsage());
const logViewport = ref<HTMLElement>();
let timer: number | undefined;
let requestInFlight = false;

const usageItems = computed<LogUsageVo[]>(() => usage.value.logs || []);
const renderedLines = computed(() => lines.value.join("\n"));
const typeLabel = (type: LogType) => t(`log.${type}`);

const scrollToBottom = async () => {
  if (!autoScroll.value) return;
  await nextTick();
  if (logViewport.value) logViewport.value.scrollTop = logViewport.value.scrollHeight;
};

const fetchLines = async (reset = false) => {
  if (requestInFlight) return;
  requestInFlight = true;
  loading.value = true;
  try {
    if (reset) {
      cursor.value = "";
      lines.value = [];
    }
    const { data } = await logStreamApi({
      type: logType.value,
      cursor: cursor.value || undefined,
      limit: 200,
    });
    exists.value = data.exists;
    if (data.reset) lines.value = [];
    if (data.lines?.length) lines.value = [...lines.value, ...data.lines].slice(-200);
    cursor.value = data.cursor || "";
    await scrollToBottom();
  } finally {
    loading.value = false;
    requestInFlight = false;
  }
};

const fetchUsage = async () => {
  usageLoading.value = true;
  try {
    const { data } = await logUsageApi();
    usage.value = data;
  } finally {
    usageLoading.value = false;
  }
};

const stopTimer = () => {
  if (timer !== undefined) {
    window.clearInterval(timer);
    timer = undefined;
  }
};

const syncTimer = () => {
  stopTimer();
  if (autoRefresh.value && !paused.value) {
    timer = window.setInterval(() => {
      void fetchLines(false).catch(() => undefined);
    }, 2000);
  }
};

const togglePaused = () => {
  paused.value = !paused.value;
  syncTimer();
};

const refreshNow = async () => {
  await fetchLines(false);
  await fetchUsage();
};

const handleTypeChange = async () => fetchLines(true);

const confirmClear = async (type: LogType | "all") => {
  const message = type === "all" ? t("log.clearAllConfirm") : t("log.clearConfirm");
  try {
    await ElMessageBox.confirm(message, t("common.confirm"), {
      type: "warning",
      confirmButtonText: t("common.confirm"),
      cancelButtonText: t("common.cancel"),
    });
  } catch {
    return;
  }
  const { data } = await clearLogApi(type);
  usage.value = data;
  await fetchLines(true);
  ElMessage.success(t("common.success"));
};

const handleExport = async () => {
  const option = { system: 0, hysteria2: 1, access: 2 }[logType.value];
  const response = await exportLogApi({ option });
  const blob = new Blob([response.data], { type: "application/octet-stream" });
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  const disposition = response.headers["content-disposition"] || "";
  anchor.download = disposition.split("attachment; filename=")[1] || `${logType.value}.log`;
  anchor.click();
  window.URL.revokeObjectURL(url);
};

onMounted(async () => {
  await Promise.allSettled([fetchLines(true), fetchUsage()]);
  syncTimer();
});
onActivated(syncTimer);
onDeactivated(stopTimer);
onBeforeUnmount(stopTimer);
</script>

<style lang="scss" scoped>
.usage-card { margin-bottom: 16px; }
.usage-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
.usage-item { display: flex; flex-direction: column; gap: 6px; padding: 12px; border: 1px solid var(--el-border-color-lighter); border-radius: 4px; }
.usage-item.total { background: var(--el-fill-color-light); }
.toolbar { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
.toolbar-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.log-alert { margin-bottom: 12px; }
.log-viewport { height: 560px; overflow: auto; padding: 12px; color: var(--el-text-color-primary); background: var(--el-fill-color-darker); border-radius: 4px; }
pre { min-height: 100%; margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; font-family: Consolas, "Liberation Mono", Menlo, monospace; font-size: 13px; line-height: 1.55; }
</style>
