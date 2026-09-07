import type { Customer, UserPresence } from "./types";

export function applyCustomerPresence(customers: Customer[], presence: UserPresence[]): Customer[] {
  const byCustomer = new Map<string, { online: boolean; lastOnline: string }>();
  const timestamp = (value: string) => Date.parse(value) || 0;
  for (const person of presence) {
    if (!person.customerId || person.role.toLowerCase() !== "customer") continue;
    const previous = byCustomer.get(person.customerId);
    byCustomer.set(person.customerId, {
      online: Boolean(previous?.online || person.online),
      lastOnline: !previous || timestamp(person.lastOnline) > timestamp(previous.lastOnline)
        ? person.lastOnline : previous.lastOnline
    });
  }
  return customers.map((customer) => {
    const status = byCustomer.get(customer.id);
    if (!status) return customer;
    const lastOnline = timestamp(status.lastOnline) > timestamp(customer.lastOnline) || !customer.lastOnline
      ? status.lastOnline : customer.lastOnline;
    if (customer.online === status.online && customer.lastOnline === lastOnline) return customer;
    return { ...customer, online: status.online, lastOnline };
  });
}
