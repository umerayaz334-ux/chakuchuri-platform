import { chromium } from "playwright";
import { mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const outputDir = fileURLToPath(new URL("../../tmp/settings-language-mobile/", import.meta.url));
await mkdir(outputDir, { recursive: true });

const browser = await chromium.launch({
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
  headless: true
});

const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
const page = await context.newPage();
const runtimeErrors = [];
page.on("pageerror", (error) => runtimeErrors.push(error.message));
page.on("console", (message) => {
  if (message.type() === "error" && !message.text().includes("favicon.ico")) runtimeErrors.push(message.text());
});

await page.goto("http://127.0.0.1:5170/dashboard", { waitUntil: "networkidle" });
await page.getByLabel("Email").fill("admin@chakuchuri.pk");
await page.getByLabel("Password").fill("admin123");
await page.getByRole("button", { name: "Sign in" }).click();
await page.locator(".cc-portal-shell").waitFor();

const originalSettings = await page.evaluate(async () => {
  const response = await fetch("/api/platform/settings");
  const payload = await response.json();
  return payload.data;
});

await page.goto("http://127.0.0.1:5170/settings", { waitUntil: "networkidle" });
await page.locator(".cc-platform-settings").waitFor();
const desktopHeaderDisplay = await page.locator(".cc-mobile-appbar").evaluate((node) => getComputedStyle(node).display);
const choice = page.getByRole("checkbox");

if (!(await choice.isChecked())) {
  await choice.check();
  await saveSettings(page);
}

await page.locator(".cc-portal-topbar .cc-language-switch button").nth(1).click();
await page.waitForFunction(() => document.documentElement.dir === "rtl");
const urduDirection = await page.evaluate(() => ({ dir: document.documentElement.dir, lang: document.documentElement.lang }));
await page.locator(".cc-portal-topbar .cc-language-switch button").nth(0).click();
await page.waitForFunction(() => document.documentElement.dir === "ltr");

await choice.uncheck();
await saveSettings(page);
await page.waitForTimeout(200);
const hiddenAfterSave = await page.locator(".cc-language-switch, .cc-public-language").count();

await page.reload({ waitUntil: "networkidle" });
await page.locator(".cc-platform-settings").waitFor();
const hiddenAfterReload = await page.locator(".cc-language-switch, .cc-public-language").count();
await page.screenshot({ path: path.join(outputDir, "settings-language-hidden.png"), fullPage: true });

const publicContext = await browser.newContext({ viewport: { width: 390, height: 844 } });
const publicPage = await publicContext.newPage();
await publicPage.goto("http://127.0.0.1:5170/", { waitUntil: "networkidle" });
const publicSwitchesHidden = (await publicPage.locator(".cc-public-language").count()) === 0;
await publicContext.close();

await page.setViewportSize({ width: 390, height: 844 });
await page.goto("http://127.0.0.1:5170/dashboard", { waitUntil: "networkidle" });
await page.locator(".cc-mobile-appbar").waitFor();
const mobileClosed = await page.evaluate(() => {
  const appbar = document.querySelector(".cc-mobile-appbar");
  const sidebar = document.querySelector(".cc-portal-sidebar");
  const main = document.querySelector(".cc-portal-main");
  const children = [...appbar.children].map((node) => node.getBoundingClientRect());
  return {
    appbarDisplay: getComputedStyle(appbar).display,
    sameRow: new Set(children.map((rect) => Math.round(rect.top + rect.height / 2))).size === 1,
    hiddenLanguageSwitches: appbar.querySelectorAll(".cc-language-switch").length,
    sidebarRight: Math.round(sidebar.getBoundingClientRect().right),
    mainWidth: Math.round(main.getBoundingClientRect().width),
    innerWidth: window.innerWidth,
    scrollWidth: document.documentElement.scrollWidth
  };
});
await page.getByRole("button", { name: "Open navigation" }).click();
await page.waitForTimeout(220);
const mobileOpen = await page.evaluate(() => {
  const rect = document.querySelector(".cc-portal-sidebar").getBoundingClientRect();
  return {
    left: Math.round(rect.left),
    width: Math.round(rect.width),
    open: document.querySelector(".cc-portal-shell").classList.contains("is-mobile-nav-open")
  };
});
await page.screenshot({ path: path.join(outputDir, "mobile-navigation-open.png"), fullPage: true });
await page.getByRole("button", { name: "Close navigation" }).last().click();

await page.evaluate(async (settings) => {
  const session = JSON.parse(localStorage.getItem("cc_session"));
  const response = await fetch("/api/platform/settings", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + session.token
    },
    body: JSON.stringify(settings)
  });
  if (!response.ok) throw new Error("Could not restore settings: " + response.status);
}, originalSettings);

const result = {
  desktopHeaderDisplay,
  urduDirection,
  hiddenAfterSave,
  hiddenAfterReload,
  publicSwitchesHidden,
  mobileClosed,
  mobileOpen,
  runtimeErrors
};
console.log(JSON.stringify(result, null, 2));

if (desktopHeaderDisplay !== "none") throw new Error("Phone header leaked into the desktop layout.");
if (urduDirection.dir !== "rtl" || urduDirection.lang !== "ur") throw new Error("Urdu did not enable RTL mode.");
if (hiddenAfterSave !== 0 || hiddenAfterReload !== 0 || !publicSwitchesHidden) throw new Error("Language switch remained visible while disabled.");
if (mobileClosed.appbarDisplay !== "grid" || !mobileClosed.sameRow) throw new Error("Phone header is not aligned in one row.");
if (mobileClosed.hiddenLanguageSwitches !== 0) throw new Error("Phone language switch remained visible while disabled.");
if (mobileClosed.scrollWidth > mobileClosed.innerWidth || mobileClosed.mainWidth !== mobileClosed.innerWidth) throw new Error("Phone layout overflows horizontally.");
if (!mobileOpen.open || mobileOpen.left !== 0 || mobileOpen.width > mobileClosed.innerWidth) throw new Error("Phone navigation did not open correctly.");
if (runtimeErrors.length) throw new Error("Runtime errors: " + runtimeErrors.join(" | "));

await browser.close();

async function saveSettings(target) {
  const response = target.waitForResponse((item) => item.url().includes("/api/platform/settings") && item.request().method() === "POST");
  await target.getByRole("button", { name: "Save settings" }).click();
  const saved = await response;
  if (!saved.ok()) throw new Error("Settings save failed: " + saved.status());
  await target.getByRole("button", { name: "Save settings" }).waitFor({ state: "visible" });
  await target.waitForTimeout(250);
}