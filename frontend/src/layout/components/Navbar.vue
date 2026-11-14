<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { useAppStore } from "@/store/modules/app";
import { useAccountStore } from "@/store/modules/account";

const appStore = useAppStore();
const accountStore = useAccountStore();

const route = useRoute();
const router = useRouter();

/**
 * 左侧菜单栏显示/隐藏
 */
function toggleSideBar() {
  appStore.toggleSidebar(true);
}

/**
 * 注销
 */
function logout() {
  ElMessageBox.confirm("确定注销并退出系统吗？", "提示", {
    confirmButtonText: "确定",
    cancelButtonText: "取消",
    type: "warning",
  }).then(() => {
    accountStore.logout().then(() => {
      router.push(`/login?redirect=${route.fullPath}`);
    });
  });
}
</script>

<template>
  <!-- 顶部导航栏 -->
  <div class="navbar">
    <!-- 左侧面包屑 -->
    <div class="navbar-navigation flex">
      <hamburger
        :is-active="appStore.sidebar.opened"
        @toggleClick="toggleSideBar"
      />
      <breadcrumb />
    </div>

    <!-- 右侧导航设置 -->
    <div class="navbar-actions flex">
      <div class="setting-container">
        <lang-select class="setting-item" />
      </div>

      <!-- 用户头像 -->
      <el-dropdown trigger="click">
        <div class="avatar-container">
          <img src="/src/assets/logo.svg" />
          <i-ep-caret-bottom class="w-3 h-3" />
        </div>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="logout">
              {{ $t("navbar.logout") }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.navbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 50px;
  background-color: #fff;
  box-shadow: 0 0 1px #0003;

  .navbar-navigation {
    min-width: 0;
    overflow: hidden;
  }

  .navbar-actions {
    flex: none;
  }

  .setting-container {
    display: flex;
    align-items: center;

    .setting-item {
      display: inline-block;
      width: 30px;
      height: 50px;
      line-height: 50px;
      color: #5a5e66;
      text-align: center;
      cursor: pointer;

      &:hover {
        background: rgb(249 250 251 / 100%);
      }
    }
  }

  .avatar-container {
    display: flex;
    align-items: center;
    justify-items: center;
    margin: 0 5px;
    cursor: pointer;

    img {
      width: 40px;
      height: 40px;
      border-radius: 5px;
    }
  }
}
</style>
