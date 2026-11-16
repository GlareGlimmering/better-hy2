<template>
  <main class="hui-page">
    <section class="hui-surface hui-error-panel">
      <el-result icon="info" :title="$t('panelErrors.missing')">
        <template #extra>
          <router-link class="hui-error-link" to="/">{{ $t("panelErrors.home") }}</router-link>
        </template>
      </el-result>
    </section>
  </main>
</template>
