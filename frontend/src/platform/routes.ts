const pagePaths: Record<string, string> = {
  Command: "/dashboard",
  Home: "/dashboard",
  Customers: "/customers",
  "Users & Access": "/users",
  Quotations: "/quotations",
  "Get Quote": "/quotations",
  Products: "/products",
  Orders: "/orders",
  Shipping: "/shipping",
  Payments: "/payments",
  Directory: "/directory",
  "App home": "/app-home",
  Messages: "/messages",
  Calls: "/calls",
  Email: "/email",
  Settings: "/settings",
  "Verify identity": "/verify-identity"
};

const routeAliases: Record<string, string[]> = {
  Command: ["/command", "/admin"],
  Home: ["/home"],
  "Users & Access": ["/users-and-access"],
  Quotations: ["/quotes"],
  "Get Quote": ["/get-quote", "/quotes"],
  Orders: ["/manufacturing"],
  Messages: ["/messages-and-calls"]
};

export function pathForPage(page: string) {
  return pagePaths[page] || `/${slug(page)}`;
}

export function pageFromPath(pathname: string, availablePages: string[]) {
  const current = normalizePath(pathname);
  if (availablePages.includes("Customers") && current.startsWith("/customers/")) return "Customers";
  return availablePages.find((page) => {
    if (normalizePath(pathForPage(page)) === current) return true;
    return (routeAliases[page] || []).some((alias) => normalizePath(alias) === current);
  });
}

export function updatePagePath(page: string, replace = false) {
  const nextPath = pathForPage(page);
  if (normalizePath(window.location.pathname) === normalizePath(nextPath)) return;
  const method = replace ? "replaceState" : "pushState";
  window.history[method]({ page }, "", nextPath);
  notifyRouteChange();
}

export function customerIdFromPath(pathname: string) {
  const match = pathname.match(/^\/customers\/([^/?#]+)/i);
  if (!match) return "";
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return "";
  }
}

export function updateCustomerPath(customerId: string, replace = false) {
  const nextPath = `/customers/${encodeURIComponent(customerId)}`;
  const method = replace ? "replaceState" : "pushState";
  window.history[method]({ page: "Customers", customerId }, "", nextPath);
  notifyRouteChange();
}

export function updatePublicPath(replace = true) {
  if (normalizePath(window.location.pathname) === "/") return;
  const method = replace ? "replaceState" : "pushState";
  window.history[method]({}, "", "/");
  notifyRouteChange();
}

function notifyRouteChange() {
  window.dispatchEvent(new Event("cc-route-change"));
}

function normalizePath(value: string) {
  const clean = `/${value}`.replace(/\/{2,}/g, "/").replace(/\/$/, "").toLowerCase();
  return clean || "/";
}

function slug(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/&/g, "and")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}
