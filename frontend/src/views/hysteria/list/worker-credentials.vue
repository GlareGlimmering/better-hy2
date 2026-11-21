<template>
  <div class="worker-credentials">
    <el-input
      v-model="credentials"
      type="password"
      show-password
      autocomplete="new-password"
      :placeholder="$t(configured ? 'hysteria.workerCredentialsSaved' : 'hysteria.workerCredentialsFormat')"
    />
    <el-button :disabled="!configured" @click="copyCredentials">
      <template #icon><i-ep-copy-document /></template>
      {{ $t("hysteria.copyWorkerCredentials") }}
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useI18n } from "vue-i18n";
import { readACMEDNSValueApi } from "@/api/config";
import { copyConfirmedDNSValue, parseWorkerCredentials } from "./dns-copy";

const props = defineProps<{ modelValue: Record<string, string> }>();
const emit = defineEmits<{ (event: "update:modelValue", values: Record<string, string>): void }>();
const { t } = useI18n();
const redacted = "********";
const configured = computed(() => !!props.modelValue.worker_url && !!props.modelValue.worker_token);
const draft = ref("");
const credentials = computed({
  get: () => draft.value,
  set: (value: string) => {
    draft.value = value;
    emit("update:modelValue", parseWorkerCredentials(value));
  },
});
watch(() => props.modelValue, (values) => {
  const { worker_url, worker_token } = values;
  const current = parseWorkerCredentials(draft.value);
  if (current.worker_url === worker_url && current.worker_token === worker_token) return;
  draft.value = worker_url === redacted || worker_token === redacted
    ? ""
    : [worker_url, worker_token].filter(Boolean).join(" ");
}, { immediate: true });

const copyCredentials = async () => {
  const snapshot = { ...props.modelValue };
  const read = async (key: "worker_url" | "worker_token") => {
    const value = snapshot[key];
    return value === redacted ? (await readACMEDNSValueApi("worker", key)).data : value;
  };
  try {
    const copied = await copyConfirmedDNSValue(
      () => ElMessageBox.confirm(t("hysteria.copyWorkerWarning")).then(() => true, () => false),
      async () => {
        if (!navigator.clipboard?.writeText) throw new Error("clipboard unavailable");
        const [url, token] = await Promise.all([read("worker_url"), read("worker_token")]);
        return [url, token].join(" ");
      },
      (value) => navigator.clipboard.writeText(value)
    );
    if (copied) ElMessage.success(t("common.success"));
  } catch {
    ElMessage.error(t("hysteria.copyDNSFailed"));
  }
};
</script>

<style scoped>
.worker-credentials {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  width: 100%;
}
.worker-credentials .el-input { flex: 1 1 250px; }
</style>
