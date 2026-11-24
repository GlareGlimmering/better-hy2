<template>
  <main class="login-container">
    <div class="login-toolbar"><LangSelect /></div>
    <section class="hui-surface login-panel" aria-labelledby="login-heading">
      <header class="login-heading">
        <img :src="logo" alt="" width="42" height="42" />
        <h1 id="login-heading">{{ $t("login.title") }}</h1>
      </header>
      <el-form
        ref="loginFormRef"
        :model="loginForm"
        :rules="loginRules"
        label-position="top"
        class="login-form"
        @submit.prevent="handleLogin"
      >
        <el-form-item :label="$t('login.username')" prop="username">
          <el-input size="large" v-model="loginForm.username" name="username" autocomplete="username" />
        </el-form-item>
        <el-form-item :label="$t('login.password')" prop="pass">
          <el-input
            size="large"
            v-model="loginForm.pass"
            type="password"
            show-password
            name="pass"
            autocomplete="current-password"
            @keydown="checkCapslock"
            @keyup="checkCapslock"
            @blur="isCapslock = false"
          />
          <small v-if="isCapslock" class="caps-warning" role="status">{{ $t("login.capsLock") }}</small>
        </el-form-item>
        <el-alert v-if="passwordChangeRequired" :title="$t('login.passwordChangeRequired')" type="warning"
          :closable="false" class="password-notice" />
        <el-button native-type="submit" size="large" :loading="loading" type="primary" class="login-submit">
          {{ $t("login.login") }}
        </el-button>
      </el-form>
    </section>
  </main>
</template>

<script lang="ts">
export default {
  name: "index",
};
</script>

<script setup lang="ts">
import router from "@/router";
import LangSelect from "@/components/LangSelect/index.vue";
import logo from "@/assets/logo.svg";

// 状态管理依赖
import { useAccountStore } from "@/store/modules/account";

// API依赖
import { LocationQuery, LocationQueryValue, useRoute } from "vue-router";
import { AccountLoginDto } from "@/api/account/types";

const accountStore = useAccountStore();
const route = useRoute();
const passwordChangeRequired = ref(false);

/**
 * 按钮loading
 */
const loading = ref(false);
/**
 * 是否大写锁定
 */
const isCapslock = ref(false);
/**
 * 登录表单引用
 */
const loginFormRef = ref(ElForm);

/**
 * 登录表单
 */
const loginForm = ref<AccountLoginDto>({
  username: "",
  pass: "",
});

const loginRules = {
  username: [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
    {
      max: 128,
      message: "Username format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
  pass: [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
    {
      max: 128,
      message: "Password format is incorrect",
      trigger: ["change", "blur"],
    },
  ],
};

/**
 * 检查输入大小写状态
 */
const checkCapslock = (event: KeyboardEvent) => {
  isCapslock.value = event.getModifierState("CapsLock");
};

/**
 * 登录
 */
const handleLogin = () => {
  loginFormRef.value.validate((valid: boolean) => {
    if (valid && !loading.value) {
      loading.value = true;
      const params = { ...loginForm.value };
      accountStore
        .login(params)
        .then((signedIn) => {
          if (!signedIn) {
            passwordChangeRequired.value = true;
            return;
          }
          loginForm.value.pass = "";
          const query: LocationQuery = route.query;

          const redirect = (query.redirect as LocationQueryValue) ?? "/";

          const otherQueryParams = Object.keys(query).reduce(
            (acc: any, cur: string) => {
              if (cur !== "redirect") {
                acc[cur] = query[cur];
              }
              return acc;
            },
            {}
          );

          router.push({ path: redirect, query: otherQueryParams });
        })
        .catch(() => {})
        .finally(() => {
          loading.value = false;
        });
    }
  });
};
</script>

<style lang="scss" scoped>
.login-container {
  display: grid;
  place-items: center;
  min-height: 100vh;
  padding: 72px 16px 36px;
  background: var(--hui-canvas);
  color: var(--hui-ink);
  font-family: Inter, system-ui, "PingFang SC", "Microsoft YaHei", sans-serif;
}

.login-toolbar {
  position: absolute;
  top: 18px;
  right: 20px;
}

.login-panel {
  width: min(100%, 416px);
  padding: clamp(20px, 5vw, 34px);
}

.login-heading {
  margin-bottom: 24px;

  h1 {
    margin: 10px 0 0;
    font-size: 29px;
    line-height: 1.2;
    letter-spacing: -0.035em;
  }
}

.login-submit { width: 100%; }
.password-notice { margin-bottom: 14px; }
.caps-warning { color: var(--el-color-warning); }
</style>
