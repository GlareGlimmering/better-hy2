<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { ArrowDown, Fold, Expand, UserFilled } from "@element-plus/icons-vue";
import { useAppStore } from "@/store/modules/app";
import { useAccountStore } from "@/store/modules/account";
import { translateRouteTitleI18n } from "@/utils/i18n";

const appStore = useAppStore();
const accountStore = useAccountStore();
const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const pageTitle = computed(() => route.meta.title ? translateRouteTitleI18n(route.meta.title) : "Better HY2");

async function logout() {
  try {
    await ElMessageBox.confirm(t("navbar.logoutConfirm"), t("navbar.logout"), {
      confirmButtonText: t("common.confirm"),
      cancelButtonText: t("common.cancel"),
      type: "warning",
    });
  } catch {
    return;
  }
  const redirect = route.fullPath;
  await accountStore.logout();
  await router.push({ path: "/login", query: { redirect } });
}
</script>

<template>
  <header class="navbar">
    <div class="navbar-navigation">
      <button
        class="nav-button"
        type="button"
        aria-controls="panel-navigation"
        :aria-expanded="appStore.sidebar.opened"
        :aria-label="$t('navbar.navigation')"
        @click="appStore.toggleSidebar(false)"
      >
        <el-icon><Fold v-if="appStore.sidebar.opened" /><Expand v-else /></el-icon>
      </button>
      <span class="navbar-title">{{ pageTitle }}</span>
    </div>

    <div class="navbar-actions">
      <lang-select class="navbar-language" />
      <el-dropdown trigger="click">
        <button type="button" class="navbar-user" :aria-label="accountStore.username || $t('route.account')">
          <span class="navbar-avatar"><el-icon><UserFilled /></el-icon></span>
          <span class="navbar-username">{{ accountStore.username }}</span>
          <el-icon class="navbar-chevron"><ArrowDown /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="logout">{{ $t("navbar.logout") }}</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<style lang="scss" scoped>
.navbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  height: var(--hui-header-height);
  padding-inline: var(--hui-page-gutter);
  background: var(--hui-surface);
  color: var(--hui-ink);
  border-bottom: 1px solid var(--hui-border);
}

.navbar-navigation,
.navbar-actions,
.navbar-user {
  display: flex;
  align-items: center;
  gap: 10px;
}

.navbar-navigation {
  min-width: 0;
}

.navbar-actions {
  flex: none;
}

.navbar-title,
.navbar-username {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 650;
}

.navbar-username {
  max-width: 110px;
}

.nav-button,
.navbar-user {
  min-height: 38px;
  background: none;
  color: var(--hui-muted);
  border-radius: var(--hui-radius-sm);
  cursor: pointer;

  &:hover {
    background: var(--hui-hover);
    color: var(--hui-ink);
  }
}

.nav-button {
  flex: none;
  width: 38px;
  display: grid;
  place-items: center;
  font-size: 19px;
}

.navbar-user {
  padding-inline: 6px 8px;
}

.navbar-avatar {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--hui-accent-soft);
  color: var(--hui-accent-ink);
}

.navbar-chevron {
  font-size: 11px;
}

@media (width <= 600px) {
  .navbar-username { display: none; }
  .navbar-actions { gap: 4px; }
}
</style>
