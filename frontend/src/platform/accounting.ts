import type { Workspace } from "./types";

export const accountingInvalidatedEvent = "cc:accounting-invalidated";
export const financialScopes = new Set(["manufacturing", "shipping", "payments", "ledger"]);

// Every customer balance surface reads the same authoritative server summary.
export function accountPosition(workspace: Workspace) {
  const m = workspace.metrics || {};
  const ready = Object.prototype.hasOwnProperty.call(m, "ledgerBalance");
  const balance = ready ? Number(m.ledgerBalance) : 0;
  return {
    ready, balance,
    due: Math.max(0, balance), credit: Math.max(0, -balance),
    charges: Number(m.totalCharges || 0),
    adjustments: Number(m.totalAdjustments || 0),
    received: Number(m.totalReceived || 0),
    pending: Number(m.pendingPaymentAmount || 0)
  };
}
