// 面板设置
interface DefaultSettings {
  /**
   * 系统title
   */
  title: string;

  /**
   * 导航栏布局
   */
  layout: string;
  /**
   * 主题颜色
   */
  themeColor: string;
  /**
   * 主题模式
   */
  theme: string;

  /**
   * 布局大小
   */
  size: string;

  /**
   * 语言
   */
  language: string;
}

const defaultSettings: DefaultSettings = {
  title: "Better HY2",
  layout: "left",
  themeColor: "#409EFF",
  /**
   *  主题模式
   *
   * dark:暗黑模式
   * light: 明亮模式
   */
  theme: "dark",
  size: "default", // default |large |small
  language: "zh-cn", // zh-cn| en
};

export default defaultSettings;
