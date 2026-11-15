<template>
  <div class="login-container">
    <el-form
      ref="loginFormRef"
      :model="loginForm"
      :rules="loginRules"
      class="login-form"
    >
      <div class="flex text-white items-center py-4">
        <span class="text-2xl flex-1 text-center">{{ $t("login.title") }}</span>
        <lang-select style="color: #fff" />
      </div>

      <el-form-item prop="username">
        <div class="p-2 text-white">
          <svg-icon icon-class="user" />
        </div>
        <el-input
          class="flex-1"
          ref="username"
          size="large"
          v-model="loginForm.username"
          :placeholder="$t('login.username')"
          name="username"
        />
      </el-form-item>

      <el-tooltip
        :disabled="isCapslock === false"
        content="Caps lock is On"
        placement="right"
      >
        <el-form-item prop="pass">
          <span class="p-2 text-white">
            <svg-icon icon-class="password" />
          </span>
          <el-input
            class="flex-1"
            v-model="loginForm.pass"
            :placeholder="$t('login.password')"
            :type="passVisible === false ? 'password' : 'input'"
            size="large"
            name="pass"
            @keyup="checkCapslock"
            @keyup.enter="changingPassword || handleLogin()"
          />
          <span class="mr-2" @click="passVisible = !passVisible">
            <svg-icon
              :icon-class="passVisible === false ? 'eye' : 'eye-open'"
              class="text-white cursor-pointer"
            />
          </span>
        </el-form-item>
      </el-tooltip>

      <template v-if="changingPassword">
        <el-alert
          v-if="passwordChangeRequired"
          :title="$t('login.passwordChangeRequired')"
          type="warning"
          :closable="false"
        />
        <el-form-item prop="newPassword" class="my-4">
          <el-input v-model="newPassword" :placeholder="$t('login.newPassword')" type="password"
            autocomplete="new-password" show-password maxlength="128" @keyup.enter="handleLogin" />
        </el-form-item>
        <el-form-item class="my-4">
          <el-input v-model="confirmPassword" :placeholder="$t('login.confirmPassword')" type="password"
            autocomplete="new-password" show-password maxlength="128" @keyup.enter="handleLogin" />
        </el-form-item>
      </template>
      <el-button
        size="default"
        :loading="loading"
        type="primary"
        class="w-full"
        @click.prevent="handleLogin"
        >{{ $t(changingPassword ? "login.changePassword" : "login.login") }}
      </el-button>
      <el-button v-if="!passwordChangeRequired" text class="mt-2 text-white"
        @click="changingPassword = !changingPassword">
        {{ $t(changingPassword ? "login.login" : "login.changePassword") }}
      </el-button>
    </el-form>
  </div>
</template>

<script lang="ts">
export default {
  name: "index",
};
</script>

<script setup lang="ts">
import router from "@/router";
import LangSelect from "@/components/LangSelect/index.vue";
import SvgIcon from "@/components/SvgIcon/index.vue";

// 状态管理依赖
import { useAccountStore } from "@/store/modules/account";

// API依赖
import { LocationQuery, LocationQueryValue, useRoute } from "vue-router";
import { AccountLoginDto } from "@/api/account/types";
import { useI18n } from "vue-i18n";

const accountStore = useAccountStore();
const route = useRoute();
const { t } = useI18n();
const changingPassword = ref(false);
const passwordChangeRequired = ref(false);
const newPassword = ref("");
const confirmPassword = ref("");

/**
 * 按钮loading
 */
const loading = ref(false);
/**
 * 是否大写锁定
 */
const isCapslock = ref(false);
/**
 * 密码是否可见
 */
const passVisible = ref(false);

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
const checkCapslock = (e: any) => {
  const { key } = e;
  isCapslock.value = key && key.length === 1 && key >= "A" && key <= "Z";
};

/**
 * 登录
 */
const handleLogin = () => {
  loginFormRef.value.validate((valid: boolean) => {
    if (valid && !loading.value) {
      if (changingPassword.value && (
        new TextEncoder().encode(newPassword.value).length < 12 ||
        new TextEncoder().encode(newPassword.value).length > 128 ||
        newPassword.value !== confirmPassword.value ||
        newPassword.value === loginForm.value.pass
      )) {
        ElMessage.error(t("login.invalidNewPassword"));
        return;
      }
      loading.value = true;
      const params = { ...loginForm.value };
      accountStore
        .login(params, changingPassword.value ? newPassword.value : undefined)
        .then((signedIn) => {
          if (!signedIn) {
            passwordChangeRequired.value = true;
            changingPassword.value = true;
            return;
          }
          loginForm.value.pass = "";
          newPassword.value = "";
          confirmPassword.value = "";
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
  width: 100%;
  min-height: 100%;
  overflow: hidden;
  background-color: #2d3a4b;

  .login-form {
    width: 520px;
    max-width: 100%;
    padding: 160px 35px 0;
    margin: 0 auto;
    overflow: hidden;
  }
}

.el-form-item {
  background: rgb(0 0 0 / 10%);
  border: 1px solid rgb(255 255 255 / 10%);
  border-radius: 5px;
}

.el-input {
  background: transparent;

  // 子组件 scoped 无效，使用 :deep
  :deep(.el-input__wrapper) {
    padding: 0;
    background: transparent;
    box-shadow: none;

    .el-input__inner {
      color: #fff;
      background: transparent;
      border: 0;
      border-radius: 0;
      caret-color: #fff;

      &:-webkit-autofill {
        box-shadow: 0 0 0 1000px transparent inset !important;
        -webkit-text-fill-color: #fff !important;
      }

      // 设置输入框自动填充的延迟属性
      &:-webkit-autofill,
      &:-webkit-autofill:hover,
      &:-webkit-autofill:focus,
      &:-webkit-autofill:active {
        transition: color 99999s ease-out, background-color 99999s ease-out;
        transition-delay: 99999s;
      }
    }
  }
}
</style>
