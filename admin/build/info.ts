import type { Plugin } from "vite";
import { getPackageSize } from "./utils";

/** 构建结束后打印产物大小 */
export function viteBuildInfo(): Plugin {
  let outDir = "dist";
  let isBuild = false;
  return {
    name: "vite:buildInfo",
    configResolved(resolvedConfig) {
      outDir = resolvedConfig.build?.outDir ?? "dist";
      isBuild = resolvedConfig.command === "build";
    },
    closeBundle() {
      if (!isBuild) return;
      getPackageSize({
        folder: outDir,
        callback: (size: string) => {
          console.log(`netx-admin 构建完成，产物大小 ${size}`);
        }
      });
    }
  };
}
