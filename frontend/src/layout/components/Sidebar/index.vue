<script setup lang="ts">
import { useRoute } from "vue-router";

import SidebarItem from "./SidebarItem.vue";
import Logo from "./Logo.vue";

import { useSettingsStore } from "@/store/modules/settings";
import { usePermissionStore } from "@/store/modules/permission";
import { useAppStore } from "@/store/modules/app";
import { storeToRefs } from "pinia";

const settingsStore = useSettingsStore();
const permissionStore = usePermissionStore();
const appStore = useAppStore();

const { sidebarLogo } = storeToRefs(settingsStore);
const route = useRoute();
</script>

<template>
  <aside id="panel-navigation" :aria-label="$t('navbar.navigation')">
    <logo v-if="sidebarLogo" :collapse="!appStore.sidebar.opened" />
    <el-scrollbar>
      <el-menu
        :default-active="route.path"
        :collapse="!appStore.sidebar.opened"
        class="hui-nav-menu"
        :unique-opened="false"
        :collapse-transition="false"
        mode="vertical"
      >
        <sidebar-item
          v-for="route in permissionStore.routes"
          :item="route"
          :key="route.path"
          :base-path="route.path"
          :is-collapse="!appStore.sidebar.opened"
        />
      </el-menu>
    </el-scrollbar>
  </aside>
</template>
