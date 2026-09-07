import type { PageInfo, Shipment } from "./types";

export type ShippingSurcharge = {
  key: string;
  label: string;
  amount: number;
  currency: string;
  description?: string;
};

export type ShippingServiceSummary = {
  id: string;
  name: string;
  etaMinDays: number;
  etaMaxDays: number;
  fixedRateCount: number;
  bandCount: number;
  minimumAmount: number;
};

export type ShippingZoneCount = { zone: string; count: number };

export type ShippingRateBook = {
  id: string;
  name: string;
  carrier: string;
  country: string;
  currency: string;
  effectiveDate: string;
  version: number;
  status: string;
  sourceFileId?: string;
  sourceName?: string;
  importedAt: string;
  importedBy: string;
  dimensionalDivisor: number;
  maxBoxWeightKg: number;
  postalPrefixCount: number;
  specialPrefixCount: number;
  services: ShippingServiceSummary[];
  zoneCounts: ShippingZoneCount[];
  surcharges: ShippingSurcharge[];
  warnings?: string[];
};

export type ShippingRateSnapshot = {
  books: ShippingRateBook[];
  activeBooks: number;
  activeServices: number;
  latestEffective?: string;
  pagination: PageInfo;
};

export type ShippingLookupRequest = {
  country: string;
  postalCode: string;
  dutyMode: "duty_paid" | "non_duty_paid";
  weightKg: number;
  lengthCm: number;
  widthCm: number;
  heightCm: number;
  packages: number;
  includePsw: boolean;
  carrier?: string;
  serviceId?: string;
};

export type ShippingChargeLine = { label: string; amount: number; currency: string };

export type ShippingRateOption = {
  rateBookId: string;
  serviceId: string;
  carrier: string;
  service: string;
  currency: string;
  effectiveDate: string;
  etaMinDays: number;
  etaMaxDays: number;
  zone: string;
  region: string;
  dutyMode: string;
  packages: number;
  actualWeightKg: number;
  volumetricWeightKg: number;
  chargeableWeightKg: number;
  rateMode: string;
  rateWeightKg: number;
  unitRate: number;
  baseAmount: number;
  charges: ShippingChargeLine[];
  totalAmount: number;
  requiresReview: boolean;
  reviewReasons?: string[];
};

export type ShippingLookupResponse = {
  postalPrefix: string;
  zone: string;
  region: string;
  options: ShippingRateOption[];
  warnings?: string[];
};

export type ShippingBookingRequest = {
  lookup: ShippingLookupRequest;
  recipientName: string;
  phone: string;
  addressLine1: string;
  addressLine2: string;
  city: string;
  state: string;
  postalCode: string;
  contents: string;
};

export type ShippingImportResult = {
  book: ShippingRateBook;
  snapshot: ShippingRateSnapshot;
  source: { id: string; originalName: string };
};

export type ShippingBookResult = Shipment;
