import { chromium } from "playwright";

const browser = await chromium.launch({
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
  headless: true
});
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
const page = await context.newPage();
const runtimeErrors = [];
let listRequests = 0;
let createRequests = 0;
let downloadRequests = 0;

page.on("pageerror", (error) => runtimeErrors.push(error.message));
page.on("console", (message) => {
  if (message.type() === "error" && !message.text().includes("favicon.ico")) runtimeErrors.push(message.text());
});
page.on("response", (response) => {
  const url = response.url();
  const method = response.request().method();
  if (url.endsWith("/api/admin/backups") && method === "GET" && response.ok()) listRequests += 1;
  if (url.endsWith("/api/admin/backups") && method === "POST" && response.ok()) createRequests += 1;
  if (url.includes("/api/admin/backups/") && url.endsWith("/content") && response.ok()) downloadRequests += 1;
});

await page.goto("http://127.0.0.1:5170/settings", { waitUntil: "networkidle" });
if (await page.getByRole("button", { name: "Sign in" }).count()) {
  await page.getByLabel("Email").fill("admin@chakuchuri.pk");
  await page.getByLabel("Password").fill("admin123");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.locator(".cc-portal-shell").waitFor();
}
await page.goto("http://127.0.0.1:5170/settings", { waitUntil: "networkidle" });
await page.getByRole("heading", { name: "Backups and recovery" }).waitFor();
await page.getByText("Every 24h0m0s").waitFor();
await page.getByText("14 archives").waitFor();
await page.getByText("Local JSON + file storage").first().waitFor();
await page.locator(".cc-backup-row").first().waitFor();

const initialRows = await page.locator(".cc-backup-row").count();
const createResponse = page.waitForResponse(
  (response) => response.url().endsWith("/api/admin/backups") && response.request().method() === "POST" && response.ok()
);
await page.getByRole("button", { name: "Create backup" }).click();
await createResponse;
await page.waitForFunction((count) => document.querySelectorAll(".cc-backup-row").length > count, initialRows);

const latestName = (await page.locator(".cc-backup-row").first().locator(".cc-backup-name strong").textContent())?.trim();
const downloadResponse = page.waitForResponse(
  (response) => response.url().includes("/api/admin/backups/") && response.url().endsWith("/content") && response.ok()
);
const downloadEvent = page.waitForEvent("download");
await page.locator(".cc-backup-row").first().getByRole("button", { name: /^Download / }).click();
const [download] = await Promise.all([downloadEvent, downloadResponse]);

const desktop = await page.evaluate(() => {
  const section = document.querySelector(".cc-platform-backups");
  const row = document.querySelector(".cc-backup-row");
  return {
    sectionVisible: Boolean(section && section.getBoundingClientRect().height > 0),
    rowColumns: row ? getComputedStyle(row).gridTemplateColumns : "",
    overflow: document.documentElement.scrollWidth - window.innerWidth
  };
});
await page.screenshot({ path: "../.logs/settings-backups-desktop.png", fullPage: true });

await page.setViewportSize({ width: 390, height: 844 });
await page.reload({ waitUntil: "networkidle" });
await page.getByRole("heading", { name: "Backups and recovery" }).waitFor();
const mobile = await page.evaluate(() => {
  const section = document.querySelector(".cc-platform-backups");
  const policy = document.querySelector(".cc-backup-policy");
  const row = document.querySelector(".cc-backup-row");
  const button = row?.querySelector("button");
  const rowRect = row?.getBoundingClientRect();
  const buttonRect = button?.getBoundingClientRect();
  return {
    innerWidth: window.innerWidth,
    scrollWidth: document.documentElement.scrollWidth,
    policyColumns: policy ? getComputedStyle(policy).gridTemplateColumns : "",
    rowColumns: row ? getComputedStyle(row).gridTemplateColumns : "",
    sectionVisible: Boolean(section && section.getBoundingClientRect().height > 0),
    downloadFits: Boolean(rowRect && buttonRect && buttonRect.right <= rowRect.right + 1 && buttonRect.left >= rowRect.left - 1)
  };
});
await page.screenshot({ path: "../.logs/settings-backups-mobile.png", fullPage: true });

const result = {
  initialRows,
  currentRows: await page.locator(".cc-backup-row").count(),
  latestName,
  suggestedFilename: download.suggestedFilename(),
  listRequests,
  createRequests,
  downloadRequests,
  desktop,
  mobile,
  runtimeErrors
};
console.log(JSON.stringify(result, null, 2));

if (!latestName?.startsWith("chakuchuri-") || !latestName.endsWith(".zip")) throw new Error("Latest archive name is invalid.");
if (download.suggestedFilename() !== latestName) throw new Error("Downloaded filename does not match the latest archive.");
if (listRequests < 2 || createRequests !== 1 || downloadRequests !== 1) throw new Error("The backup API flow was incomplete.");
if (!desktop.sectionVisible || desktop.overflow > 0) throw new Error("Desktop backup section is not laid out correctly.");
if (!mobile.sectionVisible || mobile.scrollWidth > mobile.innerWidth || !mobile.downloadFits) throw new Error("Mobile backup section overflows.");
if (runtimeErrors.length) throw new Error(`Runtime errors: ${runtimeErrors.join(" | ")}`);

await browser.close();
