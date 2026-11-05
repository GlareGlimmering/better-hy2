import {
  createRouter,
  createWebHashHistory,
  RouteLocationNormalized,
  RouteRecordRaw,
} from "vue-router";

export const Layout = () => import("@/layout/index.vue");

// 静态路由
export const constantRoutes: RouteRecordRaw[] = [
  {
    path: "/redirect",
    component: Layout,
    meta: { hidden: true },
    children: [
      {
        path: "/redirect/:path(.*)",
        component: () => import("@/views/redirect/index.vue"),
      },
    ],
  },

  {
    path: "/login",
    component: () => import("@/views/login/index.vue"),
    meta: { hidden: true },
  },
  {
    path: "/monitor/system",
    redirect: "/info/account",
    meta: { hidden: true },
  },
  {
    path: "/monitor",
    redirect: "/info/account",
    meta: { hidden: true },
  },
  {
    path: "/",
    component: Layout,
    redirect: "/info/account",
    children: [
      {
        path: "401",
        component: () => import("@/views/error-page/401.vue"),
        meta: { hidden: true },
      },
      {
        path: "404",
        component: () => import("@/views/error-page/404.vue"),
        meta: { hidden: true },
      },
    ],
  },
];

export const asyncRoutes: any[] = [
  {
    path: "/info",
    component: "Layout",
    redirect: "/info/account",
    name: "Info",
    meta: {
      title: "info",
      icon: "report",
      roles: ["user", "admin"],
    },
    children: [
      {
        path: "account",
        component: "info/account/dashboard",
        name: "AccountInfo",
        meta: {
          title: "infoAccount",
          icon: "report",
          roles: ["user", "admin"],
        },
      },
    ],
  },
  {
    path: "/account",
    component: "Layout",
    redirect: "/list",
    name: "Account",
    meta: { title: "account", icon: "users", roles: ["admin"] },
    children: [
      {
        path: "list",
        component: "account/list/index",
        name: "AccountList",
        meta: {
          title: "accountList",
          icon: "users",
          roles: ["admin"],
        },
        props: (route: RouteLocationNormalized) => ({
          focus: route.query.focus,
        }),
      },
    ],
  },
  {
    path: "/hysteria",
    component: "Layout",
    redirect: "/list",
    name: "Hysteria",
    meta: { title: "hysteria", icon: "hysteria", roles: ["admin"] },
    children: [
      {
        path: "list",
        component: "hysteria/list/index",
        name: "HysteriaList",
        meta: {
          title: "hysteriaList",
          icon: "hysteria",
          roles: ["admin"],
        },
      },
    ],
  },
  {
    path: "/xray",
    component: "Layout",
    redirect: "/xray/outbound",
    name: "Xray",
    meta: { title: "xray", icon: "setting", roles: ["admin"] },
    children: [
      {
        path: "outbound",
        component: "xray/index",
        name: "XrayOutbound",
        meta: { title: "xrayOutbound", icon: "setting", roles: ["admin"] },
      },
    ],
  },
  {
    path: "/subscription",
    component: "Layout",
    redirect: "/subscription/settings",
    name: "Subscription",
    meta: { title: "subscription", icon: "link", roles: ["admin"] },
    children: [
      {
        path: "settings",
        component: "subscription/settings/index",
        name: "SubscriptionSettings",
        meta: {
          title: "subscriptionSettings",
          icon: "setting",
          roles: ["admin"],
        },
      },
    ],
  },
  {
    path: "/config",
    component: "Layout",
    redirect: "/list",
    name: "Config",
    meta: { title: "config", icon: "setting", roles: ["admin"] },
    children: [
      {
        path: "list",
        component: "config/list/index",
        name: "ConfigList",
        meta: {
          title: "configList",
          icon: "setting",
          roles: ["admin"],
        },
        props: (route: RouteLocationNormalized) => ({
          focus: route.query.focus,
        }),
      },
    ],
  },
  {
    path: "/log",
    component: "Layout",
    redirect: "/log/system",
    name: "Log",
    meta: { title: "log", icon: "error", roles: ["admin"] },
    children: [
      {
        path: "system",
        component: "log/system/index",
        name: "LogSystem",
        meta: {
          title: "logSystem",
          icon: "log-system",
          roles: ["admin"],
        },
      },
      {
        path: "hysteria",
        component: "log/hysteria/index",
        name: "LogHysteria",
        meta: {
          title: "logHysteria",
          icon: "log-hysteria",
          roles: ["admin"],
        },
      },
      {
        path: "access",
        component: "log/access/index",
        name: "LogAccess",
        meta: {
          title: "logAccess",
          icon: "log-hysteria",
          roles: ["admin"],
        },
      },
    ],
  },
];

/**
 * 创建路由
 */
const router = createRouter({
  history: createWebHashHistory(),
  routes: constantRoutes as RouteRecordRaw[],
  // 刷新时，滚动条位置还原
  scrollBehavior: () => ({ left: 0, top: 0 }),
});

/**
 * 重置路由
 */
export function resetRouter() {
  router.replace({ path: "/login" });
  location.reload();
}

export default router;
