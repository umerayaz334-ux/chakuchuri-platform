import { chromium } from "playwright";

const browser = await chromium.launch({
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
  headless: true
});
const context = await browser.newContext({
  permissions: ["clipboard-read", "clipboard-write"],
  viewport: { width: 1440, height: 1000 }
});
const page = await context.newPage();
const runtimeErrors = [];
let connectionRequests = 0;

page.on("pageerror", (error) => runtimeErrors.push(error.message));
page.on("console", (message) => {
  if (message.type() === "error" && !message.text().includes("favicon.ico")) runtimeErrors.push(message.text());
});
page.on("response", (response) => {
  if (response.url().includes("/api/platform/connection") && response.ok()) connectionRequests += 1;
});

await page.goto("http://127.0.0.1:5170/settings", { waitUntil: "networkidle" });
if (await page.getByRole("button", { name: "Sign in" }).count()) {
  await page.getByLabel("Email").fill("admin@chakuchuri.pk");
  await page.getByLabel("Password").fill("admin123");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.locator(".cc-portal-shell").waitFor();
}
await page.goto("http://127.0.0.1:5170/settings", { waitUntil: "networkidle" });
await page.getByRole("heading", { name: "Device connection" }).waitFor();
await page.getByText("Connected / auto-updating").waitFor();

const stableUrl = (await page.locator(".cc-connection-row.is-stable code").textContent())?.trim();
const wifiUrl = (await page.locator(".cc-connection-row:not(.is-stable) code").first().textContent())?.trim();
const initialRequests = connectionRequests;
await page.getByRole("button", { name: "Refresh connection addresses" }).click();
await page.waitForFunction((count) => window.__connectionRequestCount === undefined || true, initialRequests);
await page.waitForTimeout(600);

await page.getByRole("button", { name: "Copy stable address" }).click();
const copiedUrl = await page.evaluate(() => navigator.clipboard.readText());

const desktop = await page.evaluate(() => {
  const section = document.querySelector(".cc-connection-section");
  const row = document.querySelector(".cc-connection-row");
  return {
    sectionVisible: Boolean(section && section.getBoundingClientRect().height > 0),
    rowColumns: row ? getComputedStyle(row).gridTemplateColumns : "",
    overflow: document.documentElement.scrollWidth - window.innerWidth
  };
});

await page.setViewportSize({ width: 390, height: 844 });
await page.reload({ waitUntil: "networkidle" });
await page.getByRole("heading", { name: "Device connection" }).waitFor();
const mobile = await page.evaluate(() => {
  const row = document.querySelector(".cc-connection-row");
  const actions = document.querySelector(".cc-connection-actions");
  const rowRect = row.getBoundingClientRect();
  const actionsRect = actions.getBoundingClientRect();
  return {
    innerWidth: window.innerWidth,
    scrollWidth: document.documentElement.scrollWidth,
    rowColumns: getComputedStyle(row).gridTemplateColumns,
    actionsFit: actionsRect.right <= rowRect.right + 1 && actionsRect.left >= rowRect.left - 1
  };
});

const result = { stableUrl, wifiUrl, copiedUrl, connectionRequests, desktop, mobile, runtimeErrors };
console.log(JSON.stringify(result, null, 2));

if (stableUrl !== "http://desktop-nk4iih0.local:5170") throw new Error("Stable address is missing or incorrect.");
if (!wifiUrl?.startsWith("http://") || !wifiUrl.endsWith(":5170")) throw new Error("Active Wi-Fi fallback is missing.");
if (copiedUrl !== stableUrl) throw new Error("Copy address did not use the stable URL.");
if (connectionRequests < 2) throw new Error("Manual connection refresh did not request current adapter data.");
if (!desktop.sectionVisible || desktop.overflow > 0) throw new Error("Desktop connection section is not laid out correctly.");
if (mobile.scrollWidth > mobile.innerWidth || !mobile.actionsFit) throw new Error("Mobile connection section overflows.");
if (runtimeErrors.length) throw new Error(`Runtime errors: ${runtimeErrors.join(" | ")}`);

await browser.close();
