import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "NetX 用户中心",
  description: "NetX 企业办公网络用户中心。",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="zh-CN">
      <body>{children}</body>
    </html>
  );
}
