<template>
  <div class="app-container">
    <div class="search">
      <el-form ref="queryFormRef" :model="queryParams" :inline="true">
        <el-form-item :label="$t('account.remark')" prop="remark">
          <el-input
            v-model="queryParams.remark"
            :placeholder="$t('account.remark')"
            clearable
            style="width: 200px"
            @keyup.enter="handleQuery"
          />
        </el-form-item>
        <el-form-item :label="$t('account.username')" prop="username">
          <el-input
            v-model="queryParams.username"
            :placeholder="$t('account.username')"
            clearable
            style="width: 200px"
            @keyup.enter="handleQuery"
          />
        </el-form-item>
        <el-form-item :label="$t('common.deleted')" prop="deleted">
          <el-select
            v-model="queryParams.deleted"
            :placeholder="$t('common.all')"
            clearable
            style="width: 200px"
          >
            <el-option :label="$t('common.enable')" value="0" />
            <el-option :label="$t('common.disable')" value="1" />
          </el-select>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleQuery"
            >{{ $t("common.search") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button :icon="Refresh" @click="resetQuery"
            >{{ $t("common.reset") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button :icon="Plus" @click="handleAdd"
            >{{ $t("common.add") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-upload
            v-model:file-list="fileList"
            :http-request="handleImport"
            :show-file-list="false"
            accept=".json"
            :limit="1"
            :before-upload="beforeImport"
          >
            <el-button>
              <template #icon>
                <i-ep-upload />
              </template>
              {{ $t("common.import") }}
            </el-button>
          </el-upload>
        </el-form-item>
        <el-form-item>
          <el-button @click="handleExport">
            <template #icon>
              <i-ep-download />
            </template>
            {{ $t("common.export") }}
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-alert
      :type="routingStatus.ready ? 'success' : 'warning'"
      :closable="false"
      show-icon
      :title="
        routingStatus.ready
          ? $t('account.routingReady')
          : $t('account.routingNotReady')
      "
      :description="routingStatusDescription"
      style="margin-bottom: 16px"
    />

    <el-card shadow="never">
      <el-table v-loading="loading" :data="records" class="account-table">
        <el-table-column
          key="username"
          :label="$t('account.username')"
          align="left"
          prop="username"
          min-width="130"
          fixed="left"
          show-overflow-tooltip
        />
        <el-table-column
          key="status"
          :label="$t('account.onlineStatus')"
          align="center"
          width="100"
        >
          <template #default="scope">
            <el-tag v-if="scope.row.deleted !== 0" type="info">{{ $t("common.disable") }}</el-tag>
            <el-tag v-else-if="scope.row.online" type="success">{{ $t("account.online") }}</el-tag>
            <el-tag v-else type="info">{{ $t("account.offline") }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column
          key="outbound"
          :label="$t('account.outbound')"
          align="center"
          prop="outbound"
          min-width="140"
          show-overflow-tooltip
        >
          <template #default="scope">
            <el-tag v-if="scope.row.outbound" type="success">
              {{ scope.row.outbound }}
            </el-tag>
            <el-tag v-else type="info">{{ $t("account.unbound") }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column key="traffic" :label="$t('account.trafficUsed')" align="center" min-width="130">
          <template #default="scope">
            <el-tooltip :content="`${$t('account.upload')}: ${formatBytes(scope.row.upload)} · ${$t('account.download')}: ${formatBytes(scope.row.download)}`">
              <span>{{ formatBytes(scope.row.upload + scope.row.download) }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column
          key="expireTime"
          :label="$t('account.expireTime')"
          align="center"
          prop="expireTime"
          width="132"
        >
          <template #default="scope">
            <el-tooltip
              v-if="scope.row.expireTime"
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
        <el-table-column
          :label="$t('common.operate')"
          align="center"
          width="150"
          fixed="right"
        >
          <template #default="scope">
            <el-button type="primary" link @click.stop="handleUpdate(scope.row)"
              >{{ $t("common.edit") }}
            </el-button>
            <el-dropdown
              trigger="click"
              @click.stop
              @command="handleAccountCommand($event, scope.row)"
            >
              <el-button type="primary" link @click.stop>
                {{ $t("common.more") }}<i-ep-arrow-down />
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="subscribe">{{
                    $t("common.subscribe")
                  }}</el-dropdown-item>
                  <el-dropdown-item command="node-url">{{
                    $t("common.nodeUrl")
                  }}</el-dropdown-item>
                  <el-dropdown-item command="node-qr">{{
                    $t("common.nodeQrCode")
                  }}</el-dropdown-item>
                  <el-dropdown-item command="reset-traffic" divided>{{
                    $t("common.resetTraffic")
                  }}</el-dropdown-item>
                  <el-dropdown-item command="kick">{{
                    $t("account.kick")
                  }}</el-dropdown-item>
                  <el-dropdown-item
                    v-if="
                      calculateTimeDifference(scope.row.kickUtilTime) !== '-'
                    "
                    command="release-kick"
                  >
                    {{ $t("account.releaseKick") }}
                  </el-dropdown-item>
                  <el-dropdown-item command="delete" divided>
                    {{ $t("common.delete") }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>

      <pagination
        v-if="total > 0"
        :total="total"
        v-model:page="queryParams.pageNum"
        v-model:limit="queryParams.pageSize"
        @pagination="handleQuery"
      />
    </el-card>

    <el-drawer
      :title="dialog.title"
      v-model="dialog.visible"
      :size="isMobile ? '100%' : '500px'"
      class="account-drawer"
      append-to-body
      destroy-on-close
      :close-on-click-modal="true"
      @close="closeDialog"
    >
      <el-form
        ref="dataFormRef"
        :rules="
          dialog.title === t('common.add')
            ? dataFormAddRules
            : dataFormUpdateRules
        "
        :model="dataForm"
        label-position="top"
      >
        <el-alert
          v-if="drawerAccount"
          class="account-usage"
          type="info"
          :closable="false"
          :title="`${$t('account.trafficUsed')}: ${formatBytes(drawerAccount.upload + drawerAccount.download)} · ${$t('account.device')}: ${drawerAccount.device}/${drawerAccount.deviceNo}`"
        />
        <el-form-item :label="$t('account.username')" prop="username">
          <el-input
            v-model="dataForm.username"
            :placeholder="$t('account.username')"
            maxlength="50"
            clearable
          />
        </el-form-item>
        <el-form-item
          v-if="dataForm.role === 'admin'"
          :label="$t('account.pass')"
          prop="pass"
        >
          <el-input
            v-model="dataForm.pass"
            :placeholder="$t('account.pass')"
            maxlength="50"
            clearable
            type="password"
            show-password
            ref="dataFormPassRef"
          />
        </el-form-item>
        <el-form-item :label="$t('account.conPass')" prop="conPass">
          <el-input
            v-model="dataForm.conPass"
            :placeholder="$t('account.conPass')"
            maxlength="50"
            clearable
            type="password"
            show-password
            ref="dataFormConPassRef"
          />
        </el-form-item>
        <el-form-item :label="$t('account.outbound')" prop="outbound">
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
          <div v-else-if="outboundFormState.noOptions" class="form-tip">
            {{ $t("account.outboundUnavailable") }}
            <el-button link type="primary" @click="openHysteriaSettings">
              {{ $t("account.configureOutbounds") }}
            </el-button>
          </div>
          <div v-else class="form-tip">{{ $t("account.outboundTip") }}</div>
          <el-alert v-if="outboundOptionsFailed" type="error" :closable="false" :title="$t('account.outboundLoadFailed')" />
          <el-alert v-else-if="!outboundOptionsLoading && unavailableBinding" type="warning" :closable="false" :title="$t('account.outboundBindingUnavailable', { name: dataForm.outbound })" />
          <div v-if="outboundExcludedCount > 0" class="form-tip">{{ $t('account.outboundHistoricalExcluded') }}</div>
        </el-form-item>
        <el-form-item :label="$t('account.quota')" prop="quota">
          <unit-select :setValue="setQuota" :valueTmp="quotaTmp" />
        </el-form-item>
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
            v-model="dataForm.expireTime"
            type="datetime"
            :placeholder="$t('account.expireTime')"
            value-format="x"
            format="YYYY-MM-DD HH:mm:ss"
            :shortcuts="shortcuts"
            clearable
            style="width: 100%"
          />
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

      <div v-if="drawerAccount" class="drawer-secondary-actions">
        <div class="drawer-section-title">{{ $t("account.accountTools") }}</div>
        <el-space wrap>
          <el-button @click="handleSubscribe(drawerAccount)">{{ $t("common.subscribe") }}</el-button>
          <el-button @click="handleNodeUrl(drawerAccount)">{{ $t("common.nodeUrl") }}</el-button>
          <el-button @click="handleQrCode(drawerAccount)">{{ $t("common.nodeQrCode") }}</el-button>
        </el-space>
      </div>
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
          <el-button @click="closeDialog">{{ $t("common.cancel") }}</el-button>
          <el-button type="primary" :loading="accountSaving" :disabled="accountSaving" @click="submitForm">{{ $t("common.save") }}</el-button>
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
  AccountForm,
  AccountPageDto,
  AccountUpdateDto,
  AccountVo,
  KickAccountForm,
} from "@/api/account/types";
import {
  saveAccountApi,
  deleteAccountApi,
  getAccountApi,
  pageAccountApi,
  updateAccountApi,
  exportAccountApi,
  releaseKickAccountApi,
  importAccountApi,
  resetTrafficApi,
  getAccountOutboundOptionsApi,
} from "@/api/account";
import { Search, Plus, Refresh } from "@element-plus/icons-vue";
import {
  timestampToCompactDateTime,
  timestampToFullDateTimeWithZone,
  getMonthLater,
  getWeekLater,
  getYearLater,
  calculateTimeDifference,
  getHourLater,
  getDayLater,
} from "@/utils/time";
import { formatBytes } from "@/utils/byte";

import {
  hysteria2KickApi,
  hysteria2SubscribeUrlApi,
  hysteria2UrlApi,
  hysteria2RoutingStatusApi,
} from "@/api/hysteria2";
import {
  UploadFile,
  UploadRawFile,
  UploadRequestOptions,
} from "element-plus/lib/components";
import { useI18n } from "vue-i18n";

import {
  Hysteria2SubscribeUrlDto,
  Hysteria2UrlDto,
  Hysteria2RoutingStatus,
} from "@/api/hysteria2/types";
import copy from "copy-to-clipboard";
import { useRoute, useRouter } from "vue-router";
import {
  AccountOutboundOption,
  buildAccountOutboundFormState,
  newAccountOutboundFields,
  hasUnavailableBinding,
} from "./outbound-form";

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const { width: viewportWidth } = useWindowSize();
const isMobile = computed(() => viewportWidth.value < 768);
const useFullscreenDialog = computed(() => viewportWidth.value < 520);
const accountSaving = ref(false);
const outboundOptionsLoading = ref(false);
const outboundOptionsFailed = ref(false);
const outboundExcludedCount = ref(0);
let outboundOptionsRequest: Promise<void> | undefined;
const queryFormRef = ref(ElForm); // 查询表单
const dataFormRef = ref(ElForm); // 用户表单
const kickFormRef = ref(ElForm); // 下线表单
const dataFormPassRef = ref();
const dataFormConPassRef = ref();

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
      pattern: /^[a-zA-Z0-9!@#$%^&*()_+-=]{6,32}$/,
      message: "Username format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
  conPass: [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
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
      pattern: /^[a-zA-Z0-9!@#$%^&*()_+-=]{6,32}$/,
      message: "Username format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
  pass: [
    {
      pattern: /^[a-zA-Z0-9!@#$%^&*()_+-=]{6,32}$/,
      message: "Pass format is incorrect",
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

const shortcuts = [
  {
    text: "A week later",
    value: getWeekLater,
  },
  {
    text: "A month later",
    value: getMonthLater,
  },
  {
    text: "A year later",
    value: getYearLater,
  },
];

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
  dataForm: {
    quota: 0,
    expireTime: getMonthLater(),
    deviceNo: 6,
    deleted: 0,
    ...newAccountOutboundFields(),
  } as AccountForm,
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
  quotaTmp: 0,
  fileList: [] as UploadFile[],
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
  fileList,
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

const outboundFormState = computed(() =>
  buildAccountOutboundFormState(
    state.outboundOptions,
    state.dataForm.role
  )
);
const drawerAccount = computed(() =>
  state.records.find(item => item.id === state.dataForm.id)
);
const unavailableBinding = computed(() => hasUnavailableBinding(state.outboundOptions, state.dataForm.outbound));

const resetDataForm = () => {
  Object.assign(state.dataForm, {
    id: undefined,
    quota: 0,
    expireTime: getMonthLater(),
    deleted: 0,
    ...newAccountOutboundFields(),
  });
  quotaTmp.value = 0;
};

const loadOutboundOptions = (): Promise<void> => {
  if (outboundOptionsRequest) return outboundOptionsRequest;
  outboundOptionsLoading.value = true;
  outboundOptionsRequest = (async () => {
    try {
      const { data } = await getAccountOutboundOptionsApi();
      state.outboundOptions = data.options;
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
const handleAdd = async () => {
  resetDataForm();
  await loadOutboundOptions();
  state.dialog = {
    title: t("common.add"),
    visible: true,
  };
};

/**
 * 修改
 **/
const handleUpdate = async (row: { [key: string]: any }) => {
  await loadOutboundOptions();
  const id = row.id;
  const { data } = await getAccountApi({ id: id });
  Object.assign(state.dataForm, data);
  quotaTmp.value = data.quota;
  dialog.value = {
    title: t("common.update"),
    visible: true,
  };
};

const setQuota = (newQuota: number) => {
  state.dataForm.quota = newQuota;
};

/**
 * 表单提交
 */
const submitForm = async () => {
  const valid = await dataFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  accountSaving.value = true;
  try {
    const accountUpdateDto: AccountUpdateDto = { ...state.dataForm };
    const { data } = state.dataForm.id
      ? await updateAccountApi(accountUpdateDto)
      : await saveAccountApi(accountUpdateDto);
    if (data.outboundChanged && !data.runtimeApplied) {
      ElMessage.warning(t("account.outboundSavedHy2Stopped"));
    } else {
      ElMessage.success(t("common.success"));
    }
    closeDialog();
    await Promise.all([handleQuery(), loadRoutingStatus()]);
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

const handleAccountCommand = (
  command: string,
  row: { [key: string]: any }
) => {
  switch (command) {
    case "subscribe":
      handleSubscribe(row);
      break;
    case "node-url":
      handleNodeUrl(row);
      break;
    case "node-qr":
      handleQrCode(row);
      break;
    case "reset-traffic":
      confirmResetTraffic(row);
      break;
    case "kick":
      handleKick(row);
      break;
    case "release-kick":
      confirmReleaseKickWithPrompt(row);
      break;
    case "delete":
      handleDelete(row);
      break;
  }
};

/**
 * 关闭用户弹窗
 */
const closeDialog = () => {
  dialog.value.visible = false;
  dataFormRef.value?.resetFields();
  dataFormRef.value?.clearValidate();

  if (dialog.value.title == t("common.update")) {
    resetDataForm();
  }
};

/**
 * 关闭下线弹窗
 */
const closeDialogKick = () => {
  dialogKick.value.visible = false;
  kickFormRef.value.resetFields();
  kickFormRef.value.clearValidate();
};

/**
 * 导入
 */
const handleImport = (params: UploadRequestOptions) => {
  if (state.fileList.length > 0) {
    let formData = new FormData();
    formData.append("file", params.file);
    importAccountApi(formData).then(() => {
      ElMessage.success(t("common.success"));
    });
    state.fileList = [];
  }
};

const beforeImport = (file: UploadRawFile) => {
  if (!file.name.endsWith(".json")) {
    ElMessage.error("file format not supported");
    return false;
  }
  if (file.size / 1024 / 1024 > 2) {
    ElMessage.error("the file is too big, less than 2 MB");
    return false;
  }
};

/**
 * 导出
 */
const handleExport = () => {
  exportAccountApi().then((res) => {
    const blob = new Blob([res.data], {
      type: "application/octet-stream",
    });
    let url = window.URL.createObjectURL(blob);
    let a = document.createElement("a");
    document.body.appendChild(a);
    a.href = url;
    let dis = res.headers["content-disposition"];
    a.download = dis.split("attachment; filename=")[1];
    // 模拟点击下载
    a.click();
    window.URL.revokeObjectURL(url);
    ElMessage.success(t("common.success"));
  });
};

const handleSubscribe = async (row: { [key: string]: any }) => {
  try {
    const dto: Hysteria2SubscribeUrlDto = {
      accountId: row.id,
      protocol: window.location.protocol,
      host: window.location.host,
    };
    const { data } = await hysteria2SubscribeUrlApi(dto);
    copy(data.url);
    ElMessage.success(t("common.copySuccess"));
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
    copy(data.url);
    ElMessage.success(t("common.copySuccess"));
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
  if (route.query.focus === "change-pass") {
    nextTick(() => {
      handleUpdate({ id: 1 }).then(() => {
        setTimeout(() => {
          const inputPass = dataFormPassRef.value.$el.querySelector(
            ".el-input__wrapper input"
          );
          if (inputPass) {
            inputPass.focus();
          }
        }, 50);
      });
    });
  }
});
</script>

<style lang="scss" scoped>
.time-cell {
  white-space: nowrap;
  cursor: help;
}

.qr-code-image {
  width: min(300px, 100%);
  height: auto;
  aspect-ratio: 1;
}

:deep(.account-table .el-button + .el-dropdown) {
  margin-left: 8px;
}

:deep(.account-table .el-table__row) {
  cursor: pointer;
}

.account-usage {
  margin-bottom: 20px;
}

.drawer-secondary-actions,
.drawer-danger-zone {
  padding-top: 18px;
  margin-top: 18px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.drawer-danger-zone {
  padding: 16px;
  background: var(--el-color-danger-light-9);
  border: 1px solid var(--el-color-danger-light-7);
  border-radius: var(--hui-radius-md);
}

.drawer-section-title {
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 600;
}

.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (width <= 767px) {
  .app-container {
    padding: 12px;
  }

  .search :deep(.el-form-item) {
    width: 100%;
    margin-right: 0;
  }

  .search :deep(.el-form-item__content),
  .search :deep(.el-input),
  .search :deep(.el-select) {
    width: 100% !important;
  }

  :deep(.account-drawer .el-drawer__body) {
    padding: 14px;
  }
}
</style>
