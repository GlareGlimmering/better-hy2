<script setup lang="ts">
import { ElConfigProvider } from "element-plus";
import { useAppStore } from "@/store/modules/app";
import { useSettingsStore } from "@/store/modules/settings";
const appStore = useAppStore();
const settingsStore = useSettingsStore();

watchEffect(() => {
  document.documentElement.style.setProperty(
    "--el-color-primary",
    settingsStore.themeColor
  );
  document.body.setAttribute("layout", settingsStore.layout);
});
</script>

<template>
  <el-config-provider :locale="appStore.locale" :size="appStore.size">
    <router-view />
  </el-config-provider>
</template>
