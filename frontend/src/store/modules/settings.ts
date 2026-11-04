import { defineStore } from "pinia";
import defaultSettings from "@/settings";
import { useStorage } from "@vueuse/core";

export const useSettingsStore = defineStore("setting", () => {
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
    fixedHeader,
    sidebarLogo,
    layout,
    themeColor,
    changeSetting,
  };
});
