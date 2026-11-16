<script setup lang="ts">
import logo from "@/assets/logo.svg";

defineProps<{ collapse: boolean }>();
</script>

<template>
  <router-link class="sidebar-brand" :class="{ collapsed: collapse }" aria-label="Better HY2" to="/">
    <img :src="logo" alt="" width="32" height="32" />
    <strong v-if="!collapse">Better HY2</strong>
  </router-link>
</template>

<style lang="scss" scoped>
.sidebar-brand {
  display: flex;
  flex: none;
  align-items: center;
  gap: 10px;
  height: var(--hui-header-height);
  padding-inline: 16px;
  border-bottom: 1px solid var(--hui-border);
  color: var(--hui-ink);
  white-space: nowrap;
}

.sidebar-brand.collapsed {
  justify-content: center;
  padding-inline: 0;
}

.sidebar-brand strong {
  font-size: 16px;
  font-weight: 750;
  letter-spacing: -0.025em;
}

.sidebar-brand:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: -3px;
}
</style>
