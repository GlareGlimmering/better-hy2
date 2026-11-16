<template>
  <main class="hui-page">
    <section class="hui-surface hui-error-panel">
      <el-result icon="warning" :title="$t('panelErrors.forbidden')">
        <template #extra>
          <router-link class="hui-error-link" to="/">{{ $t("panelErrors.home") }}</router-link>
        </template>
      </el-result>
    </section>
  </main>
</template>
