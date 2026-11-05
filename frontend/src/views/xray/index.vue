<template>
  <div class="app-container xray-page">
    <el-card class="section-card">
      <template #header>
        <div class="section-header">
          <div><div class="section-title">{{ $t("xray.serviceTitle") }}</div><div class="section-subtitle">{{ $t("xray.serviceHelp") }}</div></div>
          <el-button :loading="loading" @click="refreshStatus">{{ $t("common.refresh") }}</el-button>
        </div>
      </template>
      <el-row :gutter="16">
        <el-col :xs="24" :sm="8"><div class="status-block"><span>{{ $t("xray.running") }}</span><el-tag :type="status.running ? 'success' : status.runtimeReady ? 'info' : 'warning'">{{ runtimeStateLabel }}</el-tag></div></el-col>
        <el-col :xs="24" :sm="8"><div class="status-block"><span>{{ $t("xray.activeVersion") }}</span><strong>{{ status.activeVersion || "-" }}</strong></div></el-col>
        <el-col :xs="24" :sm="8"><div class="status-block"><span>{{ $t("xray.configValid") }}</span><el-tag :type="status.configValid ? 'success' : 'warning'">{{ yesNo(status.configValid) }}</el-tag></div></el-col>
      </el-row>
      <el-alert v-if="status.error || status.lastApplyError" class="status-error" :title="status.lastApplyError || status.error" type="error" :closable="false" show-icon />
      <el-space wrap class="service-actions">
        <el-tooltip v-if="!status.running" :content="status.runtimeReady ? $t('xray.tipStart') : (status.error || $t('xray.notAppliedState'))"><el-button type="success" :loading="loading" :disabled="!status.runtimeReady" @click="runAction('start')">{{ $t("xray.startService") }}</el-button></el-tooltip>
        <template v-else>
          <el-tooltip :content="$t('xray.tipRestart')"><el-button :loading="loading" @click="runAction('restart')">{{ $t("xray.restartService") }}</el-button></el-tooltip>
          <el-tooltip :content="$t('xray.tipStop')"><el-button type="danger" :loading="loading" @click="runAction('stop')">{{ $t("xray.stopService") }}</el-button></el-tooltip>
        </template>
        <el-button @click="$router.push('/log')">{{ $t("xray.viewLogs") }}</el-button>
      </el-space>
    </el-card>

    <el-row :gutter="12" class="node-summary">
      <el-col v-for="item in nodeSummary" :key="item.label" :xs="12" :sm="6"><el-card shadow="never"><div class="summary-value">{{ item.value }}</div><div class="section-subtitle">{{ item.label }}</div></el-card></el-col>
    </el-row>

    <el-card class="section-card">
      <template #header>
        <div class="section-header">
          <div><div class="section-title">{{ $t("xray.outboundList") }}</div><div class="section-subtitle">{{ $t("xray.outboundListHelp") }}</div></div>
          <el-button type="primary" @click="openCreate">+ {{ $t("xray.addOutbound") }}</el-button>
        </div>
      </template>
      <el-table v-loading="loading" :data="settings.outbounds" stripe class="outbound-table">
        <el-table-column :label="$t('xray.name')" min-width="150"><template #default="scope"><el-tooltip :content="scope.row.name" placement="top"><span class="truncate-cell">{{ scope.row.name }}</span></el-tooltip></template></el-table-column>
        <el-table-column :label="$t('xray.protocol')" width="130"><template #default="scope">{{ protocolLabel(scope.row.protocol) }}</template></el-table-column>
        <el-table-column :label="$t('xray.remoteAddress')" min-width="220"><template #default="scope"><el-tooltip :content="`${scope.row.address}:${scope.row.port}`" placement="top"><span class="truncate-cell">{{ scope.row.address }}:{{ scope.row.port }}</span></el-tooltip></template></el-table-column>
        <el-table-column :label="$t('xray.localSocks')" min-width="170"><template #default="scope">{{ scope.row.protocol === 'socks5' ? $t('xray.nativeHy2') : `${scope.row.socksListen}:${scope.row.socksPort}` }}</template></el-table-column>
        <el-table-column :label="$t('xray.enabled')" width="100"><template #default="scope"><el-tag :type="scope.row.enabled ? 'success' : 'info'">{{ yesNo(scope.row.enabled) }}</el-tag></template></el-table-column>
        <el-table-column :label="$t('xray.lastTest')" min-width="270">
          <template #default="scope">
            <div class="test-results" :aria-label="$t('xray.lastTestOnly')">
              <div :class="testClass(scope.row.id, 'tcp')">
                <span class="test-protocol">TCP</span>
                <span>{{ protocolTestSummary(scope.row.id, "tcp") }}</span>
              </div>
              <div :class="testClass(scope.row.id, 'udp')">
                <span class="test-protocol">UDP</span>
                <span>{{ protocolTestSummary(scope.row.id, "udp") }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operate')" width="230" fixed="right">
          <template #default="scope">
            <el-tooltip :content="$t('xray.tipTest')"><el-button link type="primary" :loading="testing[scope.row.id]" :disabled="!scope.row.enabled || !status.running || testing[scope.row.id]" @click="testOutbound(scope.row)">{{ $t("xray.test") }}</el-button></el-tooltip>
            <el-button link type="primary" @click="openEdit(scope.row)">{{ $t("common.update") }}</el-button>
            <el-tooltip :content="$t('xray.tipDelete')"><el-button link type="danger" @click="removeOutbound(scope.row)">{{ $t("common.delete") }}</el-button></el-tooltip>
          </template>
        </el-table-column>
        <template #empty><div class="empty-state"><p>{{ $t("xray.emptyNodes") }}</p><el-button type="primary" @click="openCreate">{{ $t("xray.addOutbound") }}</el-button></div></template>
      </el-table>
    </el-card>

    <el-card class="section-card">
      <template #header><div class="section-title">{{ $t("xray.configurationState") }}</div></template>
      <el-alert :title="dirty || status.pendingChanges ? $t('xray.pendingChanges') : $t('xray.noPendingChanges')" :type="dirty || status.pendingChanges ? 'warning' : 'success'" :closable="false" show-icon />
      <div class="apply-time">{{ $t("xray.lastApplied") }}: {{ formatTestTime(status.lastAppliedAt) }}</div>
      <el-form label-position="top" class="advanced-form">
        <el-row :gutter="16">
          <el-col :xs="24" :md="12"><el-form-item :label="$t('xray.version')"><el-space wrap><strong>{{ settings.version }}</strong><el-tooltip :content="$t('xray.tipInstall')"><el-button :loading="loading" @click="install">{{ $t("xray.installKernel") }}</el-button></el-tooltip></el-space></el-form-item></el-col>
          <el-col :xs="24" :md="12"><el-form-item :label="$t('xray.caCertificate')"><el-upload :auto-upload="false" :show-file-list="false" accept=".crt,.pem" :on-change="uploadCA"><el-button>{{ $t("xray.uploadCA") }}</el-button></el-upload><span class="detail">{{ status.ca.valid ? status.ca.subject : status.ca.error || $t("xray.systemCaAvailable") }}</span></el-form-item></el-col>
        </el-row>
      </el-form>
      <el-space wrap>
        <el-tooltip :content="$t('xray.tipApply')"><el-button type="primary" :loading="loading" @click="saveAndApply">{{ $t("xray.applyChanges") }}</el-button></el-tooltip>
        <el-tooltip :content="$t('xray.tipSaveDraft')"><el-button :loading="loading" @click="saveDraft">{{ $t("xray.saveDraft") }}</el-button></el-tooltip>
        <el-tooltip :content="$t('xray.tipValidate')"><el-button :loading="loading" @click="validateConfig">{{ $t("xray.checkConfig") }}</el-button></el-tooltip>
      </el-space>
    </el-card>

    <el-drawer v-model="editorVisible" :title="editingId ? $t('xray.editOutbound') : $t('xray.addOutbound')" size="min(600px, 94%)" destroy-on-close :close-on-click-modal="false">
      <el-form ref="editorRef" :model="editor" :rules="rules" label-position="top">
        <div class="form-section-title">{{ $t("xray.basicFields") }}</div>
        <el-form-item :label="$t('xray.protocol')" prop="protocol"><el-select v-model="editor.protocol" :disabled="!!editingId" class="full-width"><el-option label="VLESS" value="vless" /><el-option label="HTTP Proxy" value="http" /><el-option label="SOCKS5 Proxy" value="socks5" /></el-select><div class="field-help">{{ $t(`xray.protocolHelp.${editor.protocol}`) }}</div></el-form-item>
        <el-form-item :label="$t('xray.name')" prop="name"><el-input v-model="editor.name" maxlength="64" show-word-limit /><div class="field-help">{{ $t("xray.nameHelp") }}</div></el-form-item>
        <el-row :gutter="12"><el-col :xs="24" :sm="16"><el-form-item :label="$t('xray.address')" prop="address"><el-input v-model="editor.address" /></el-form-item></el-col><el-col :xs="24" :sm="8"><el-form-item :label="$t('xray.port')" prop="port"><el-input-number v-model="editor.port" :min="1" :max="65535" class="full-width" /></el-form-item></el-col></el-row>
        <div class="form-section-title">{{ $t("xray.authenticationFields") }}</div>
        <template v-if="editor.protocol === 'vless'">
          <el-form-item :label="$t('xray.uuid')" prop="uuid"><el-input v-model="editor.uuid" type="password" show-password :placeholder="editor.uuidSet ? $t('xray.uuidUnchanged') : $t('xray.uuidRequired')" autocomplete="new-password" /></el-form-item>
          <el-form-item :label="$t('xray.sni')" prop="tlsSni"><el-input v-model="editor.tlsSni" /><div class="field-help">{{ $t("xray.sniHelp") }}</div></el-form-item>
          <el-form-item :label="$t('xray.caMode')"><el-radio-group v-model="editor.useCustomCa"><el-radio :label="false">{{ $t("xray.systemCa") }}</el-radio><el-radio :label="true">{{ $t("xray.customCa") }}</el-radio></el-radio-group><div class="field-help">{{ $t("xray.caHelp") }}</div></el-form-item>
        </template>
        <template v-else>
          <el-form-item :label="$t('xray.username')"><el-input v-model="editor.username" autocomplete="off" /></el-form-item>
          <el-form-item :label="$t('xray.password')"><el-input v-model="editor.password" type="password" show-password :placeholder="editor.passwordSet ? $t('xray.passwordUnchanged') : ''" autocomplete="new-password" /></el-form-item>
        </template>
        <div class="form-section-title">{{ $t("xray.routingFields") }}</div>
        <el-form-item v-if="editor.protocol !== 'socks5'" :label="$t('xray.localSocksPort')" prop="socksPort"><el-input-number v-model="editor.socksPort" :min="1024" :max="65535" class="full-width" /><div class="field-help">{{ $t("xray.socksHelp") }}</div></el-form-item>
        <el-alert v-else :title="$t('xray.nativeSocksHelp')" type="info" :closable="false" show-icon />
        <el-form-item :label="$t('xray.enabled')"><el-switch v-model="editor.enabled" /></el-form-item>
      </el-form>
      <template #footer><el-button :disabled="loading" @click="editorVisible = false">{{ $t("common.cancel") }}</el-button><el-button type="primary" :loading="loading" :disabled="loading" @click="saveEditor">{{ $t("common.save") }}</el-button></template>
    </el-drawer>

  </div>
</template>

<script setup lang="ts">
import { ElMessage, ElMessageBox, FormInstance, FormRules, UploadFile } from "element-plus";
import { useI18n } from "vue-i18n";
import { applyXrayApi, getXraySettingsApi, getXrayStatusApi, installXrayApi, restartXrayApi, saveXrayDraftApi, startXrayApi, stopXrayApi, testXrayOutboundApi, uploadXrayCAApi, validateXrayApi } from "@/api/xray";
import type { XrayOutbound, XraySettings, XrayStatus, XrayTestResult } from "@/api/xray/types";
import { validateOutboundEditor } from "./outbound-form";
import { failedTestResult, relativeTestTime, visibleExitIp } from "./test-result";

const { t } = useI18n();
const loading = ref(false);
const dirty = ref(false);
const editorVisible = ref(false);
const editingId = ref("");
const editorRef = ref<FormInstance>();
const settings = reactive<XraySettings>({ version: "", outbounds: [] });
const status = reactive<XrayStatus>({ installedVersions: [], versionInstalled: false, serviceInstalled: false, runtimeReady: false, runtimeState: "not_installed", running: false, configValid: false, pendingChanges: false, ca: { exists: false, valid: false } });
const emptyOutbound = (): XrayOutbound => ({ id: "", name: "", protocol: "vless", enabled: true, address: "", port: 443, uuid: "", uuidSet: false, username: "", password: "", passwordSet: false, tcpSupported: true, udpSupported: true, network: "tcp", tlsSni: "", useCustomCa: false, socksListen: "127.0.0.1", socksPort: 10808, outboundTag: "" });
const editor = reactive<XrayOutbound>(emptyOutbound());
const tests = reactive<Record<string, { tcp?: XrayTestResult; udp?: XrayTestResult }>>({});
const testing = reactive<Record<string, boolean>>({});
const testClock = ref(Date.now());
let testClockTimer: number | undefined;
const nodeSummary = computed(() => [
  { label: t("xray.totalNodes"), value: settings.outbounds.length },
  { label: "VLESS", value: settings.outbounds.filter(item => item.protocol === "vless").length },
  { label: "HTTP", value: settings.outbounds.filter(item => item.protocol === "http").length },
  { label: "SOCKS5", value: settings.outbounds.filter(item => item.protocol === "socks5").length },
]);
const runtimeStateLabel = computed(() => {
  if (status.runtimeState === "not_required") return t("xray.notRequiredState");
  if (status.runtimeState === "not_installed") return t("xray.notInstalledState");
  if (status.runtimeState === "service_missing") return t("xray.serviceMissingState");
  if (status.runtimeState === "not_applied") return t("xray.notAppliedState");
  return status.running ? t("xray.runningState") : t("xray.stoppedState");
});
const rules: FormRules = { name: [{ required: true, message: t("xray.validation.nameRequired"), trigger: "blur" }], address: [{ required: true, message: t("xray.validation.addressRequired"), trigger: "blur" }], port: [{ required: true, message: t("xray.validation.portRequired"), trigger: "blur" }], tlsSni: [{ required: true, message: t("xray.validation.sniRequired"), trigger: "blur" }], socksPort: [{ required: true, message: t("xray.validation.socksPortRequired"), trigger: "blur" }] };
const yesNo = (value: boolean) => value ? t("common.yes") : t("common.no");
const load = async () => { loading.value = true; try { const [{ data: draft }, { data: current }] = await Promise.all([getXraySettingsApi(), getXrayStatusApi()]); Object.assign(settings, draft, { outbounds: draft.outbounds.map(item => ({ ...item, uuid: "" })) }); Object.assign(status, current); dirty.value = false; } finally { loading.value = false; } };
const refreshStatus = async () => { const { data } = await getXrayStatusApi(); Object.assign(status, data); };
const openCreate = () => { editingId.value = ""; Object.assign(editor, emptyOutbound(), { socksPort: nextPort() }); editorVisible.value = true; };
const openEdit = (outbound: XrayOutbound) => { editingId.value = outbound.id; Object.assign(editor, JSON.parse(JSON.stringify(outbound)), { uuid: "", password: "" }); editorVisible.value = true; };
const nextPort = () => { const used = new Set(settings.outbounds.map(item => item.socksPort)); let port = 10808; while (used.has(port)) port++; return port; };
const saveEditor = async () => { await editorRef.value?.validate(); const validationError = validateOutboundEditor(editor, settings.outbounds, editingId.value); if (validationError) { ElMessage.error(t(`xray.validation.${validationError}`)); return; } const copy = JSON.parse(JSON.stringify(editor)) as XrayOutbound; copy.name = copy.name.trim(); copy.address = copy.address.trim(); const index = settings.outbounds.findIndex(item => item.id === editingId.value); const previous = index >= 0 ? JSON.parse(JSON.stringify(settings.outbounds[index])) as XrayOutbound : undefined; if (index >= 0) settings.outbounds.splice(index, 1, copy); else settings.outbounds.push(copy); dirty.value = true; try { await saveAndApply(); editorVisible.value = false; } catch (error) { if (index >= 0 && previous) settings.outbounds.splice(index, 1, previous); else settings.outbounds.pop(); throw error; } };
const removeOutbound = async (outbound: XrayOutbound) => { await ElMessageBox.confirm(t("xray.deleteConfirm", { name: outbound.name }), t("common.confirm")); const index = settings.outbounds.findIndex(item => item.id === outbound.id); if (index < 0) return; const before = settings.outbounds.map(item => ({ ...item })); settings.outbounds.splice(index, 1); dirty.value = true; try { await saveAndApply(); } catch (error) { try { await saveXrayDraftApi({ ...settings, outbounds: before }); await load(); } catch { await load(); } throw error; } };
const saveDraft = async () => { loading.value = true; try { await saveXrayDraftApi(settings); ElMessage.success(t("xray.saved")); await load(); } finally { loading.value = false; } };
const saveAndApply = async () => { loading.value = true; try { await saveXrayDraftApi(settings); await applyXrayApi(); dirty.value = false; ElMessage.success(t("xray.applied")); await load(); } catch (error) { dirty.value = true; await refreshStatus(); throw error; } finally { loading.value = false; } };
const validateConfig = async () => { loading.value = true; try { await saveXrayDraftApi(settings); await validateXrayApi(); ElMessage.success(t("xray.validationPassed")); } finally { loading.value = false; } };
const install = async () => { if (!settings.version) { ElMessage.error(t("xray.versionRequired")); return; } loading.value = true; try { await installXrayApi(settings.version); ElMessage.success(t("xray.installed")); await refreshStatus(); } finally { loading.value = false; } };
const runAction = async (action: "start" | "stop" | "restart") => { if (action === "stop") await ElMessageBox.confirm(t("xray.stopConfirm"), t("common.confirm")); loading.value = true; try { if (action === "start") await startXrayApi(); else if (action === "stop") await stopXrayApi(); else await restartXrayApi(); ElMessage.success(t("common.success")); await refreshStatus(); } finally { loading.value = false; } };
const testOutbound = async (outbound: XrayOutbound) => {
  testing[outbound.id] = true;
  try {
    const [tcp, udp] = await Promise.allSettled([
      testXrayOutboundApi(outbound.id, "tcp"),
      testXrayOutboundApi(outbound.id, "udp"),
    ]);
    tests[outbound.id] = {
      tcp: tcp.status === "fulfilled" ? tcp.value.data : failedTestResult("tcp", tcp.reason),
      udp: udp.status === "fulfilled" ? udp.value.data : failedTestResult("udp", udp.reason),
    };
    testClock.value = Date.now();
  } finally {
    testing[outbound.id] = false;
  }
};
const relativeTimeLabel = (testedAt: number) => {
  const relative = relativeTestTime(testedAt, testClock.value);
  return t(`xray.timeAgo.${relative.unit}`, { count: relative.value });
};
const protocolTestSummary = (id: string, protocol: "tcp" | "udp") => {
  const result = tests[id]?.[protocol];
  if (!result) return t("xray.notTested");
  if (!result.supported) return `${t("xray.unsupported")} · ${relativeTimeLabel(result.testedAt)}`;
  if (!result.remoteConnectivity) return `${t("xray.failed")} · ${result.error || "-"} · ${relativeTimeLabel(result.testedAt)}`;
  const parts = [t("xray.success"), `${result.latencyMs} ms`, relativeTimeLabel(result.testedAt)];
  const exitIp = visibleExitIp(result);
  if (exitIp) parts.push(`${t("xray.exitIp")} ${exitIp}`);
  return parts.join(" · ");
};
const testClass = (id: string, protocol: "tcp" | "udp") => {
  const result = tests[id]?.[protocol];
  if (!result) return "test-result muted";
  if (!result.supported) return "test-result warning-text";
  return result.remoteConnectivity ? "test-result success-text" : "test-result danger-text";
};
const formatTestTime = (value?: number) => value ? new Date(value).toLocaleString() : "-";
const protocolLabel = (protocol: XrayOutbound["protocol"]) => protocol === "vless" ? "VLESS" : protocol === "http" ? "HTTP Proxy" : "SOCKS5 Proxy";
const uploadCA = async (file: UploadFile) => { if (!file.raw) return; const data = new FormData(); data.append("file", file.raw); loading.value = true; try { await uploadXrayCAApi(data); ElMessage.success(t("xray.caUploaded")); await refreshStatus(); } finally { loading.value = false; } };
onMounted(() => {
  load();
  testClockTimer = window.setInterval(() => { testClock.value = Date.now(); }, 30_000);
});
onUnmounted(() => {
  if (testClockTimer) window.clearInterval(testClockTimer);
});
</script>

<style scoped>
.xray-page { max-width: 1500px; margin: 0 auto; }
.section-card { margin-bottom: 18px; }
.node-summary { margin-bottom: 18px; }
.summary-value { font-size: 24px; font-weight: 600; }
.section-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.section-title { font-size: 17px; font-weight: 600; }
.section-subtitle, .field-help, .detail, .muted { color: var(--el-text-color-secondary); font-size: 12px; }
.status-block { display: flex; align-items: center; justify-content: space-between; min-height: 36px; padding: 0 8px; }
.status-error, .service-actions, .advanced-form, .apply-time { margin-top: 16px; }
.apply-time { color: var(--el-text-color-secondary); font-size: 13px; }
.success-text { color: var(--el-color-success); }
.danger-text { color: var(--el-color-danger); }
.warning-text { color: var(--el-color-warning); }
.test-results { display: grid; gap: 5px; font-size: 12px; line-height: 1.4; }
.test-result { display: flex; align-items: flex-start; gap: 8px; }
.test-protocol { flex: 0 0 30px; color: var(--el-text-color-secondary); font-weight: 600; }
.field-help { margin-top: 5px; line-height: 1.4; }
.full-width { width: 100%; }
.form-section-title { margin: 18px 0 12px; padding-bottom: 8px; border-bottom: 1px solid var(--el-border-color-lighter); color: var(--el-text-color-primary); font-weight: 600; }
.truncate-cell { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.empty-state { padding: 28px 0; color: var(--el-text-color-secondary); }
@media (max-width: 768px) { .section-header { align-items: flex-start; flex-direction: column; } .outbound-table { width: 100%; } .service-actions { width: 100%; } }
</style>
