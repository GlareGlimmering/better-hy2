import { defineStore } from "pinia";
import defaultSettings from "@/settings";
import { useStorage, useColorMode, usePreferredDark } from "@vueuse/core";
import { computed } from "vue";

export const useSettingsStore = defineStore("setting", () => {
  // Initialize once with App, not when the settings route is opened. Keep the
  // existing storage key; an explicit toggle persists light/dark, never auto.
  const colorMode = useColorMode({ emitAuto: true });
  const preferredDark = usePreferredDark();
  const isDark = computed({
    get: () => colorMode.value === "dark" || (colorMode.value === "auto" && preferredDark.value),
    set: (dark: boolean) => { colorMode.value = dark ? "dark" : "light"; },
  });
  // state
  const fixedHeader = useStorage<boolean>(
    "fixedHeader",
    defaultSettings.fixedHeader
  );
  const sidebarLogo = useStorage<boolean>(
    "sidebarLogo",
    defaultSettings.sidebarLogo
  );

  const layout = useStorage<string>("layout", defaultSettings.layout);

  const themeColor = useStorage<string>(
    "themeColor",
    defaultSettings.themeColor
  );

  // actions
  function changeSetting(param: { key: string; value: any }) {
    const { key, value } = param;
    switch (key) {
      case "fixedHeader":
        fixedHeader.value = value;
        break;
      case "sidevarLogo":
        sidebarLogo.value = value;
        break;
      case "layout":
        layout.value = value;
        break;
      case "themeColor":
        themeColor.value = value;
        break;
      default:
        break;
    }
  }

  return {
    isDark,
    fixedHeader,
    sidebarLogo,
    layout,
    themeColor,
    changeSetting,
  };
});
