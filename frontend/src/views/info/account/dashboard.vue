<template>
  <div class="hui-page dashboard-container">
    <header class="hui-page-heading">
      <div>
        <h1>{{ $t("dashboard.title") }}</h1>
        <p>{{ $t("dashboard.description") }}</p>
      </div>
      <div class="dashboard-actions">
        <small v-if="lastUpdated">{{ $t("dashboard.lastUpdated") }} {{ lastUpdated }}</small>
        <el-button :icon="Refresh" :loading="refreshing" @click="refreshAll">
          {{ $t("common.refresh") }}
        </el-button>
      </div>
    </header>

    <el-alert v-if="systemError" :title="systemError" type="warning" show-icon :closable="false" />
    <el-alert v-if="trafficError" :title="trafficError" type="warning" show-icon :closable="false" />

    <section v-loading="trafficLoading" class="traffic-grid" :aria-label="$t('dashboard.totalTraffic')">
      <div class="hui-surface traffic-metric traffic-metric-total">
        <div class="metric-top">
          <span>{{ $t("dashboard.totalTraffic") }}</span>
          <span class="metric-icon"><el-icon><DataAnalysis /></el-icon></span>
        </div>
        <strong>{{ formatTrafficG(trafficMonitor.total) }}</strong>
      </div>
      <div class="hui-surface traffic-metric">
        <div class="metric-top">
          <span>{{ $t("dashboard.totalUpload") }}</span>
          <span class="metric-icon"><el-icon><Upload /></el-icon></span>
        </div>
        <strong>{{ formatTrafficG(trafficMonitor.upload) }}</strong>
      </div>
      <div class="hui-surface traffic-metric">
        <div class="metric-top">
          <span>{{ $t("dashboard.totalDownload") }}</span>
          <span class="metric-icon"><el-icon><Download /></el-icon></span>
        </div>
        <strong>{{ formatTrafficG(trafficMonitor.download) }}</strong>
      </div>
    </section>

    <section v-loading="systemLoading" class="hui-surface service-strip" :aria-label="$t('monitor.hysteria2Running')">
      <div class="service-cell service-version">
        <div class="metric-top">
          <span>{{ $t("monitor.hysteria2Running") }}</span>
          <span v-if="hy2StatusKnown" class="hui-status" :class="hysteria2Monitor.running ? 'is-live' : 'is-offline'">
            {{ $t(hysteria2Monitor.running ? "monitor.hysteria2RunningTrue" : "monitor.hysteria2RunningFalse") }}
          </span>
          <span v-else>—</span>
        </div>
        <small>
          <span :title="kernelTooltip">HY2 {{ kernelDisplay.label }}</span>
          · Better HY2 {{ systemMonitor.huiVersion || "—" }}
        </small>
      </div>
      <div class="service-cell">
        <span>{{ $t("monitor.hysteria2UserTotal") }}</span>
        <strong>{{ hy2StatusKnown ? hysteria2Monitor.userTotal : "—" }}</strong>
      </div>
      <div class="service-cell">
        <span>{{ $t("monitor.hysteria2DeviceTotal") }}</span>
        <strong>{{ hy2StatusKnown ? hysteria2Monitor.deviceTotal : "—" }}</strong>
      </div>
    </section>

    <div class="dashboard-grid">
      <section class="hui-surface ranking-card">
        <header class="hui-surface-heading">
          <div>
            <h2>{{ $t("dashboard.trafficRanking") }}</h2>
            <p>{{ $t("dashboard.trafficRankingTip") }}</p>
          </div>
          <el-button link @click="viewAllUsers">
            {{ $t("dashboard.viewAllUsers") }} <el-icon class="el-icon--right"><ArrowRight /></el-icon>
          </el-button>
        </header>
        <el-table v-loading="trafficLoading" :data="trafficMonitor.topUsers" :empty-text="$t('dashboard.noTrafficData')" class="hui-table ranking-table">
          <el-table-column prop="username" :label="$t('account.username')" min-width="115" show-overflow-tooltip />
          <el-table-column prop="upload" :label="$t('account.upload')" min-width="74" align="right">
            <template #default="scope">{{ formatTrafficG(scope.row.upload) }}</template>
          </el-table-column>
          <el-table-column prop="download" :label="$t('account.download')" min-width="76" align="right">
            <template #default="scope">{{ formatTrafficG(scope.row.download) }}</template>
          </el-table-column>
          <el-table-column prop="total" :label="$t('dashboard.totalTraffic')" min-width="94" align="right">
            <template #default="scope"><strong>{{ formatTrafficG(scope.row.total) }}</strong></template>
          </el-table-column>
        </el-table>
      </section>

      <section v-loading="systemLoading" class="hui-surface resource-card">
        <header class="hui-surface-heading">
          <h2>{{ $t("dashboard.resources") }}</h2>
        </header>
        <div class="resource-list">
          <div v-for="resource in usage" :key="resource.label" class="resource-item">
            <div class="metric-top">
              <span>{{ $t(resource.label) }}</span>
              <strong>{{ formatPercent(resource.value) }}</strong>
            </div>
            <el-progress :percentage="normalizePercent(resource.value)" :show-text="false" :stroke-width="6" />
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ArrowRight, DataAnalysis, Download, Refresh, Upload } from "@element-plus/icons-vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import {
  monitorHysteria2Api,
  monitorSystemApi,
  monitorTrafficApi,
} from "@/api/monitor";
import {
  Hysteria2MonitorVo,
  SystemMonitorVo,
  TrafficMonitorVo,
} from "@/api/monitor/types";
import { useAccountStore } from "@/store/modules/account";
import { formatTrafficG } from "@/utils/account-quota";
import { describeHY2Kernel } from "@/utils/hy2-kernel";
import { timestampToCompactDateTime } from "@/utils/time";
import { createDashboardPolling } from "./polling";

const { t } = useI18n();
const router = useRouter();
const accountStore = useAccountStore();

const state = reactive({
  systemMonitor: {
    huiVersion: "",
    cpuPercent: 0,
    memPercent: 0,
    diskPercent: 0,
  } as SystemMonitorVo,
  hysteria2Monitor: {
    userTotal: 0,
    deviceTotal: 0,
    version: "",
    running: false,
  } as Hysteria2MonitorVo,
  hy2StatusKnown: false,
  trafficMonitor: {
    upload: 0,
    download: 0,
    total: 0,
    topUsers: [],
  } as TrafficMonitorVo,
  systemLoading: true,
  trafficLoading: true,
  refreshingSystem: false,
  refreshingTraffic: false,
  systemError: "",
  trafficError: "",
  lastUpdatedAt: 0,
});

const {
  systemMonitor,
  hysteria2Monitor,
  hy2StatusKnown,
  trafficMonitor,
  systemLoading,
  trafficLoading,
  systemError,
  trafficError,
} = toRefs(state);
const kernelDisplay = computed(() => describeHY2Kernel(hysteria2Monitor.value.version));
const kernelTooltip = computed(() => {
  const { upstream, channel } = kernelDisplay.value;
  if (!upstream) return undefined;
  const source = t("hysteria.kernelDerivedFrom", { upstream, build: hysteria2Monitor.value.version });
  return [channel, source].filter(Boolean).join(" · ");
});


let componentActive = true;

const refreshing = computed(
  () => state.refreshingSystem || state.refreshingTraffic
);
const lastUpdated = computed(() =>
  state.lastUpdatedAt
    ? timestampToCompactDateTime(state.lastUpdatedAt)
    : ""
);

const normalizePercent = (value: number) =>
  Math.min(100, Math.max(0, Number(value) || 0));
const formatPercent = (value: number) => `${normalizePercent(value).toFixed(1)}%`;

const usage = computed(() => [
  { label: "monitor.cpuPercent", value: state.systemMonitor.cpuPercent },
  { label: "monitor.memPercent", value: state.systemMonitor.memPercent },
  { label: "monitor.diskPercent", value: state.systemMonitor.diskPercent },
]);

const refreshSystem = async () => {
  if (state.refreshingSystem) return;
  state.refreshingSystem = true;
  const results = await Promise.allSettled([
    monitorSystemApi(),
    monitorHysteria2Api(),
  ]);
  if (componentActive) {
    const failures: string[] = [];
    if (results[0].status === "fulfilled") {
      Object.assign(state.systemMonitor, results[0].value.data);
    } else {
      failures.push(t("dashboard.systemLoadFailed"));
    }
    if (results[1].status === "fulfilled") {
      Object.assign(state.hysteria2Monitor, results[1].value.data);
      state.hy2StatusKnown = true;
    } else {
      state.hy2StatusKnown = false;
      failures.push(t("dashboard.hy2LoadFailed"));
    }
    state.systemError = failures.join(" · ");
    state.systemLoading = false;
    if (failures.length === 0) state.lastUpdatedAt = Date.now();
  }
  state.refreshingSystem = false;
};

const refreshTraffic = async () => {
  if (state.refreshingTraffic) return;
  state.refreshingTraffic = true;
  try {
    const { data } = await monitorTrafficApi();
    if (componentActive) {
      Object.assign(state.trafficMonitor, data);
      state.trafficError = "";
      state.lastUpdatedAt = Date.now();
    }
  } catch (_error) {
    if (componentActive) state.trafficError = t("dashboard.trafficLoadFailed");
  } finally {
    if (componentActive) state.trafficLoading = false;
    state.refreshingTraffic = false;
  }
};

const polling = createDashboardPolling([
  { interval: 8000, run: refreshSystem },
  { interval: 30000, run: refreshTraffic },
]);

const refreshAll = async () => {
  await Promise.all([refreshSystem(), refreshTraffic()]);
};

const handleVisibilityChange = () => {
  if (document.hidden) polling.pause();
  else polling.resume();
};

const viewAllUsers = () => router.push({ name: "AccountList" });

const showSecurityWarnings = () => {
  if (!accountStore.roles.includes("admin")) return;
  if (window.location.protocol !== "https:") {
    ElNotification({
      title: t("common.securityRisk"),
      dangerouslyUseHTMLString: true,
      message: t("common.noHttpsTip"),
      type: "warning",
    });
  }
};

onMounted(() => {
  componentActive = true;
  document.addEventListener("visibilitychange", handleVisibilityChange);
  if (!document.hidden) polling.start();
  showSecurityWarnings();
});

onBeforeUnmount(() => {
  componentActive = false;
  polling.stop();
  document.removeEventListener("visibilitychange", handleVisibilityChange);
});
</script>

<style lang="scss" scoped>
.dashboard-container {
  display: grid;
  gap: 18px;
}

.dashboard-actions,
.metric-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dashboard-actions small,
.metric-top,
.service-cell span,
.service-cell small {
  color: var(--hui-muted);
}

.traffic-grid,
.service-strip,
.dashboard-grid {
  display: grid;
  gap: 14px;
}

.traffic-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.traffic-metric {
  position: relative;
  display: grid;
  gap: 10px;
  padding: 17px 20px 19px;
}

.traffic-metric-total {
  border-color: color-mix(in srgb, var(--el-color-primary) 32%, var(--hui-border));
  background: color-mix(in srgb, var(--hui-accent-soft) 56%, var(--hui-surface));
}

.traffic-metric strong {
  overflow-wrap: anywhere;
  font-size: clamp(23px, 3vw, 34px);
  font-weight: 740;
  line-height: 1.15;
  letter-spacing: -0.035em;
}

.metric-icon {
  display: grid;
  place-items: center;
  flex: none;
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: var(--hui-accent-soft);
  color: var(--hui-accent-ink);
  font-size: 17px;
}

.service-strip {
  grid-template-columns: minmax(0, 2fr) repeat(2, minmax(0, 1fr));
  gap: 0;
}

.service-cell {
  display: grid;
  align-content: center;
  gap: 7px;
  padding: 14px 18px;
  min-width: 0;

  + .service-cell {
    border-inline-start: 1px solid var(--hui-border);
  }

  strong {
    font-size: 24px;
    line-height: 1.05;
    font-variant-numeric: tabular-nums;
  }

  small {
    overflow-wrap: anywhere;
  }
}

.dashboard-grid {
  grid-template-columns: minmax(0, 1.7fr) minmax(230px, 1fr);
  align-items: start;
}

.ranking-card,
.resource-card {
  min-width: 0;
  overflow: hidden;
}

.resource-list {
  display: grid;
  gap: 24px;
  padding: 4px 20px 22px;
}

.resource-item {
  display: grid;
  gap: 9px;

  strong { color: var(--hui-ink); }
}

@media (width <= 950px) {
  .dashboard-grid { grid-template-columns: minmax(0, 1fr); }
  .resource-card { grid-row: 1; }
}

@media (width <= 600px) {
  .dashboard-actions { align-items: flex-start; flex-wrap: wrap; }
  .traffic-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 9px; }
  .traffic-metric { padding: 12px; }
  .traffic-metric-total { grid-column: 1 / -1; }
  .metric-icon { width: 27px; height: 27px; }
  .service-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .service-version { grid-column: 1 / -1; border-bottom: 1px solid var(--hui-border); }
  .service-version + .service-cell { border-inline-start: 0; }
  .ranking-card :deep(.cell) { padding-inline: 7px; }
}
</style>
