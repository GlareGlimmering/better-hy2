<script setup lang="ts">
import { computed, watch } from "vue";
import { useRoute } from "vue-router";
import { useWindowSize } from "@vueuse/core";
import { AppMain, Navbar } from "./components";
import Sidebar from "./components/Sidebar/index.vue";
import { useAppStore } from "@/store/modules/app";
import { useSettingsStore } from "@/store/modules/settings";

const { width } = useWindowSize();
const viewport = computed(() => width.value < 992 ? "mobile" : width.value < 1200 ? "compact" : "wide");
const route = useRoute();
const appStore = useAppStore();
const settingsStore = useSettingsStore();

const classObj = computed(() => ({
  hideSidebar: !appStore.sidebar.opened,
  openSidebar: appStore.sidebar.opened,
  withoutAnimation: appStore.sidebar.withoutAnimation,
  mobile: appStore.device === "mobile",
}));

watch(viewport, (mode) => {
  appStore.toggleDevice(mode === "mobile" ? "mobile" : "desktop");
  if (mode === "wide") appStore.openSideBar(true);
  else appStore.closeSideBar(true);
}, { immediate: true });

function closeMobileMenu() {
  if (classObj.value.mobile && classObj.value.openSidebar) appStore.closeSideBar(false);
}

watch(() => route.path, closeMobileMenu);
</script>

<template>
  <div :class="classObj" class="app-wrapper" @keydown.esc="closeMobileMenu">
    <button
      v-if="classObj.mobile && classObj.openSidebar"
      type="button"
      class="drawer-bg"
      :aria-label="$t('navbar.navigation')"
      @click="closeMobileMenu"
    />
    <Sidebar class="sidebar-container" />
    <div class="main-container">
      <div :class="{ 'fixed-header': settingsStore.fixedHeader }">
        <Navbar />
      </div>
      <AppMain />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.app-wrapper {
  min-height: 100vh;
  width: 100%;
  background: var(--hui-canvas);
  color: var(--hui-ink);
  font-family: Inter, system-ui, "PingFang SC", "Microsoft YaHei", sans-serif;

  &.mobile.openSidebar {
    position: fixed;
    inset: 0;
  }
}

.fixed-header {
  position: sticky;
  top: 0;
  z-index: 9;
}

.drawer-bg {
  position: fixed;
  inset: 0;
  z-index: 999;
  background: rgb(9 17 31 / 52%);
}
</style>
