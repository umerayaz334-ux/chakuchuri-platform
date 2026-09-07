import { apiGet, apiPost } from "./api";
import type { Envelope, Session, Shipment } from "./types";
import { apiMutationEventName } from "./workspace-realtime";
import type {
  ShippingBookingRequest,
  ShippingImportResult,
  ShippingLookupRequest,
  ShippingLookupResponse,
  ShippingRateBook,
  ShippingRateSnapshot
} from "./shipping-types";

export function getShippingRateSnapshot(session: Session, offset = 0) {
  return apiGet<ShippingRateSnapshot>(`/api/shipping-rates?offset=${offset}&limit=20`, session.token);
}

export function lookupShippingRates(request: ShippingLookupRequest, session: Session) {
  return apiPost<ShippingLookupResponse>("/api/shipping-rates/lookup", request, session.token);
}

export async function bookShippingRate(request: ShippingBookingRequest, session: Session) {
  const shipment = await apiPost<Shipment>("/api/shipping-rates/book", request, session.token);
  window.dispatchEvent(new CustomEvent(apiMutationEventName, { detail: { path: "/api/workflow/shipping", data: shipment } }));
  return shipment;
}

export async function dispatchShipment(id: string, tracking: string, session: Session) {
  const shipment = await apiPost<Shipment>(`/api/workflow/shipping/${encodeURIComponent(id)}/dispatch`, { tracking }, session.token);
  window.dispatchEvent(new CustomEvent(apiMutationEventName, { detail: { path: "/api/workflow/shipping", data: shipment } }));
  return shipment;
}

export function previewShippingRateBook(file: File, fields: Record<string, string>, session: Session) {
  return upload<{ book: ShippingRateBook }>("/api/shipping-rates/preview", file, fields, session.token);
}

export function importShippingRateBook(file: File, fields: Record<string, string>, session: Session) {
  return upload<ShippingImportResult>("/api/shipping-rates/import", file, fields, session.token);
}

export function setShippingRateBookStatus(id: string, active: boolean, session: Session) {
  return apiPost<ShippingRateSnapshot>(`/api/shipping-rates/books/${encodeURIComponent(id)}/status`, { active }, session.token);
}

async function upload<T>(path: string, file: File, fields: Record<string, string>, token: string): Promise<T> {
  const body = new FormData();
  body.append("file", file);
  Object.entries(fields).forEach(([key, value]) => {
    if (value.trim()) body.append(key, value.trim());
  });
  const response = await fetch(path, { method: "POST", body, headers: { Authorization: `Bearer ${token}` } });
  const raw = await response.text();
  let envelope: Envelope<T>;
  try {
    envelope = JSON.parse(raw) as Envelope<T>;
  } catch {
    throw new Error(response.ok ? "The server returned an invalid response." : `Rate sheet request failed (${response.status}).`);
  }
  if (!response.ok || !envelope.ok || envelope.data === undefined) {
    throw new Error(envelope.error?.message || "Rate sheet request failed.");
  }
  return envelope.data;
}
