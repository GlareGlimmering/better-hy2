<script setup lang="ts">
import { useI18n } from "vue-i18n";
import SvgIcon from "@/components/SvgIcon/index.vue";
import { useAppStore } from "@/store/modules/app";

const appStore = useAppStore();
const { locale } = useI18n();

function handleLanguageChange(lang: string) {
  locale.value = lang;
  appStore.changeLanguage(lang);
  if (lang == "en") {
    ElMessage.success("Switch Language Successful!");
  } else if (lang == "ru") {
    ElMessage.success("Смена языка прошла успешно!");
  } else {
    ElMessage.success("切换语言成功！");
  }
}
</script>

<template>
  <el-dropdown trigger="click" @command="handleLanguageChange">
    <button type="button" class="language-trigger" :aria-label="$t('navbar.language')">
      <svg-icon icon-class="language" size="18px" />
    </button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item :disabled="appStore.language === 'en'" command="en">
          English
        </el-dropdown-item>
        <el-dropdown-item :disabled="appStore.language === 'ru'" command="ru">
          Русский
        </el-dropdown-item>
        <el-dropdown-item
          :disabled="appStore.language === 'zh-cn'"
          command="zh-cn"
        >
          简体中文
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<style scoped>
.language-trigger {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: var(--hui-radius-sm);
  background: transparent;
  color: var(--hui-muted, var(--el-text-color-regular));
  cursor: pointer;
}

.language-trigger:hover {
  background: var(--hui-hover, var(--el-fill-color-light));
}
</style>
