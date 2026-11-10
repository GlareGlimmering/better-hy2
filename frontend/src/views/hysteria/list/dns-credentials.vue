<template>
  <div class="credentials">
    <div v-for="(value, key) in modelValue" :key="key" class="credential">
      <div>
        <code>{{ key }}</code>
        <el-button text @click="copyKey(String(key))">{{ $t("hysteria.copyDNSKey") }}</el-button>
      </div>
      <div>
        <span>********</span>
        <el-button text :disabled="value !== redacted" @click="copyValue(String(key))">{{ $t("hysteria.copyDNSValue") }}</el-button>
        <el-button text @click="edit(String(key))">{{ $t("common.update") }}</el-button>
        <el-button text type="danger" @click="remove(String(key))">{{ $t("common.delete") }}</el-button>
      </div>
    </div>
    <el-button @click="edit('')">+</el-button>
    <el-dialog v-model="editing" append-to-body width="min(420px, 95vw)" :title="$t('hysteria.config.acme.dns.config')" @closed="clearDraft">
      <el-form label-position="top">
        <el-form-item label="key">
          <el-input v-model="draftKey" :disabled="!!originalKey" />
        </el-form-item>
        <el-form-item label="value">
          <el-input v-model="draftValue" type="password" autocomplete="new-password" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editing = false">{{ $t("common.cancel") }}</el-button>
        <el-button type="primary" :disabled="!canSave" @click="save">{{ $t("common.confirm") }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useI18n } from "vue-i18n";
import { readACMEDNSValueApi } from "@/api/config";
import { copyConfirmedDNSValue } from "./dns-copy";

const props = defineProps<{ modelValue: Record<string, string>; provider: string }>();
const emit = defineEmits<{ (e: "update:modelValue", values: Record<string, string>): void }>();
const { t } = useI18n();
const redacted = "********";
const editing = ref(false);
const originalKey = ref("");
const draftKey = ref("");
const draftValue = ref("");
const canSave = computed(() => !!draftKey.value.trim() && !!draftValue.value.trim() &&
  draftValue.value !== redacted && (originalKey.value || !Object.prototype.hasOwnProperty.call(props.modelValue, draftKey.value.trim())));

const clearDraft = () => {
  originalKey.value = "";
  draftKey.value = "";
  draftValue.value = "";
};
const edit = (key: string) => {
  clearDraft();
  originalKey.value = key;
  draftKey.value = key;
  editing.value = true;
};
const save = () => {
  if (!canSave.value) return;
  emit("update:modelValue", { ...props.modelValue, [draftKey.value.trim()]: draftValue.value });
  editing.value = false;
  clearDraft();
};
const remove = (key: string) => {
  const next = { ...props.modelValue };
  delete next[key];
  emit("update:modelValue", next);
};
const copyKey = async (key: string) => {
  try {
    await navigator.clipboard.writeText(key);
    ElMessage.success(t("common.success"));
  } catch {
    ElMessage.error(t("hysteria.copyDNSFailed"));
  }
};
const copyValue = async (key: string) => {
  const provider = props.provider;
  try {
    const copied = await copyConfirmedDNSValue(
      () => ElMessageBox.confirm(t("hysteria.copyDNSWarning")).then(() => true, () => false),
      async () => {
        if (!navigator.clipboard?.writeText) throw new Error("clipboard unavailable");
        return (await readACMEDNSValueApi(provider, key)).data;
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
.credentials { width: 100%; }
.credential { display: flex; flex-wrap: wrap; gap: 0 12px; align-items: center; }
.credential code { overflow-wrap: anywhere; }
</style>
