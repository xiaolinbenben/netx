import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import DashboardPage from "./DashboardPage";
import AccessPage from "./AccessPage";
import "./globals.css";

const isAccessPage =
  window.location.pathname.startsWith("/access/") ||
  window.location.pathname.startsWith("/dashboard/access/");

createRoot(document.getElementById("root")!).render(
  <StrictMode>{isAccessPage ? <AccessPage /> : <DashboardPage />}</StrictMode>,
);
