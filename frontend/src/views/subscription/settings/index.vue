<template>
  <div class="app-container">
    <el-card>
      <template #header>
        <span>{{ $t("route.subscriptionSettings") }}</span>
      </template>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item
          :label="$t('subscriptionTemplate.nodeName')"
          prop="nodeNameTemplate"
        >
          <el-input
            v-model="form.nodeNameTemplate"
            maxlength="256"
          />
        </el-form-item>
        <el-form-item
          :label="$t('subscriptionTemplate.serverAddress')"
          prop="serverAddressTemplate"
        >
          <el-radio-group v-model="addressMode" @change="onAddressModeChange">
            <el-radio label="auto">{{ $t("subscriptionTemplate.addressModeAuto") }}</el-radio>
            <el-radio label="custom">{{ $t("subscriptionTemplate.addressModeCustom") }}</el-radio>
          </el-radio-group>
          <el-input
            v-if="addressMode === 'custom'"
            v-model="customAddress"
            class="address-input"
            :placeholder="$t('subscriptionTemplate.ipPlaceholder')"
            maxlength="45"
          />
          <el-alert
            v-if="addressMode === 'legacy'"
            class="legacy-warning"
            type="warning"
            :closable="false"
            show-icon
            :title="$t('subscriptionTemplate.legacyAddressWarning', { value: legacyAddress })"
          />
          <div class="hint">{{ $t("subscriptionTemplate.addressHint") }}</div>
        </el-form-item>

        <el-form-item :label="$t('subscriptionTemplate.variablesTitle')">
          <el-space wrap>
            <el-button
              v-for="variable in form.variables"
              :key="variable"
              size="small"
              @click="insertVariable(variable)"
              >{{ variable }}</el-button
            >
          </el-space>
        </el-form-item>

        <el-table
          :data="form.variables.map((variable) => ({ variable }))"
          size="small"
          border
        >
          <el-table-column
            prop="variable"
            :label="$t('subscriptionTemplate.variable')"
            width="210"
          />
          <el-table-column :label="$t('subscriptionTemplate.description')">
            <template #default="scope">
              {{
                $t(
                  `subscriptionTemplate.variables.${variableKey(
                    scope.row.variable
                  )}`
                )
              }}
            </template>
          </el-table-column>
        </el-table>

        <el-descriptions
          :title="$t('subscriptionTemplate.detected')"
          :column="1"
          border
        >
          <el-descriptions-item label="IPv4">
            <el-text v-if="form.serverIPv4" type="success">{{
              form.serverIPv4
            }}</el-text>
            <el-text v-else type="danger">{{ form.serverIPv4Error }}</el-text>
          </el-descriptions-item>
          <el-descriptions-item label="IPv6">
            <el-text v-if="form.serverIPv6" type="success">{{
              form.serverIPv6
            }}</el-text>
            <el-text v-else type="warning">{{ form.serverIPv6Error }}</el-text>
          </el-descriptions-item>
        </el-descriptions>

        <el-alert
          v-if="form.previewError"
          :title="form.previewError"
          type="error"
          :closable="false"
        />
        <el-descriptions
          v-else
          :title="$t('subscriptionTemplate.preview')"
          :column="1"
          border
        >
          <el-descriptions-item :label="$t('subscriptionTemplate.name')">{{
            form.previewName
          }}</el-descriptions-item>
          <el-descriptions-item :label="$t('subscriptionTemplate.server')">{{
            form.previewAddress
          }}</el-descriptions-item>
        </el-descriptions>

        <el-form-item class="actions">
          <el-button type="primary" @click="save">{{
            $t("common.save")
          }}</el-button>
          <el-button @click="reset">{{
            $t("subscriptionTemplate.restore")
          }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ElMessage, ElMessageBox, FormInstance, FormRules } from "element-plus";
import {
  getSubscriptionTemplateApi,
  resetSubscriptionTemplateApi,
  updateSubscriptionTemplateApi,
} from "@/api/config";
import type { SubscriptionTemplate } from "@/api/config/types";
import { useI18n } from "vue-i18n";
import {
  classifyServerAddress,
  isLiteralIp,
  serverAddressValue,
  type ServerAddressMode,
} from "./server-address";

const { t } = useI18n();
const formRef = ref<FormInstance>();
const addressMode = ref<ServerAddressMode>("auto");
const customAddress = ref("");
const legacyAddress = ref("");
const form = reactive<SubscriptionTemplate>({
  nodeNameTemplate: "Hysteria2",
  serverAddressTemplate: "{SERVER_IP}",
  variables: [],
});
const rules: FormRules = {
  nodeNameTemplate: [{ required: true, message: "Required", trigger: "blur" }],
};

const load = async () => {
  const { data } = await getSubscriptionTemplateApi();
  Object.assign(form, data);
  addressMode.value = classifyServerAddress(data.serverAddressTemplate);
  if (addressMode.value === "custom") customAddress.value = data.serverAddressTemplate;
  if (addressMode.value === "legacy") legacyAddress.value = data.serverAddressTemplate;
};

const insertVariable = (variable: string) => {
  form.nodeNameTemplate += variable;
};

const variableKey = (variable: string) => variable.slice(1, -1);
const onAddressModeChange = (mode: ServerAddressMode) => {
  if (mode === "auto") form.serverAddressTemplate = "{SERVER_IP}";
  if (mode === "custom" && !customAddress.value) customAddress.value = form.serverIPv4 || "";
};

const save = async () => {
  await formRef.value?.validate();
  if (addressMode.value === "legacy") {
    ElMessage.error(t("subscriptionTemplate.legacyAddressBlocked"));
    return;
  }
  if (addressMode.value === "custom" && !isLiteralIp(customAddress.value)) {
    ElMessage.error(t("subscriptionTemplate.invalidIp"));
    return;
  }
  form.serverAddressTemplate = serverAddressValue(addressMode.value, customAddress.value);
  await updateSubscriptionTemplateApi(form);
  ElMessage.success(t("subscriptionTemplate.saved"));
  await load();
};

const reset = async () => {
  await ElMessageBox.confirm(
    t("subscriptionTemplate.restoreConfirm"),
    t("common.confirm")
  );
  await resetSubscriptionTemplateApi();
  ElMessage.success(t("subscriptionTemplate.saved"));
  await load();
};

onMounted(load);
</script>

<style scoped>
.hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-top: 4px;
}
.address-input,
.legacy-warning {
  margin-top: 12px;
}
.el-table,
.el-descriptions,
.el-alert {
  margin-bottom: 20px;
}
.actions {
  margin-top: 20px;
}
</style>
