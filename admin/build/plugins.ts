import vue from "@vitejs/plugin-vue";
import { viteBuildInfo } from "./info";
import svgLoader from "vite-svg-loader";
import Icons from "unplugin-icons/vite";
import type { PluginOption } from "vite";
import vueJsx from "@vitejs/plugin-vue-jsx";
import tailwindcss from "@tailwindcss/vite";
import removeConsole from "vite-plugin-remove-console";

export function getPluginsList(): PluginOption[] {
  return [
    tailwindcss(),
    vue(),
    // jsx、tsx语法支持
    vueJsx(),
    viteBuildInfo(),
    // svg组件化支持
    svgLoader(),
    // 自动按需加载图标
    Icons({
      compiler: "vue3",
      scale: 1
    }),
    // 线上环境删除console
    removeConsole({ external: ["src/assets/iconfont/iconfont.js"] })
  ];
}
