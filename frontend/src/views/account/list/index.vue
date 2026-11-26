<template>
  <div class="hui-page accounts-page">
    <header class="hui-page-heading">
      <h1>{{ $t("route.accountList") }}</h1>
      <el-button type="primary" :icon="Plus" @click="handleAdd">{{ $t("common.add") }}</el-button>
    </header>

    <section class="hui-surface accounts-panel">
      <div class="account-filters">
      <el-form ref="queryFormRef" :model="queryParams" :inline="true">
        <el-form-item :label="$t('account.remark')" prop="remark">
          <el-input
            v-model="queryParams.remark"
            :placeholder="$t('account.remark')"
            clearable
            @keyup.enter="handleQuery"
          />
        </el-form-item>
        <el-form-item :label="$t('account.username')" prop="username">
          <el-input
            v-model="queryParams.username"
            :placeholder="$t('account.username')"
            clearable
            @keyup.enter="handleQuery"
          />
        </el-form-item>
        <el-form-item :label="$t('common.deleted')" prop="deleted">
          <el-select
            v-model="queryParams.deleted"
            :placeholder="$t('common.all')"
            clearable
          >
            <el-option :label="$t('common.enable')" value="0" />
            <el-option :label="$t('common.disable')" value="1" />
          </el-select>
        </el-form-item>

        <div class="filter-actions">
          <el-button :icon="Search" @click="handleQuery">{{ $t("common.search") }}</el-button>
          <el-button text :icon="Refresh" @click="resetQuery">{{ $t("common.reset") }}</el-button>
        </div>
      </el-form>
      </div>

      <div class="routing-strip" role="status">
        <span class="hui-status" :class="routingStatus.ready ? 'is-live' : 'is-warning'">
          {{ $t(routingStatus.ready ? 'account.routingReady' : 'account.routingNotReady') }}
        </span>
        <span class="routing-detail">{{ routingStatusDescription }}</span>
        <el-button v-if="routingStatus.unboundUsers.length" link type="primary" @click="openRoutingBindings">
          {{ $t("account.bindUnassigned") }}
        </el-button>
      </div>
      <el-table v-loading="loading" :data="records" class="hui-table account-table" @row-click="handleUpdate">
        <el-table-column
          key="username"
          :label="$t('account.username')"
          align="left"
          prop="username"
          :min-width="isMobile ? 164 : 160"
          fixed="left"
        >
          <template #default="{ row }">
            <div class="account-identity">
              <span class="account-avatar" aria-hidden="true">{{ row.username?.slice(0, 1).toUpperCase() }}</span>
              <div class="account-identity-details">
                <button type="button" class="account-name" :title="row.username" @click.stop="handleUpdate(row)">{{ row.username }}</button>
                <small v-if="isMobile" class="account-summary">
                  {{ row.deleted !== 0 ? $t("common.disable") : (row.online ? $t("account.online") : $t("account.offline")) }}
                  · {{ row.outbound || $t("account.unbound") }}
                  · {{ formatTrafficG(row.upload + row.download) }}
                </small>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          v-if="!isMobile"
          key="status"
          :label="$t('account.onlineStatus')"
          align="center"
          width="100"
        >
          <template #default="scope">
            <span class="hui-status" :class="{ 'is-live': scope.row.deleted === 0 && scope.row.online }">
              {{ $t(scope.row.deleted !== 0 ? "common.disable" : (scope.row.online ? "account.online" : "account.offline")) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column
          v-if="!isMobile"
          key="outbound"
          :label="$t('account.outbound')"
          align="center"
          prop="outbound"
          min-width="140"
          show-overflow-tooltip
        >
          <template #default="scope">
            <span class="outbound-name" :class="{ unbound: !scope.row.outbound }">
              {{ scope.row.outbound || $t("account.unbound") }}
            </span>
          </template>
        </el-table-column>
        <el-table-column v-if="!isMobile" :label="$t('account.bandwidth')" min-width="120">
          <template #default="{ row }">
            <span v-if="row.role !== 'admin'">↑ {{ row.uploadMbps || "∞" }} · ↓ {{ row.downloadMbps || "∞" }} Mbps</span>
          </template>
        </el-table-column>
        <el-table-column v-if="!isMobile" key="traffic" :label="$t('account.trafficUsed')" align="center" min-width="130">
          <template #default="scope">
            <el-tooltip :content="`${$t('account.upload')}: ${formatTrafficG(scope.row.upload)} · ${$t('account.download')}: ${formatTrafficG(scope.row.download)}`">
              <span>{{ formatTrafficG(scope.row.upload + scope.row.download) }} / {{ scope.row.quota < 0 ? "∞" : formatTrafficG(scope.row.quota) }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column
          v-if="!isMobile"
          key="expireTime"
          :label="$t('account.expireTime')"
          align="center"
          prop="expireTime"
          width="151"
        >
          <template #default="scope">
            <span v-if="scope.row.expireTime >= permanentAccountExpiry">{{ $t("account.expirePermanent") }}</span>
            <el-tooltip
              v-else-if="scope.row.expireTime"
              :content="timestampToFullDateTimeWithZone(scope.row.expireTime)"
              placement="top"
            >
              <span class="time-cell">{{
                timestampToCompactDateTime(scope.row.expireTime)
              }}</span>
            </el-tooltip>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column align="right" width="124" fixed="right">
          <template #default="{ row }">
            <div class="account-links">
              <el-tooltip :content="$t('common.subscribe')">
                <el-button text :icon="Link" :aria-label="$t('common.subscribe')" @click.stop="handleSubscribe(row)" />
              </el-tooltip>
              <el-tooltip :content="$t('common.nodeUrl')">
                <el-button text :icon="Document" :aria-label="$t('common.nodeUrl')" @click.stop="handleNodeUrl(row)" />
              </el-tooltip>
              <el-tooltip :content="$t('common.nodeQrCode')">
                <el-button text :aria-label="$t('common.nodeQrCode')" @click.stop="handleQrCode(row)">
                  <span class="account-qr-icon" aria-hidden="true"><i /><i /><i /><i /></span>
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="total > 0" class="accounts-pagination">
        <pagination
          :total="total"
          :layout="isMobile ? 'prev, pager, next' : undefined"
          v-model:page="queryParams.pageNum"
          v-model:limit="queryParams.pageSize"
          @pagination="handleQuery"
        />
      </div>
    </section>

    <el-dialog
      v-model="routingBindingsVisible"
      :title="$t('account.bindUnassigned')"
      :width="useFullscreenDialog ? 'calc(100% - 24px)' : '480px'"
      :close-on-click-modal="!routingBindingsSaving"
      :close-on-press-escape="!routingBindingsSaving"
      :show-close="!routingBindingsSaving"
    >
      <p>{{ $t("account.bindUnassignedHelp") }}</p>
      <el-form label-position="top">
        <el-form-item v-for="username in routingBindingUsers" :key="username" :label="username">
          <el-select
            :model-value="routingBindings.get(username)"
            :placeholder="$t('account.unbound')"
            :disabled="routingBindingsSaving"
            style="width: 100%"
            @update:model-value="routingBindings.set(username, $event)"
          >
            <el-option v-for="item in ordinaryOutboundOptions" :key="item.name" :label="item.name" :value="item.name" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button :disabled="routingBindingsSaving" @click="routingBindingsVisible = false">{{ $t("common.cancel") }}</el-button>
        <el-button type="primary" :loading="routingBindingsSaving" :disabled="!routingBindingsComplete" @click="saveRoutingBindings">
          {{ $t("common.save") }}
        </el-button>
      </template>
    </el-dialog>

    <el-drawer
      :title="dialog.title"
      v-model="dialog.visible"
      :size="isMobile ? '100%' : '500px'"
      class="account-drawer"
      append-to-body
      destroy-on-close
      :close-on-click-modal="!accountSaving"
      :close-on-press-escape="!accountSaving"
      :show-close="!accountSaving"
    >
      <el-form
        ref="dataFormRef"
        :rules="dataForm.id === undefined ? dataFormAddRules : dataFormUpdateRules"
        :model="dataForm"
        label-position="top"
      >
        <el-alert
          v-if="drawerAccount"
          class="account-usage"
          type="info"
          :closable="false"
          :title="`${$t('account.trafficUsed')}: ${formatTrafficG(drawerAccount.upload + drawerAccount.download)} · ${$t('account.trafficRemaining')}: ${drawerAccount.quota < 0 ? '∞' : formatTrafficG(drawerAccount.quota - drawerAccount.upload - drawerAccount.download)} · ${$t('account.device')}: ${drawerAccount.device}/${drawerAccount.deviceNo}`"
        />
        <el-form-item :label="$t('account.username')" prop="username">
          <el-input
            v-model="dataForm.username"
            :placeholder="$t('account.username')"
            maxlength="32"
            clearable
          />
        </el-form-item>
        <el-form-item :label="$t('account.conPass')" prop="conPass">
          <el-input
            :model-value="connectionPasswordDisplay"
            :placeholder="$t('account.conPass')"
            :type="passwordVisible ? 'text' : 'password'"
            readonly
          >
            <template #suffix>
              <el-button
                v-if="dataForm.conPass || dataForm.id !== undefined"
                link
                :icon="View"
                :loading="passwordRevealing"
                :disabled="passwordGenerating"
                :aria-label="$t(passwordVisible ? 'account.hidePassword' : 'account.showPassword')"
                :aria-pressed="passwordVisible"
                @click="togglePasswordVisibility"
              />
            </template>
            <template #append>
              <el-button :loading="passwordGenerating" :disabled="accountSaving || passwordRevealing" @click="generatePassword">{{ $t("account.generatePassword") }}</el-button>
            </template>
          </el-input>
        </el-form-item>
        <el-alert
          v-if="dataForm.role === 'user' && dataForm.username && !legacyHomeProxyUsername.test(dataForm.username)"
          class="account-usage"
          type="warning"
          :closable="false"
          :title="$t('account.homeProxyUsernameWarning')"
        />
        <el-alert
          v-else-if="dataForm.role === 'user' && !dataForm.conPass && savedHomeProxyCredentialIncompatible"
          class="account-usage"
          type="warning"
          :closable="false"
          :title="$t('account.homeProxyCredentialWarning')"
        />
        <el-alert
          v-if="dataForm.role === 'user' && dataForm.id !== undefined && dataForm.conPass"
          class="account-usage"
          type="info"
          :closable="false"
          :title="$t('account.homeProxyRotationNotice')"
        />
        <el-form-item v-if="dataForm.role !== 'admin' && (dataForm.id === undefined || relayAccount)" :label="$t('account.kind')">
          <el-radio-group v-if="dataForm.id === undefined" v-model="relayAccount" @change="changeAccountKind">
            <el-radio :label="false">{{ $t("account.kindProxy") }}</el-radio>
            <el-radio :label="true">{{ $t("account.kindRelay") }}</el-radio>
          </el-radio-group>
          <el-tag v-else>{{ $t("account.kindRelay") }}</el-tag>
          <div v-if="relayAccount" class="form-tip">{{ $t("account.relayHelp") }}</div>
        </el-form-item>
        <el-form-item :label="$t('account.outbound')" prop="outbound" :required="relayAccount">
          <el-select
            v-model="dataForm.outbound"
            :loading="outboundOptionsLoading"
            @visible-change="refreshOutboundOptions"
            :placeholder="$t('account.unbound')"
            :disabled="outboundFormState.disabled"
            clearable
            style="width: 100%"
          >
            <el-option
              v-for="item in outboundFormState.options"
              :key="item.name.toLowerCase()"
              :label="`${item.name} (${item.type.toUpperCase()})`"
              :value="item.name"
            />
          </el-select>
          <div v-if="outboundFormState.admin" class="form-tip">
            {{ $t("account.outboundAdminOnly") }}
          </div>
          <div v-else-if="relayAccount && outboundFormState.noOptions && !outboundOptionsLoading && !outboundOptionsFailed" class="form-tip">
            {{ $t("account.relayOutboundMissing") }}
            <el-button link type="primary" @click="openHysteriaSettings">
              {{ $t("account.configureRelayOutbound") }}
            </el-button>
          </div>
          <div v-else-if="!relayAccount && outboundFormState.noOptions" class="form-tip">
            {{ $t("account.outboundUnavailable") }}
            <el-button link type="primary" @click="openHysteriaSettings">
              {{ $t("account.configureOutbounds") }}
            </el-button>
          </div>
          <div v-else class="form-tip">{{ $t("account.outboundTip") }}</div>
          <div v-if="dataForm.id !== undefined && dataForm.outbound && routingStatus.unboundUsers.some(user => user !== dataForm.username)" class="form-tip">
            {{ $t("account.bindUnassignedHelp") }}
          </div>
          <el-alert v-if="outboundOptionsFailed" type="error" :closable="false" :title="$t('account.outboundLoadFailed')" />
          <el-alert v-else-if="!outboundOptionsLoading && unavailableBinding" type="warning" :closable="false" :title="$t('account.outboundBindingUnavailable', { name: dataForm.outbound })" />
          <div v-if="outboundExcludedCount > 0" class="form-tip">{{ $t('account.outboundHistoricalExcluded') }}</div>
        </el-form-item>
        <template v-if="creationBindingUsers.length">
          <p class="form-tip">{{ $t("account.bindUnassignedOnCreate") }}</p>
          <el-form-item v-for="username in creationBindingUsers" :key="username" :label="username" required>
            <el-select
              :model-value="creationBindings.get(username)"
              :placeholder="$t('account.unbound')"
              style="width: 100%"
              @update:model-value="creationBindings.set(username, $event)"
            >
              <el-option v-for="item in ordinaryOutboundOptions" :key="item.name" :label="item.name" :value="item.name" />
            </el-select>
          </el-form-item>
        </template>
        <el-form-item :label="$t('account.quota')" prop="quota">
          <el-switch
            v-model="unlimitedQuota"
            :inactive-text="$t('account.quotaUnlimited')"
            :active-text="$t('account.quotaUnlimited')"
          />
          <el-input-number
            v-if="!unlimitedQuota"
            v-model="quotaTmp"
            :min="0"
            :step="0.1"
            :precision="1"
          />
          <div>{{ $t("account.quotaGBHelp") }}</div>
        </el-form-item>
        <template v-if="dataForm.role !== 'admin'">
          <el-form-item :label="$t('account.uploadMbps')" prop="uploadMbps">
            <el-input-number v-model="dataForm.uploadMbps" :min="0" :max="100000" :precision="0" />
          </el-form-item>
          <el-form-item :label="$t('account.downloadMbps')" prop="downloadMbps">
            <el-input-number v-model="dataForm.downloadMbps" :min="0" :max="100000" :precision="0" />
            <div class="form-tip">{{ $t("account.bandwidthHelp") }}</div>
          </el-form-item>
        </template>
        <el-form-item :label="$t('account.deviceNo')" prop="deviceNo">
          <el-input-number
            v-model="dataForm.deviceNo"
            :placeholder="$t('account.deviceNo')"
            :min="1"
            :controls="false"
            :precision="0"
            clearable
            style="width: 220px"
          />
        </el-form-item>
        <el-form-item :label="$t('account.expireTime')" prop="expireTime">
          <el-date-picker
            v-model="accountExpiryDate"
            type="datetime"
            :placeholder="$t(dataForm.expireTime >= permanentAccountExpiry ? 'account.expirePermanent' : 'account.expireTime')"
            format="YYYY-MM-DD HH:mm:ss"
            :clearable="false"
            style="width: 100%"
          />
          <div class="account-expiry-presets">
            <el-button v-for="months in accountExpiryMonths" :key="months" size="small" @click="dataForm.expireTime = accountExpiryAfterMonths(months)">
              {{ $t("account.expireInMonths", { months }) }}
            </el-button>
            <el-button size="small" @click="dataForm.expireTime = permanentAccountExpiry">{{ $t("account.expirePermanent") }}</el-button>
          </div>
        </el-form-item>
        <el-form-item :label="$t('common.deleted')" prop="deleted">
          <el-radio-group v-model="dataForm.deleted">
            <el-radio :label="0">{{ $t("common.enable") }}</el-radio>
            <el-radio :label="1">{{ $t("common.disable") }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="$t('account.remark')" prop="remark">
          <el-input
            v-model="dataForm.remark"
            type="textarea"
            :rows="3"
            :placeholder="$t('account.remark')"
            maxlength="50"
            show-word-limit
          />
        </el-form-item>
      </el-form>

      <div v-if="drawerAccount" class="drawer-danger-zone">
        <div class="drawer-section-title">{{ $t("account.dangerZone") }}</div>
        <el-space wrap>
          <el-button @click="confirmResetTraffic(drawerAccount)">{{ $t("common.resetTraffic") }}</el-button>
          <el-button @click="handleKick(drawerAccount)">{{ $t("account.kick") }}</el-button>
          <el-button v-if="calculateTimeDifference(drawerAccount.kickUtilTime) !== '-'" @click="confirmReleaseKickWithPrompt(drawerAccount)">{{ $t("account.releaseKick") }}</el-button>
          <el-button type="danger" plain @click="handleDelete(drawerAccount)">{{ $t("common.delete") }}</el-button>
        </el-space>
      </div>
      <template #footer>
        <div class="drawer-footer">
          <el-button :disabled="accountSaving" @click="closeDialog">{{ $t("common.cancel") }}</el-button>
          <el-button type="primary" :loading="accountSaving" :disabled="accountSaving || passwordGenerating || !creationBindingsComplete || !relayBindingReady" @click="submitForm">{{ $t("common.save") }}</el-button>
        </div>
      </template>
    </el-drawer>

    <el-dialog
      :title="dialogKick.title"
      v-model="dialogKick.visible"
      width="min(600px, calc(100vw - 24px))"
      :fullscreen="useFullscreenDialog"
      append-to-body
      @close="closeDialogKick"
    >
      <el-form
        ref="kickFormRef"
        :model="kickForm"
        :label-width="isMobile ? 'auto' : '110px'"
        :label-position="isMobile ? 'top' : 'right'"
      >
        <el-form-item :label="$t('account.kickUtilTime')" prop="kickUtilTime">
          <el-date-picker
            v-model="kickForm.kickUtilTime"
            type="datetime"
            :placeholder="$t('account.kickUtilTime')"
            value-format="x"
            format="YYYY-MM-DD HH:mm:ss"
            :shortcuts="shortcutsKick"
            clearable
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button type="primary" @click="submitKickForm"
            >{{ $t("common.confirm") }}
          </el-button>
          <el-button @click="closeDialogKick"
            >{{ $t("common.cancel") }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      :title="qrCodeDialog.title"
      v-model="qrCodeDialog.visible"
      width="min(600px, calc(100vw - 24px))"
      :fullscreen="useFullscreenDialog"
      append-to-body
      @close="qrCodeDialog.visible = false"
    >
      <el-form style="text-align: center">
        <el-image
          class="qr-code-image"
          :src="qrCodeSrc"
        ></el-image>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button type="primary" @click="qrCodeDialog.visible = false"
            >{{ $t("common.confirm") }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script lang="ts">
export default {
  name: "index",
};
</script>

<script setup lang="ts">
import {
  AccountPageDto,
  AccountVo,
  KickAccountForm,
} from "@/api/account/types";
import {
  saveAccountApi,
  deleteAccountApi,
  getAccountApi,
  pageAccountApi,
  updateAccountApi,
  releaseKickAccountApi,
  resetTrafficApi,
  getAccountOutboundOptionsApi,
  generateConnectionPasswordApi,
  getConnectionPasswordApi,
  bindUnassignedOutboundsApi,
} from "@/api/account";
import { Document, Link, Search, Plus, Refresh, View } from "@element-plus/icons-vue";
import {
  timestampToCompactDateTime,
  timestampToFullDateTimeWithZone,
  getMonthLater,
  getWeekLater,
  calculateTimeDifference,
  getHourLater,
  getDayLater,
} from "@/utils/time";
import { formatTrafficG, quotaInputBytes, quotaInput } from "@/utils/account-quota";

import {
  hysteria2KickApi,
  hysteria2SubscribeUrlApi,
  hysteria2UrlApi,
  hysteria2RoutingStatusApi,
} from "@/api/hysteria2";
import { useI18n } from "vue-i18n";

import {
  Hysteria2SubscribeUrlDto,
  Hysteria2UrlDto,
  Hysteria2RoutingStatus,
} from "@/api/hysteria2/types";
import { copyText } from "@/utils/clipboard";
import { useRouter } from "vue-router";
import { accountExpiryAfterMonths, accountExpiryMonths, accountExpiryPickerDate, editAccountForm, newAccountForm, permanentAccountExpiry } from "./account-form";
import {
  AccountOutboundOption,
  buildAccountOutboundFormState,
  hasUnavailableBinding,
} from "./outbound-form";


const { t } = useI18n();
const router = useRouter();
const { width: viewportWidth } = useWindowSize();
const isMobile = computed(() => viewportWidth.value < 768);
const useFullscreenDialog = computed(() => viewportWidth.value < 520);
const accountSaving = ref(false);
const routingBindingsVisible = ref(false);
const routingBindingsSaving = ref(false);
const routingBindingUsers = ref<string[]>([]);
const routingBindings = reactive(new Map<string, string>());
const routingBindingsComplete = computed(() =>
  routingBindingUsers.value.length > 0 &&
  routingBindingUsers.value.every(username => routingBindings.has(username))
);
const passwordGenerating = ref(false);
const savedHomeProxyCredentialIncompatible = ref(false);
const legacyHomeProxyUsername = /^[-A-Za-z0-9+_.]+$/;
const passwordVisible = ref(false);
const passwordRevealing = ref(false);
const revealedConnectionPassword = ref("");
const outboundOptionsLoading = ref(false);
const outboundOptionsFailed = ref(false);
const outboundExcludedCount = ref(0);
let outboundOptionsRequest: Promise<void> | undefined;
const queryFormRef = ref(ElForm); // 查询表单
const dataFormRef = ref(ElForm); // 用户表单
const kickFormRef = ref(ElForm); // 下线表单

const dataFormAddRules = {
  remark: [
    {
      min: 0,
      max: 32,
      message: "Remark format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
  username: [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
    {
      pattern: /^[a-zA-Z0-9!@#$%^&*()_+-=]{2,32}$/,
      message: "Username format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
  conPass: [
    {
      pattern: /^[a-zA-Z0-9!@#$%^&*()_+-=]{6,32}$/,
      message: "ConPass format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
  expireTime: [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
  ],
  deviceNo: [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
  ],
  deleted: [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
  ],
};

const dataFormUpdateRules = {
  remark: [
    {
      min: 0,
      max: 32,
      message: "Remark format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
  username: [
    {
      validator: (_rule: unknown, value: string, done: (error?: Error) => void) => {
        const valid = /^[a-zA-Z0-9!@#$%^&*()_+-=]{2,32}$/.test(value);
        done(valid ? undefined : new Error("Username format is incorrect"));
      },
      trigger: ["change", "blur"],
    },
  ],
  conPass: [
    {
      pattern: /^[a-zA-Z0-9!@#$%^&*()_+-=]{6,32}$/,
      message: "Pass format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
};

const shortcutsKick = [
  {
    text: "A hour later",
    value: getHourLater,
  },
  {
    text: "A day later",
    value: getDayLater,
  },
  {
    text: "A week later",
    value: getWeekLater,
  },
  {
    text: "A month later",
    value: getMonthLater,
  },
];

const state = reactive({
  loading: true,
  total: 0,
  records: [] as AccountVo[],
  dialog: {
    visible: false,
  } as DialogType,
  dialogKick: {
    visible: false,
  } as DialogType,
  dataForm: newAccountForm(accountExpiryAfterMonths(1)),
  kickForm: {
    kickUtilTime: getHourLater(),
  } as KickAccountForm,
  queryParams: {
    remark: undefined,
    username: undefined,
    deleted: undefined,
    pageNum: 1,
    pageSize: 10,
  } as AccountPageDto,
  quotaTmp: 0 as number | undefined,
  unlimitedQuota: false,
  qrCodeDialog: {
    title: "QR Code",
    visible: false,
  } as DialogType,
  qrCodeSrc: "",
  outboundOptions: [] as AccountOutboundOption[],
  routingStatus: {
    enabled: false,
    ready: false,
    activeUsers: 0,
    boundUsers: 0,
    unboundUsers: [],
    errors: [],
    kernelVersion: "-",
    kernelCompatible: false,
  } as Hysteria2RoutingStatus,
});

const {
  loading,
  total,
  records,
  dialog,
  dialogKick,
  dataForm,
  kickForm,
  queryParams,
  quotaTmp,
  unlimitedQuota,
  qrCodeDialog,
  qrCodeSrc,
  outboundOptions,
  routingStatus,
} = toRefs(state);

const routingStatusDescription = computed(() => {
  const parts = [
    `${t("account.boundUsers")}: ${state.routingStatus.boundUsers}/${state.routingStatus.activeUsers}`,
    `${t("account.kernelVersion")}: ${state.routingStatus.kernelVersion}`,
  ];
  if (state.routingStatus.unboundUsers.length) {
    parts.push(
      `${t("account.unboundUsers")}: ${state.routingStatus.unboundUsers.join(", ")}`
    );
  }
  if (state.routingStatus.errors.length) {
    parts.push(state.routingStatus.errors.join("; "));
  }
  return parts.join(" · ");
});

const relayAccount = ref(false);
const unavailableOutboundOptions = ref<AccountOutboundOption[]>([]);
const ordinaryOutboundOptions = computed(() => buildAccountOutboundFormState(state.outboundOptions, "user").options);
const changeAccountKind = () => {
  state.dataForm.outbound = "";
  creationBindings.clear();
};
const outboundFormState = computed(() =>
  buildAccountOutboundFormState(
    state.outboundOptions,
    state.dataForm.role,
    relayAccount.value
  )
);
const relayBindingReady = computed(() =>
  !relayAccount.value || outboundFormState.value.options.some(item => item.name === state.dataForm.outbound)
);
const drawerAccount = computed(() =>
  state.records.find(item => item.id === state.dataForm.id)
);
const unavailableBinding = computed(() => hasUnavailableBinding(state.outboundOptions, state.dataForm.outbound));

const creationBindings = reactive(new Map<string, string>());
const creationBindingUsers = computed(() =>
  state.dataForm.id === undefined && state.dataForm.deleted === 0 && state.dataForm.outbound
    ? state.routingStatus.unboundUsers
    : []
);
const creationBindingsComplete = computed(() =>
  creationBindingUsers.value.every(username => !!creationBindings.get(username))
);

let accountEditorRequest = 0;

const loadOutboundOptions = (): Promise<void> => {
  if (outboundOptionsRequest) return outboundOptionsRequest;
  outboundOptionsLoading.value = true;
  outboundOptionsRequest = (async () => {
    try {
      const { data } = await getAccountOutboundOptionsApi();
      state.outboundOptions = data.options;
      unavailableOutboundOptions.value = data.unavailable;
      outboundExcludedCount.value = data.unavailable.length;
      outboundOptionsFailed.value = false;
    } catch {
      state.outboundOptions = [];
      outboundOptionsFailed.value = true;
    } finally {
      outboundOptionsLoading.value = false;
      outboundOptionsRequest = undefined;
    }
  })();
  return outboundOptionsRequest;
};
const refreshOutboundOptions = (visible: boolean) => { if (visible) void loadOutboundOptions(); };

const openHysteriaSettings = () => {
  closeDialog();
  router.push({ name: "XrayOutbound" });
};

const loadRoutingStatus = async () => {
  const { data } = await hysteria2RoutingStatusApi();
  state.routingStatus = data;
};

const openRoutingBindings = async () => {
  await Promise.all([loadRoutingStatus(), loadOutboundOptions()]);
  if (outboundOptionsFailed.value || !state.routingStatus.unboundUsers.length) return;
  routingBindings.clear();
  routingBindingUsers.value = [...state.routingStatus.unboundUsers];
  routingBindingsVisible.value = true;
};

const saveRoutingBindings = async () => {
  if (!routingBindingsComplete.value || routingBindingsSaving.value) return;
  routingBindingsSaving.value = true;
  try {
    const { data } = await bindUnassignedOutboundsApi(Object.fromEntries(routingBindings));
    if (data.outboundChanged && !data.runtimeApplied) {
      ElMessage.warning(t("account.outboundSavedHy2Stopped"));
    } else {
      ElMessage.success(t("common.success"));
    }
    routingBindingsVisible.value = false;
    await Promise.all([handleQuery(), loadRoutingStatus()]);
  } finally {
    routingBindingsSaving.value = false;
  }
};

/**
 * 查询
 */
const handleQuery = async () => {
  state.loading = true;
  try {
    const { data } = await pageAccountApi(state.queryParams);
    state.records = data.records;
    state.total = data.total;
  } finally {
    state.loading = false;
  }
};

/**
 * 重置
 */
const resetQuery = () => {
  queryFormRef.value.resetFields();
  handleQuery();
};

/**
 * 保存
 **/
const accountExpiryDate = computed({
  get: () => accountExpiryPickerDate(state.dataForm.expireTime),
  set: (value: Date | undefined) => {
    if (value instanceof Date && Number.isFinite(value.getTime())) state.dataForm.expireTime = value.getTime();
  },
});
const connectionPasswordDisplay = computed(() => {
  const form = state.dataForm;
  if (form.conPass || form.id === undefined) return form.conPass;
  return passwordVisible.value ? revealedConnectionPassword.value : "••••••••";
});

let passwordRequest = 0;
const togglePasswordVisibility = async () => {
  if (passwordGenerating.value || passwordRevealing.value) return;
  if (passwordVisible.value) {
    passwordVisible.value = false;
    revealedConnectionPassword.value = "";
    return;
  }
  const { id, conPass } = state.dataForm;
  if (conPass) {
    passwordVisible.value = true;
    return;
  }
  if (id === undefined) return;

  const request = ++passwordRequest;
  passwordRevealing.value = true;
  try {
    const { data } = await getConnectionPasswordApi({ id });
    if (request === passwordRequest && dialog.value.visible && state.dataForm.id === id) {
      revealedConnectionPassword.value = data;
      passwordVisible.value = true;
    }
  } catch {
    // The API interceptor reports the error.
  } finally {
    if (request === passwordRequest) passwordRevealing.value = false;
  }
};
const generatePassword = async () => {
  if (passwordGenerating.value || accountSaving.value) return;
  const request = ++passwordRequest;
  const accountID = state.dataForm.id;
  passwordGenerating.value = true;
  try {
    const { data } = await generateConnectionPasswordApi();
    if (request === passwordRequest && dialog.value.visible && state.dataForm.id === accountID) {
      state.dataForm.conPass = data;
    }
  } catch {
    if (request === passwordRequest && dialog.value.visible) {
      ElMessage.error(t("account.generatePasswordFailed"));
    }
  } finally {
    if (request === passwordRequest) passwordGenerating.value = false;
  }
};

const handleAdd = async () => {
  const request = ++accountEditorRequest;
  await Promise.all([loadOutboundOptions(), loadRoutingStatus().catch(() => {})]);
  if (request !== accountEditorRequest) return;
  savedHomeProxyCredentialIncompatible.value = false;
  state.dataForm = newAccountForm(accountExpiryAfterMonths(1));
  relayAccount.value = false;
  creationBindings.clear();
  quotaTmp.value = 0;
  unlimitedQuota.value = false;
  state.dialog = {
    title: t("common.add"),
    visible: true,
  };
  void generatePassword();
};

/**
 * 修改
 **/
const handleUpdate = async (row: AccountVo) => {
  const request = ++accountEditorRequest;
  const [, { data }] = await Promise.all([loadOutboundOptions(), getAccountApi({ id: row.id })]);
  if (request !== accountEditorRequest) return;
  state.dataForm = editAccountForm(data);
  relayAccount.value = [...state.outboundOptions, ...unavailableOutboundOptions.value].some(item =>
    item.type === "udprelay" && item.name.toLowerCase() === data.outbound?.toLowerCase()
  );
  savedHomeProxyCredentialIncompatible.value = data.homeProxyCredentialIncompatible === true;
  const input = quotaInput(data.quota);
  unlimitedQuota.value = input < 0;
  quotaTmp.value = Math.max(0, input);
  dialog.value = {
    title: t("common.update"),
    visible: true,
  };
};

/**
 * 表单提交
 */
const submitForm = async () => {
  const request = accountEditorRequest;
  const valid = await dataFormRef.value?.validate().catch(() => false);
  if (!valid || request !== accountEditorRequest || passwordGenerating.value || accountSaving.value || !creationBindingsComplete.value) return;
  if (!relayBindingReady.value) {
    ElMessage.error(t("account.relayBindingRequired"));
    return;
  }
  const quotaValue = unlimitedQuota.value ? -1 : quotaTmp.value;
  const unchangedQuota = state.dataForm.id && quotaValue === quotaInput(state.dataForm.quota);
  const quota = unchangedQuota ? undefined : quotaInputBytes(quotaValue);
  if (!unchangedQuota && quota === undefined) {
    ElMessage.error(t("account.quotaGBInvalid"));
    return;
  }
  accountSaving.value = true;
  try {
    const { id, role, uploadMbps, downloadMbps, conPass, ...fields } = state.dataForm;
    const bandwidth = role === "admin" ? {} : { uploadMbps, downloadMbps };
    const { data } = id !== undefined
      ? await updateAccountApi({ ...fields, ...bandwidth, id, quota, conPass: conPass || undefined })
      : await saveAccountApi({
          ...fields,
          ...bandwidth,
          conPass,
          quota: quota!,
          existingBindings: Object.fromEntries(creationBindingUsers.value.map(username => [username, creationBindings.get(username)!])),
        });
    if (data.outboundChanged && !data.runtimeApplied) {
      ElMessage.warning(t("account.outboundSavedHy2Stopped"));
    } else if (data.bandwidthStatus && data.bandwidthStatus !== "applied" && (uploadMbps || downloadMbps || data.bandwidthStatus === "unconfirmed")) {
      ElMessage.warning(t("account.bandwidthNotApplied"));
    } else {
      ElMessage.success(t("common.success"));
    }
    closeDialog();
    await Promise.all([handleQuery(), loadRoutingStatus()]);
  } catch {
    // The API interceptor reports the error. Refresh accounts needing a binding
    // while keeping the current form and choices available for retry.
    await loadRoutingStatus().catch(() => {});
  } finally {
    accountSaving.value = false;
  }
};

/**
 * 下线表单提交
 */
const submitKickForm = () => {
  kickFormRef.value.validate((valid: any) => {
    if (valid) {
      const params = { ...state.kickForm };
      hysteria2KickApi(params).then(() => {
        ElMessage.success(t("common.success"));
        closeDialogKick();
        handleQuery();
      });
    }
  });
};

/**
 * 删除
 */
const handleDelete = (row: { [key: string]: any }) => {
  const id = row.id;
  const username = row.username;
  ElMessageBox.confirm(
    "Are you sure to delete the data item with the username「" +
      username +
      "」?",
    "Warning",
    {
      confirmButtonText: t("common.confirm"),
      cancelButtonText: t("common.cancel"),
      type: "warning",
    }
  )
    .then(() => {
      deleteAccountApi({ id: id }).then(({ data }) => {
        if (data.outboundChanged && !data.runtimeApplied) {
          ElMessage.warning(t("account.outboundSavedHy2Stopped"));
        } else {
          ElMessage.success(t("common.success"));
        }
        handleQuery();
        loadRoutingStatus();
        if (state.dataForm.id === id) closeDialog();
      });
    })
    .catch(() => ElMessage.info(t("common.cancel")));
};

/**
 * 强制用户下线
 * @param row
 */
const handleKick = (row: { [key: string]: any }) => {
  state.kickForm.ids = [row.id];
  dialogKick.value = {
    title: t("account.kickTip"),
    visible: true,
  };
};

/**
 * 解除下线状态
 * @param row
 */
const confirmReleaseKick = (row: { [key: string]: any }) => {
  releaseKickAccountApi({ id: row.id }).then(() => {
    ElMessage.success(t("account.releaseSuccess"));
    handleQuery();
  });
};

const confirmResetTraffic = (row: { [key: string]: any }) => {
  ElMessageBox.confirm(
    t("account.resetTrafficConfirm"),
    t("common.confirm"),
    {
      confirmButtonText: t("common.confirm"),
      cancelButtonText: t("common.cancel"),
      type: "warning",
    }
  ).then(() => resetTraffic(row));
};

const confirmReleaseKickWithPrompt = (row: { [key: string]: any }) => {
  ElMessageBox.confirm(
    t("account.releaseKickTip"),
    t("common.confirm"),
    {
      confirmButtonText: t("common.confirm"),
      cancelButtonText: t("common.cancel"),
      type: "warning",
    }
  ).then(() => confirmReleaseKick(row));
};

/**
 * 关闭用户弹窗
 */
const closeDialog = () => {
  dialog.value.visible = false;
};

watch(() => dialog.value.visible, visible => {
  if (visible) return;
  accountEditorRequest++;
  passwordRequest++;
  passwordGenerating.value = false;
  passwordRevealing.value = false;
  passwordVisible.value = false;
  revealedConnectionPassword.value = "";
  state.dataForm = newAccountForm(accountExpiryAfterMonths(1));
  creationBindings.clear();
}, { flush: "sync" });

/**
 * 关闭下线弹窗
 */
const closeDialogKick = () => {
  dialogKick.value.visible = false;
  kickFormRef.value.resetFields();
  kickFormRef.value.clearValidate();
};

const handleSubscribe = async (row: { [key: string]: any }) => {
  try {
    const dto: Hysteria2SubscribeUrlDto = {
      accountId: row.id,
      protocol: window.location.protocol,
      host: window.location.host,
    };
    const { data } = await hysteria2SubscribeUrlApi(dto);
    if (await copyText(data.url)) ElMessage.success(t("common.copySuccess"));
    else ElMessage.error(t("common.copyFailed"));
  } catch (e) {
    /* empty */
  }
};

const handleNodeUrl = async (row: { [key: string]: any }) => {
  try {
    const dto: Hysteria2UrlDto = {
      accountId: row.id,
      hostname: window.location.hostname,
    };
    const { data } = await hysteria2UrlApi(dto);
    if (await copyText(data.url)) ElMessage.success(t("common.copySuccess"));
    else ElMessage.error(t("common.copyFailed"));
  } catch (e) {
    /* empty */
  }
};

const handleQrCode = async (row: { [key: string]: any }) => {
  try {
    const dto: Hysteria2UrlDto = {
      accountId: row.id,
      hostname: window.location.hostname,
    };
    const { data } = await hysteria2UrlApi(dto);
    state.qrCodeSrc = "data:image/png;base64," + data.qrCode;
    state.qrCodeDialog.visible = true;
  } catch (e) {
    /* empty */
  }
};

const resetTraffic = async (row: { [key: string]: any }) => {
  try {
    await resetTrafficApi({ id: row.id });
    ElMessage.success(t("common.success"));
    await handleQuery();
  } catch (e) {
    /* empty */
  }
};

onMounted(() => {
  // 初始化用户列表数据
  handleQuery();
  loadOutboundOptions();
  loadRoutingStatus();

});
</script>

<style lang="scss" scoped>
.accounts-page {
  display: grid;
  gap: 18px;
}

.accounts-panel { min-width: 0; overflow: hidden; }

.account-filters {
  padding: 14px 16px;

  :deep(.el-form) {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 11px;
  }

  :deep(.el-form-item) {
    display: flex;
    flex: 1 1 140px;
    max-width: 210px;
    flex-direction: column;
    margin: 0;
  }

  :deep(.el-form-item__label) {
    justify-content: flex-start;
    height: auto;
    margin: 0 0 3px;
    color: var(--hui-muted);
    line-height: 1.4;
    font-size: 12px;
  }

  :deep(.el-input),
  :deep(.el-select) { width: 100%; }
}

.filter-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;

  .el-button { margin: 0; }
}

.routing-strip {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 5px 14px;
  padding: 10px 16px;
  border-block-start: 1px solid var(--hui-border);
  color: var(--hui-muted);
  font-size: 12px;
}

.routing-detail { overflow-wrap: anywhere; min-width: 0; }

.account-identity {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}

.account-avatar {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  flex: none;
  border-radius: 9px;
  background: var(--hui-accent-soft);
  color: var(--hui-accent-ink);
  font-weight: 700;
}

.account-identity-details { display: grid; gap: 2px; min-width: 0; }
.account-summary { color: var(--hui-muted); line-height: 1.35; overflow-wrap: anywhere; }

.account-name {
  justify-self: start;
  max-width: 100%;
  overflow: hidden;
  padding: 0;
  background: transparent;
  color: var(--hui-ink);
  font: inherit;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.account-links {
  display: flex;
  justify-content: flex-end;
  gap: 2px;

  .el-button {
    width: 30px;
    padding: 0;
    margin: 0;
    color: var(--hui-muted);
  }

  .el-button:hover { color: var(--hui-accent-ink); background: var(--hui-accent-soft); }
}

.account-qr-icon {
  display: grid;
  grid-template-columns: repeat(2, 5px);
  gap: 2px;

  i {
    width: 5px;
    height: 5px;
    border: 1px solid currentColor;
  }

  i:last-child { border-style: dotted; }
}

.outbound-name {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  padding: 2px 8px;
  border-radius: 6px;
  background: var(--hui-hover);
  color: var(--hui-ink);
  font-size: 12px;
  line-height: 1.5;
  vertical-align: middle;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.outbound-name.unbound { color: var(--hui-muted); }

.time-cell { white-space: nowrap; font-variant-numeric: tabular-nums; }
.qr-code-image { width: min(300px, 100%); height: auto; aspect-ratio: 1; }
:deep(.account-table .el-table__row) { cursor: pointer; }

.accounts-pagination { padding: 10px 14px; }
.accounts-pagination :deep(.el-pagination) { flex-wrap: wrap; row-gap: 6px; }

:global(.account-drawer .el-drawer__header) {
  margin: 0;
  padding: 18px 22px;
  border-bottom: 1px solid var(--hui-border);
}
:global(.account-drawer .el-drawer__title) { font-weight: 700; color: var(--hui-ink); }
:global(.account-drawer .el-drawer__body) { padding: 18px 22px; }
:global(.account-drawer .el-drawer__footer) {
  padding: 13px 22px;
  border-top: 1px solid var(--hui-border);
}

.account-usage { margin-bottom: 18px; }
.account-expiry-presets { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
.account-expiry-presets .el-button + .el-button { margin-left: 0; }

.drawer-danger-zone {
  padding: 14px;
  margin-top: 18px;
  background: var(--el-color-danger-light-9);
  border: 1px solid var(--el-color-danger-light-7);
  border-radius: var(--hui-radius-md);
}

.drawer-section-title { margin-bottom: 10px; font-size: 13px; font-weight: 650; }
.drawer-footer { display: flex; justify-content: flex-end; gap: 8px; }

@media (width <= 767px) {
  .account-filters { padding: 12px; }
  .account-filters :deep(.el-form-item) { max-width: none; }
  .account-avatar { width: 26px; height: 26px; }
  .routing-strip { padding: 9px 12px; }
  :global(.account-drawer .el-drawer__body) { padding: 14px; }
}
</style>
