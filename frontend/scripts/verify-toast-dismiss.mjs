import { chromium } from "playwright";

const browser = await chromium.launch({
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
  headless: true
});
const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await context.newPage();
const runtimeErrors = [];

page.on("pageerror", (error) => runtimeErrors.push(error.message));
page.on("console", (message) => {
  if (message.type() === "error" && !message.text().includes("favicon.ico")) runtimeErrors.push(message.text());
});

await page.goto("http://127.0.0.1:5170/login", { waitUntil: "networkidle" });
await page.getByLabel("Email").fill("admin@chakuchuri.pk");
await page.getByLabel("Password").fill("admin123");
await page.getByRole("button", { name: "Sign in" }).click();

const toast = page.locator(".cc-toast");
await toast.waitFor({ state: "visible" });
const text = (await toast.textContent())?.trim();
const shownAt = Date.now();
await toast.waitFor({ state: "detached", timeout: 5500 });
const elapsed = Date.now() - shownAt;

console.log(JSON.stringify({ text, elapsed, remainingToasts: await toast.count(), runtimeErrors }, null, 2));
if (text !== "Signed in as ChakuChuri Admin.") throw new Error("The expected sign-in notice was not shown.");
if (elapsed < 3500 || elapsed > 5200) throw new Error(`Toast dismissal timing was outside the expected range: ${elapsed}ms.`);
if (runtimeErrors.length) throw new Error(`Runtime errors: ${runtimeErrors.join(" | ")}`);

await browser.close();
