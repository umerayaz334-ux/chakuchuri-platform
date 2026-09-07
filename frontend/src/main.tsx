import React from "react";
import { createRoot } from "react-dom/client";
import PlatformApp from "./PlatformAppNext";
import "./styles.css";
import "./platform.css";
import "./platform-workflow.css";
import "./platform/user-access.css";
import "./platform/messenger-unread.css";
import "./platform/profile-access.css";
import "./platform/customer-directory.css";
import "./platform/accounting-desk.css";
import "./platform/call-experience.css";
import "./platform/orders-desk.css";
import "./platform/quotations-desk.css";
import "./platform/email-center.css";
import "./platform/customer-documents.css";
import "./platform/shipping-desk.css";
import "./platform/trade-directory.css";
import "./platform/app-home-desk.css";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <PlatformApp />
  </React.StrictMode>
);
