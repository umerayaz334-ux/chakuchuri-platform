import type { AccessOptions, Customer, Envelope, FileRecord, PageInfo, Rate, Session, User } from "./types";
import { apiMutationEventName } from "./workspace-realtime";
import { createBlobLoader } from "./blob-loader";

export async function apiGet<T>(path: string, token?: string): Promise<T> {
  const response = await fetch(path, { headers: token ? { Authorization: `Bearer ${token}` } : undefined });
  return unwrap<T>(response);
}

export async function apiPost<T>(path: string, body: unknown, token?: string): Promise<T> {
  const response = await fetch(path, {
    body: JSON.stringify(body),
    headers: token
      ? { "Content-Type": "application/json", Authorization: `Bearer ${token}` }
      : { "Content-Type": "application/json" },
    method: "POST"
  });
  const data = await unwrap<T>(response);
  emitApiMutation(path, data);
  return data;
}

export async function apiUploadFile(file: File, fields: Record<string, string>, token?: string): Promise<FileRecord> {
  const body = new FormData();
  body.append("file", file);
  Object.entries(fields).forEach(([key, value]) => {
    if (value.trim()) body.append(key, value.trim());
  });

  const response = await fetch("/api/files", {
    body,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    method: "POST"
  });
  return unwrap<FileRecord>(response);
}

export async function apiUploadCustomerDocument(
  file: File,
  fields: { customerId: string; kind: string; label?: string; requestId?: string },
  token?: string
): Promise<{ customer: Customer; file: FileRecord }> {
  const body = new FormData();
  body.append("file", file);
  body.append("kind", fields.kind);
  if (fields.label?.trim()) body.append("label", fields.label.trim());
  if (fields.requestId?.trim()) body.append("requestId", fields.requestId.trim());

  const path = `/api/customers/${encodeURIComponent(fields.customerId)}/documents/upload`;
  const response = await fetch(path, {
    body,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    method: "POST"
  });
  const data = await unwrap<{ customer: Customer; file: FileRecord }>(response);
  emitApiMutation(path, data);
  return data;
}

export async function apiUploadRateSheet(
  file: File,
  fields: Record<string, string>,
  token?: string
): Promise<{ source: FileRecord; rates: Rate[]; count: number }> {
  const body = new FormData();
  body.append("file", file);
  Object.entries(fields).forEach(([key, value]) => {
    if (value.trim()) body.append(key, value.trim());
  });

  const response = await fetch("/api/workflow/ratesheets/import", {
    body,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    method: "POST"
  });
  const data = await unwrap<{ source: FileRecord; rates: Rate[]; count: number }>(response);
  emitApiMutation("/api/workflow/ratesheets/import", data);
  return data;
}

export type ManagedUserPayload = {
  name: string;
  email: string;
  password?: string;
  role: string;
  status: string;
  customerId?: string;
  assignedCustomerIds?: string[];
  permissions: string[];
  pageAccess: string[];
  suspensionNotice?: string;
};

export async function apiUserOptions(token?: string): Promise<AccessOptions> {
  return apiGet<AccessOptions>("/api/auth/users/options", token);
}

export async function apiListUsers(token?: string, options: { offset?: number; q?: string; role?: string; status?: string } = {}): Promise<{ users: User[]; pagination: PageInfo }> {
  const query = new URLSearchParams({
    limit: "20",
    offset: String(options.offset || 0),
    ...(options.q ? { q: options.q } : {}),
    ...(options.role && options.role !== "All roles" ? { role: options.role } : {}),
    ...(options.status && options.status !== "All statuses" ? { status: options.status } : {})
  });
  return apiGet<{ users: User[]; pagination: PageInfo }>(`/api/auth/users?${query}`, token);
}

export async function apiCreateUser(payload: ManagedUserPayload, token?: string): Promise<User> {
  return apiPost<User>("/api/auth/users", payload, token);
}

export async function apiUpdateUser(id: string, payload: ManagedUserPayload, token?: string): Promise<User> {
  return apiPost<User>(`/api/auth/users/${id}`, payload, token);
}

export async function apiDeleteUser(id: string, token?: string): Promise<{ deleted: string }> {
  return apiPost<{ deleted: string }>(`/api/auth/users/${id}/delete`, {}, token);
}

export const apiBlob = createBlobLoader();

function emitApiMutation(path: string, data: unknown) {
  window.dispatchEvent(new CustomEvent(apiMutationEventName, { detail: { path, data } }));
}

async function unwrap<T>(response: Response): Promise<T> {
  const raw = await response.text();
  let envelope: Envelope<T>;

  try {
    envelope = JSON.parse(raw) as Envelope<T>;
  } catch {
    throw new Error(response.ok ? "The server returned an invalid response." : `Request failed (${response.status}). Please refresh and try again.`);
  }

  if (!response.ok || !envelope.ok || envelope.data === undefined) {
    throw new Error(envelope.error?.message || "Request failed.");
  }
  return envelope.data;
}

export function readSession(): Session | null {
  try {
    const raw = localStorage.getItem("cc_session");
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}
