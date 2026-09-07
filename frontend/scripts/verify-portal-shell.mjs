import { chromium } from "playwright";
import { mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const outputDir = fileURLToPath(new URL("../../tmp/portal-shell/", import.meta.url));
await mkdir(outputDir, { recursive: true });

const browser = await chromium.launch({
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
  headless: true
});
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
const page = await context.newPage();
const runtimeErrors = [];
const badResponses = [];
page.on("pageerror", (error) => runtimeErrors.push(`page: ${error.message}`));
page.on("console", (message) => {
  if (message.type() === "error") {
    const source = message.location().url;
    runtimeErrors.push(`console: ${message.text()}${source ? ` (${source})` : ""}`);
  }
});
page.on("response", (response) => {
  if (response.status() >= 400) badResponses.push(`${response.status()} ${response.url()}`);
});

await page.goto("http://localhost:5170/", { waitUntil: "networkidle" });
const publicRootVisible = await page.locator(".cc-public").isVisible();
const publicRootText = (await page.locator("body").innerText()).slice(0, 120);
await page.screenshot({ path: path.join(outputDir, "public-root.png"), fullPage: true });

await page.goto("http://127.0.0.1:5170/dashboard", { waitUntil: "networkidle" });
if (await page.getByRole("button", { name: "Sign in" }).count()) {
  await page.getByLabel("Email").fill("admin@chakuchuri.pk");
  await page.getByLabel("Password").fill("admin123");
  await page.getByRole("button", { name: "Sign in" }).click();
}
await page.locator(".cc-portal-shell").waitFor();
await page.waitForTimeout(500);

const expanded = await shellMetrics(page);
await page.screenshot({ path: path.join(outputDir, "sidebar-expanded.png"), fullPage: true });

const headerLayout = await page.evaluate(() => {
  const head = document.querySelector(".cc-sidebar-head").getBoundingClientRect();
  const brand = document.querySelector(".cc-sidebar-head .cc-brand").getBoundingClientRect();
  const bell = document.querySelector(".cc-notification-trigger").getBoundingClientRect();
  const toggle = document.querySelector(".cc-sidebar-toggle").getBoundingClientRect();
  const centers = [brand, bell, toggle].map((rect) => Math.round(rect.top + rect.height / 2));
  return { centers, fits: toggle.right <= head.right + 1 };
});
await page.getByRole("button", { name: /Notifications/ }).click();
await page.locator(".cc-notification-drawer").waitFor();
await page.waitForTimeout(220);
const updatesDrawer = await page.evaluate(() => {
  const drawer = document.querySelector(".cc-notification-drawer").getBoundingClientRect();
  const sidebar = document.querySelector(".cc-portal-sidebar").getBoundingClientRect();
  return {
    left: Math.round(drawer.left),
    width: Math.round(drawer.width),
    height: Math.round(drawer.height),
    sidebarWidth: Math.round(sidebar.width),
    viewportHeight: window.innerHeight
  };
});
await page.screenshot({ path: path.join(outputDir, "updates-drawer.png"), fullPage: true });
await page.locator(".cc-notification-close").click();
await page.getByRole("button", { name: "Collapse sidebar" }).click();
await page.waitForTimeout(250);
const collapsed = await shellMetrics(page);
await page.screenshot({ path: path.join(outputDir, "sidebar-collapsed.png"), fullPage: true });

await page.getByRole("link", { name: "Settings" }).click();
await page.waitForURL("**/settings");
await page.reload({ waitUntil: "networkidle" });
await page.locator('.cc-portal-link[aria-current="page"]', { hasText: "Settings" }).waitFor();
const reloadPath = new URL(page.url()).pathname;
const reloadPage = await page.locator('.cc-portal-link[aria-current="page"]').getAttribute("aria-label");
await page.screenshot({ path: path.join(outputDir, "settings-reloaded.png"), fullPage: true });

await page.goBack({ waitUntil: "networkidle" });
await page.locator(".cc-portal-shell").waitFor();
const backPath = new URL(page.url()).pathname;

await page.setViewportSize({ width: 390, height: 844 });
await page.reload({ waitUntil: "networkidle" });
const mobile = await page.evaluate(() => ({
  innerWidth: window.innerWidth,
  scrollWidth: document.documentElement.scrollWidth,
  sidebarWidth: Math.round(document.querySelector(".cc-portal-sidebar").getBoundingClientRect().width),
  mainWidth: Math.round(document.querySelector(".cc-portal-main").getBoundingClientRect().width)
}));
await page.screenshot({ path: path.join(outputDir, "sidebar-mobile.png"), fullPage: true });

const result = {
  publicRootVisible,
  publicRootText,
  headerLayout,
  updatesDrawer,
  expanded,
  collapsed,
  routeAfterReload: reloadPath,
  activeAfterReload: reloadPage || "Settings",
  routeAfterBack: backPath,
  mobile,
  runtimeErrors,
  badResponses
};
console.log(JSON.stringify(result, null, 2));

if (!publicRootVisible) throw new Error("Public root did not render.");
if (new Set(headerLayout.centers).size !== 1 || !headerLayout.fits) throw new Error("Sidebar header controls are not aligned in one row.");
if (updatesDrawer.left !== updatesDrawer.sidebarWidth || updatesDrawer.height !== updatesDrawer.viewportHeight) throw new Error("Updates drawer is not anchored beside the full-height sidebar.");
if (expanded.path !== "/dashboard") throw new Error("Dashboard route did not load.");
if (collapsed.sidebarWidth >= expanded.sidebarWidth) throw new Error("Sidebar did not collapse.");
if (collapsed.mainWidth <= expanded.mainWidth) throw new Error("Main page did not expand.");
if (reloadPath !== "/settings") throw new Error("Settings route did not survive reload.");
if (backPath !== "/dashboard") throw new Error("Browser Back did not restore dashboard.");
if (mobile.scrollWidth > mobile.innerWidth) throw new Error("Mobile layout has horizontal overflow.");
const actionableErrors = runtimeErrors.filter((item) => !item.includes("favicon.ico"));
const actionableResponses = badResponses.filter((item) => !item.includes("favicon.ico"));
if (actionableErrors.length) throw new Error(`Runtime errors: ${actionableErrors.join(" | ")}`);
if (actionableResponses.length) throw new Error(`Failed requests: ${actionableResponses.join(" | ")}`);

await browser.close();

async function shellMetrics(target) {
  return target.evaluate(() => ({
    path: window.location.pathname,
    sidebarWidth: Math.round(document.querySelector(".cc-portal-sidebar").getBoundingClientRect().width),
    mainWidth: Math.round(document.querySelector(".cc-portal-main").getBoundingClientRect().width),
    collapsed: document.querySelector(".cc-portal-shell").classList.contains("is-collapsed"),
    activePage: document.querySelector('.cc-portal-link[aria-current="page"]')?.textContent?.trim() || ""
  }));
}