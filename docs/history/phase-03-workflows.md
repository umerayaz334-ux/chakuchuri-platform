# Phase 3 Workflows

Phase 3 turns the portal shell into working customer and admin workflows.

## Backend Module

`internal/workflow` owns quotations, manufacturing orders, shipping requests, rate sheets, payments, products, messages and direct calls behind one repository contract.

## Active Endpoints

- `GET /api/workspace`
- `POST /api/workflow/quotes`
- `POST /api/workflow/quotes/{id}/price`
- `POST /api/workflow/quotes/{id}/accept`
- `POST /api/workflow/quotes/{id}/reject`
- `POST /api/workflow/manufacturing/{id}/stage`
- `POST /api/workflow/shipping`
- `POST /api/workflow/payments`
- `POST /api/workflow/payments/{id}/confirm`
- `POST /api/workflow/products`
- `POST /api/workflow/messages`
- `POST /api/workflow/calls`
- `POST /api/workflow/calls/{id}/status`
- `POST /api/workflow/calls/{id}/end`
- `GET|POST /api/workflow/calls/{id}/signals`
- `GET /api/realtime/calls/{id}` using WebSocket upgrade

## Portal Coverage

- Customers request quotes for catalog or custom products.
- Admin staff price quotations and accepted quotes become manufacturing orders.
- Admin staff advance production stages and estimated dates.
- Customers request shipping for manufactured goods or outside products.
- Payment proof and customer balances flow through the ledger.
- Messenger conversations support protected photos and files.
- Customers and staff can place direct audio calls, answer, decline, cancel and hang up.
- Call outcomes and durations remain available in admin call history.
