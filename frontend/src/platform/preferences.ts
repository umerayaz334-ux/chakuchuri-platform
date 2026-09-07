import { useEffect } from "react";
import type { PlatformSettings } from "./types";

export type PlatformLanguage = "en" | "ur";

export const defaultPlatformSettings: PlatformSettings = {
  themePreset: "Jade",
  primaryColor: "#0f766e",
  accentColor: "#c58b35",
  surfaceColor: "#f6f8f7",
  defaultLanguage: "en",
  enabledLanguages: ["en", "ur"],
  allowUserLanguageChoice: true
};

export const themePresets = [
  { name: "Jade", primaryColor: "#0f766e", accentColor: "#c58b35", surfaceColor: "#f6f8f7" },
  { name: "Cobalt", primaryColor: "#155eaa", accentColor: "#c75b4c", surfaceColor: "#f5f7fa" },
  { name: "Graphite", primaryColor: "#263a35", accentColor: "#2a8f78", surfaceColor: "#f4f6f5" },
  { name: "Burgundy", primaryColor: "#8a3148", accentColor: "#4f7b67", surfaceColor: "#f8f6f6" }
] as const;

const settingsCacheKey = "cc_platform_settings";

export function normalizePlatformSettings(value?: Partial<PlatformSettings> | null): PlatformSettings {
  return {
    ...defaultPlatformSettings,
    ...value,
    primaryColor: validColor(value?.primaryColor) || defaultPlatformSettings.primaryColor,
    accentColor: validColor(value?.accentColor) || defaultPlatformSettings.accentColor,
    surfaceColor: validColor(value?.surfaceColor) || defaultPlatformSettings.surfaceColor,
    defaultLanguage: value?.defaultLanguage === "ur" ? "ur" : "en",
    enabledLanguages: ["en", "ur"]
  };
}

export function readCachedPlatformSettings() {
  try {
    return normalizePlatformSettings(JSON.parse(localStorage.getItem(settingsCacheKey) || "null"));
  } catch {
    return defaultPlatformSettings;
  }
}

export function cachePlatformSettings(settings: PlatformSettings) {
  localStorage.setItem(settingsCacheKey, JSON.stringify(normalizePlatformSettings(settings)));
}

export function applyPlatformTheme(settings: PlatformSettings) {
  const next = normalizePlatformSettings(settings);
  const root = document.documentElement;
  root.dataset.ccTheme = next.themePreset.toLowerCase();
  root.style.setProperty("--cc-theme-primary", next.primaryColor);
  root.style.setProperty("--cc-theme-accent", next.accentColor);
  root.style.setProperty("--cc-theme-surface", next.surfaceColor);
  root.style.setProperty("--cc-theme-on-primary", readableText(next.primaryColor));
}

export function readLanguageChoice(settings: PlatformSettings, userId?: string): PlatformLanguage {
  if (!settings.allowUserLanguageChoice) return settings.defaultLanguage;
  const saved = localStorage.getItem(languageStorageKey(userId));
  return saved === "ur" || saved === "en" ? saved : settings.defaultLanguage;
}

export function saveLanguageChoice(language: PlatformLanguage, userId?: string) {
  localStorage.setItem(languageStorageKey(userId), language);
}

function languageStorageKey(userId?: string) {
  return userId ? `cc_language_${userId}` : "cc_language_public";
}

function validColor(value?: string) {
  const color = (value || "").trim().toLowerCase();
  return /^#[0-9a-f]{6}$/.test(color) ? color : "";
}

function readableText(color: string) {
  const value = color.replace("#", "");
  const red = Number.parseInt(value.slice(0, 2), 16);
  const green = Number.parseInt(value.slice(2, 4), 16);
  const blue = Number.parseInt(value.slice(4, 6), 16);
  return (red * 299 + green * 587 + blue * 114) / 1000 > 150 ? "#111714" : "#ffffff";
}

const translations: Record<string, string> = {
  Workspace: "ورک اسپیس",
  Command: "کنٹرول",
  Home: "ہوم",
  Customers: "کسٹمرز",
  "Users & Access": "صارفین اور رسائی",
  Quotations: "کوٹیشنز",
  "Get Quote": "کوٹیشن حاصل کریں",
  Products: "مصنوعات",
  Manufacturing: "مینوفیکچرنگ",
  Shipping: "شپنگ",
  Payments: "ادائیگیاں",
  "Messages & Calls": "پیغامات اور کالز",
  Messages: "پیغامات",
  Settings: "ترتیبات",
  "Sign out": "سائن آؤٹ",
  "Company workspace": "کمپنی ورک اسپیس",
  Refresh: "تازہ کریں",
  "Refresh page data": "صفحہ تازہ کریں",
  Notifications: "اطلاعات",
  Updates: "تازہ معلومات",
  "Mark all read": "سب کو پڑھا ہوا کریں",
  "Close updates": "تازہ معلومات بند کریں",
  "No new notifications": "کوئی نئی اطلاع نہیں",
  "You're all caught up": "تمام اطلاعات دیکھ لی گئی ہیں",
  "Platform preferences, reliability and version controls.": "پلیٹ فارم کی ترجیحات، قابل اعتماد نظام اور ورژن کنٹرول۔",
  "Customer accounts, balances and service activity.": "کسٹمر اکاؤنٹس، بقایا جات اور سروس سرگرمی۔",
  "Create users, set roles and control page access.": "صارف بنائیں، کردار مقرر کریں اور صفحات کی رسائی کنٹرول کریں۔",
  "Price requests and convert accepted quotes into orders.": "درخواستوں کی قیمت مقرر کریں اور منظور شدہ کوٹیشن کو آرڈر بنائیں۔",
  "Move confirmed work through production and quality control.": "تصدیق شدہ کام کو پیداوار اور کوالٹی کنٹرول سے گزاریں۔",
  "Manage courier requests, rates and delivery progress.": "کورئیر درخواستیں، نرخ اور ترسیل کی پیش رفت منظم کریں۔",
  "Confirm proofs and review customer balances.": "ادائیگی کے ثبوت کی تصدیق اور کسٹمر بقایا جات کا جائزہ لیں۔",
  "Live conversations, photos, files and direct audio calls.": "لائیو گفتگو، تصاویر، فائلیں اور براہ راست آڈیو کالز۔",
  "Active orders, balances and the next action in one view.": "فعال آرڈرز، بقایا جات اور اگلا قدم ایک ہی جگہ۔",
  "Request pricing for a catalog item or a new design.": "کیٹلاگ آئٹم یا نئے ڈیزائن کی قیمت طلب کریں۔",
  "Manage your own stock and product references.": "اپنا اسٹاک اور مصنوعات کا ریکارڈ منظم کریں۔",
  "Track confirmed work from deposit through completion.": "تصدیق شدہ کام کو ڈپازٹ سے تکمیل تک ٹریک کریں۔",
  "Request courier service and follow every shipment.": "کورئیر سروس کی درخواست دیں اور ہر شپمنٹ ٹریک کریں۔",
  "Review balances and upload payment proof.": "بقایا رقم دیکھیں اور ادائیگی کا ثبوت اپ لوڈ کریں۔",
  "Incoming, outgoing, answered and missed call history.": "آنے والی، جانے والی، موصول اور مس کالز کی تاریخ۔",
  English: "انگریزی",
  Urdu: "اردو",
  Appearance: "ظاہری شکل",
  Language: "زبان",
  "Default language": "پہلے سے منتخب زبان",
  "User language switch": "صارف کے لیے زبان کی تبدیلی",
  Theme: "تھیم",
  "Primary color": "بنیادی رنگ",
  "Accent color": "نمایاں رنگ",
  "Surface color": "پس منظر کا رنگ",
  "Save settings": "ترتیبات محفوظ کریں",
  Reset: "دوبارہ ترتیب دیں",
  "Live preview": "براہ راست پیش منظر",
  Website: "ویب سائٹ",
  "Admin portal": "ایڈمن پورٹل",
  "Customer portal": "کسٹمر پورٹل",
  Enabled: "فعال",
  Disabled: "غیر فعال",
  "Platform settings saved.": "پلیٹ فارم کی ترتیبات محفوظ ہو گئیں۔",
  Login: "لاگ اِن",
  Signup: "سائن اپ",
  "Sign in": "سائن اِن",
  "Create account": "اکاؤنٹ بنائیں",
  Email: "ای میل",
  Password: "پاس ورڈ",
  Name: "نام",
  Phone: "فون",
  Country: "ملک",
  Company: "کمپنی",
  Contact: "رابطہ",
  Portal: "پورٹل",
  Services: "سروسز",
  Support: "سپورٹ",
  "Welcome back": "خوش آمدید",
  "Secure portal access": "محفوظ پورٹل رسائی",
  "Customer onboarding": "کسٹمر رجسٹریشن",
  "Create portal": "پورٹل بنائیں",
  "Admin demo": "ایڈمن ڈیمو",
  "Customer demo": "کسٹمر ڈیمو",
  Active: "فعال",
  Paused: "روکا ہوا",
  Blocked: "بلاک شدہ",
  Pending: "زیر التوا",
  Requested: "درخواست موصول",
  Priced: "قیمت مقرر",
  Confirmed: "تصدیق شدہ",
  Rejected: "مسترد",
  Production: "پیداوار",
  "Quality check": "کوالٹی چیک",
  Ready: "تیار",
  Completed: "مکمل",
  Cancelled: "منسوخ",
  "In transit": "راستے میں",
  Delivered: "پہنچایا گیا",
  Waiting: "انتظار میں",
  "Waiting confirmation": "تصدیق کا انتظار",
  Open: "کھولیں",
  Close: "بند کریں",
  View: "دیکھیں",
  Edit: "ترمیم",
  Delete: "حذف کریں",
  Cancel: "منسوخ کریں",
  Submit: "جمع کریں",
  Save: "محفوظ کریں",
  Create: "بنائیں",
  Update: "تازہ کریں",
  Amount: "رقم",
  Balance: "بقایا",
  Total: "کل",
  Deposit: "ڈپازٹ",
  Due: "واجب الادا",
  Status: "حالت",
  Progress: "پیش رفت",
  Quantity: "تعداد",
  Product: "مصنوعہ",
  Order: "آرڈر",
  Orders: "آرڈرز",
  Quote: "کوٹیشن",
  Payment: "ادائیگی",
  Courier: "کورئیر",
  Destination: "منزل",
  Tracking: "ٹریکنگ",
  Notes: "نوٹس",
  "Online now": "ابھی آن لائن",
  Never: "کبھی نہیں",
  Recently: "حال ہی میں",
  "Just now": "ابھی"
};

const sourceText = new WeakMap<Text, string>();
const renderedText = new WeakMap<Text, string>();
const sourceAttributes = new WeakMap<Element, Map<string, string>>();
const renderedAttributes = new WeakMap<Element, Map<string, string>>();

export function translateText(value: string, language: PlatformLanguage) {
  if (language === "en") return value;
  const leading = value.match(/^\s*/)?.[0] || "";
  const trailing = value.match(/\s*$/)?.[0] || "";
  const content = value.trim();
  if (!content) return value;
  const exact = translations[content];
  if (exact) return `${leading}${exact}${trailing}`;

  const greeting = content.match(/^Good (morning|afternoon|evening), (.+)!$/);
  if (greeting) {
    const salutation = greeting[1] === "morning" ? "صبح بخیر" : greeting[1] === "afternoon" ? "دوپہر بخیر" : "شام بخیر";
    return `${leading}${salutation}، ${greeting[2]}!${trailing}`;
  }
  const count = content.match(/^(\d+) (unread|orders|pages|actions|shipments)$/);
  if (count) {
    const labels: Record<string, string> = { unread: "غیر پڑھی", orders: "آرڈرز", pages: "صفحات", actions: "اختیارات", shipments: "شپمنٹس" };
    return `${leading}${count[1]} ${labels[count[2]]}${trailing}`;
  }
  return value;
}

export function useAutomaticTranslation(language: PlatformLanguage) {
  useEffect(() => {
    document.documentElement.lang = language;
    document.documentElement.dir = language === "ur" ? "rtl" : "ltr";
    let applying = false;
    let queued = false;
    const apply = () => {
      queued = false;
      applying = true;
      translateTree(document.body, language);
      applying = false;
    };
    const queueApply = () => {
      if (applying || queued) return;
      queued = true;
      queueMicrotask(apply);
    };
    apply();
    const observer = new MutationObserver(queueApply);
    observer.observe(document.body, {
      subtree: true,
      childList: true,
      characterData: true,
      attributes: true,
      attributeFilter: ["placeholder", "title", "aria-label"]
    });
    return () => observer.disconnect();
  }, [language]);
}

function translateTree(root: HTMLElement, language: PlatformLanguage) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let node = walker.nextNode() as Text | null;
  while (node) {
    const parent = node.parentElement;
    if (parent && !parent.closest("script, style, code, pre, [data-no-translate]")) translateTextNode(node, language);
    node = walker.nextNode() as Text | null;
  }
  root.querySelectorAll<HTMLElement>("[placeholder], [title], [aria-label]").forEach((element) => {
    if (!element.closest("[data-no-translate]")) translateAttributes(element, language);
  });
}

function translateTextNode(node: Text, language: PlatformLanguage) {
  const current = node.nodeValue || "";
  const lastRendered = renderedText.get(node);
  if (lastRendered === undefined || current !== lastRendered) sourceText.set(node, current);
  const source = sourceText.get(node) || current;
  const next = translateText(source, language);
  renderedText.set(node, next);
  if (current !== next) node.nodeValue = next;
}

function translateAttributes(element: Element, language: PlatformLanguage) {
  const sources = sourceAttributes.get(element) || new Map<string, string>();
  const rendered = renderedAttributes.get(element) || new Map<string, string>();
  ["placeholder", "title", "aria-label"].forEach((attribute) => {
    const current = element.getAttribute(attribute);
    if (current === null) return;
    if (!rendered.has(attribute) || rendered.get(attribute) !== current) sources.set(attribute, current);
    const next = translateText(sources.get(attribute) || current, language);
    rendered.set(attribute, next);
    if (current !== next) element.setAttribute(attribute, next);
  });
  sourceAttributes.set(element, sources);
  renderedAttributes.set(element, rendered);
}
