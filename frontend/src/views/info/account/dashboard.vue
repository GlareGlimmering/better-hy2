<template>
  <div class="dashboard-container">
    <div class="dashboard-heading">
      <div>
        <h2>{{ $t("dashboard.title") }}</h2>
        <p>
          {{ $t("dashboard.description") }}
          <span v-if="lastUpdated">
            · {{ $t("dashboard.lastUpdated") }} {{ lastUpdated }}
          </span>
        </p>
      </div>
      <el-button :icon="Refresh" :loading="refreshing" @click="refreshAll">
        {{ $t("common.refresh") }}
      </el-button>
    </div>

    <el-alert
      v-if="systemError"
      :title="systemError"
      type="warning"
      show-icon
      :closable="false"
      class="dashboard-alert"
    />
    <el-alert
      v-if="trafficError"
      :title="trafficError"
      type="warning"
      show-icon
      :closable="false"
      class="dashboard-alert"
    />

    <section v-loading="systemLoading" class="metric-grid system-grid">
      <el-card shadow="never" class="metric-card">
        <span class="metric-label">{{ $t("monitor.cpuPercent") }}</span>
        <strong>{{ formatPercent(systemMonitor.cpuPercent) }}</strong>
        <el-progress
          :percentage="normalizePercent(systemMonitor.cpuPercent)"
          :show-text="false"
        />
      </el-card>
      <el-card shadow="never" class="metric-card">
        <span class="metric-label">{{ $t("monitor.memPercent") }}</span>
        <strong>{{ formatPercent(systemMonitor.memPercent) }}</strong>
        <el-progress
          :percentage="normalizePercent(systemMonitor.memPercent)"
          :show-text="false"
        />
      </el-card>
      <el-card shadow="never" class="metric-card">
        <span class="metric-label">{{ $t("monitor.diskPercent") }}</span>
        <strong>{{ formatPercent(systemMonitor.diskPercent) }}</strong>
        <el-progress
          :percentage="normalizePercent(systemMonitor.diskPercent)"
          :show-text="false"
        />
      </el-card>
      <el-card shadow="never" class="metric-card">
        <div class="metric-row">
          <span class="metric-label">{{
            $t("monitor.hysteria2Running")
          }}</span>
          <el-tag :type="hysteria2Monitor.running ? 'success' : 'danger'">
            {{
              hysteria2Monitor.running
                ? $t("monitor.hysteria2RunningTrue")
                : $t("monitor.hysteria2RunningFalse")
            }}
          </el-tag>
        </div>
        <strong>{{ hysteria2Monitor.version || "-" }}</strong>
        <span class="metric-note">HY2 · Better HY2 {{ systemMonitor.huiVersion || "-" }}</span>
      </el-card>
      <el-card shadow="never" class="metric-card">
        <span class="metric-label">{{
          $t("monitor.hysteria2UserTotal")
        }}</span>
        <strong>{{ hysteria2Monitor.userTotal }}</strong>
        <span class="metric-note">{{ $t("dashboard.currentOnline") }}</span>
      </el-card>
      <el-card shadow="never" class="metric-card">
        <span class="metric-label">{{
          $t("monitor.hysteria2DeviceTotal")
        }}</span>
        <strong>{{ hysteria2Monitor.deviceTotal }}</strong>
        <span class="metric-note">{{ $t("dashboard.currentOnline") }}</span>
      </el-card>
    </section>

    <section v-loading="trafficLoading" class="metric-grid traffic-grid">
      <el-card shadow="never" class="metric-card traffic-card upload-card">
        <span class="metric-label">{{ $t("dashboard.totalUpload") }}</span>
        <strong>{{ formatBytes(trafficMonitor.upload) }}</strong>
      </el-card>
      <el-card shadow="never" class="metric-card traffic-card download-card">
        <span class="metric-label">{{ $t("dashboard.totalDownload") }}</span>
        <strong>{{ formatBytes(trafficMonitor.download) }}</strong>
      </el-card>
      <el-card shadow="never" class="metric-card traffic-card total-card">
        <span class="metric-label">{{ $t("dashboard.totalTraffic") }}</span>
        <strong>{{ formatBytes(trafficMonitor.total) }}</strong>
      </el-card>
    </section>

    <el-card shadow="never" class="ranking-card">
      <template #header>
        <div class="ranking-header">
          <div>
            <strong>{{ $t("dashboard.trafficRanking") }}</strong>
            <p>{{ $t("dashboard.trafficRankingTip") }}</p>
          </div>
          <el-button type="primary" link @click="viewAllUsers">
            {{ $t("dashboard.viewAllUsers") }}
          </el-button>
        </div>
      </template>
      <el-table
        v-loading="trafficLoading"
        :data="trafficMonitor.topUsers"
        :empty-text="$t('dashboard.noTrafficData')"
        class="ranking-table"
      >
        <el-table-column type="index" label="#" width="54" align="center" />
        <el-table-column
          prop="username"
          :label="$t('account.username')"
          min-width="140"
          show-overflow-tooltip
        />
        <el-table-column
          prop="upload"
          :label="$t('account.upload')"
          min-width="120"
          align="right"
        >
          <template #default="scope">{{ formatBytes(scope.row.upload) }}</template>
        </el-table-column>
        <el-table-column
          prop="download"
          :label="$t('account.download')"
          min-width="120"
          align="right"
        >
          <template #default="scope">{{ formatBytes(scope.row.download) }}</template>
        </el-table-column>
        <el-table-column
          prop="total"
          :label="$t('dashboard.totalTraffic')"
          min-width="120"
          align="right"
        >
          <template #default="scope">
            <strong>{{ formatBytes(scope.row.total) }}</strong>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { Refresh } from "@element-plus/icons-vue";
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
import { verifyDefaultPassApi } from "@/api/account";
import { useAccountStore } from "@/store/modules/account";
import { formatBytes } from "@/utils/byte";
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
  trafficMonitor,
  systemLoading,
  trafficLoading,
  systemError,
  trafficError,
} = toRefs(state);

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
    } else {
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
  verifyDefaultPassApi().then((response) => {
    if (response.data) {
      ElNotification({
        title: t("common.securityRisk"),
        dangerouslyUseHTMLString: true,
        message: t("common.defaultPassTip"),
        type: "warning",
      });
    }
  });
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
  padding: 24px;
  background: var(--el-bg-color-page);
}

.dashboard-heading,
.ranking-header,
.metric-row {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.dashboard-heading {
  margin-bottom: 16px;

  h2 {
    margin: 0 0 6px;
    font-size: 24px;
  }

  p {
    margin: 0;
    color: var(--el-text-color-secondary);
  }
}

.dashboard-alert {
  margin-bottom: 12px;
}

.metric-grid {
  display: grid;
  gap: 12px;
  margin-bottom: 16px;
}

.system-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.traffic-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.metric-card :deep(.el-card__body) {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 116px;
}

.metric-card strong {
  overflow: hidden;
  font-size: 24px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.metric-label,
.metric-note,
.ranking-header p {
  color: var(--el-text-color-secondary);
}

.metric-note,
.ranking-header p {
  font-size: 12px;
}

.traffic-card {
  border-left: 4px solid var(--el-color-primary);
}

.upload-card {
  border-left-color: var(--el-color-warning);
}

.download-card {
  border-left-color: var(--el-color-success);
}

.ranking-header p {
  margin: 4px 0 0;
}

@media (width <= 991px) {
  .system-grid,
  .traffic-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (width <= 767px) {
  .dashboard-container {
    padding: 12px;
  }

  .dashboard-heading,
  .ranking-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .dashboard-heading .el-button,
  .ranking-header .el-button {
    width: 100%;
  }

  .system-grid,
  .traffic-grid {
    grid-template-columns: 1fr;
  }

  .ranking-card :deep(.el-card__body) {
    padding: 0;
  }
}
</style>
