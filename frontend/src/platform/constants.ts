import type { Blueprint, Foundation, Workspace } from "./types";

export const services = ["Manufacturing", "Shipping", "Chat", "Calls"];

export const adminPages = [
  "Command",
  "Quotations",
  "Orders",
  "Messages",
  "Shipping",
  "Payments",
  "Directory",
  "App home",
  "Customers",
  "Calls",
  "Email",
  "Users & Access",
  "Settings"
];

export const customerPages = [
  "Home",
  "Get Quote",
  "Orders",
  "Messages",
  "Shipping",
  "Payments",
  "Directory",
  "Products",
  "Settings"
];

export const emptyWorkspace: Workspace = {
  products: [],
  quotations: [],
  manufacturing: [],
  rateSheets: [],
  shipping: [],
  payments: [],
  ledger: [],
  conversations: [],
  calls: [],
  presence: [],
  notices: [],
  featuredProducts: [],
  metrics: {},
  pagination: {}
};

export const fallbackFoundation: Foundation = {
  phase: "Phase 3: workflow APIs and customer portal",
  principles: [],
  modules: []
};

export const fallbackBlueprint: Blueprint = {
  productName: "ChakuChuri.pk",
  backendStatus: "Loading",
  customerMetrics: [],
  adminMetrics: [],
  orders: [],
  shipping: [],
  calls: []
};
