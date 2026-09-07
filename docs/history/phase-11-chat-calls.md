# Phase 11: Chat + Calls

## Current Build

- Customer and admin messaging is available in a messenger-style workspace.
- Messages store author role, unread counters, conversation activity, and up to five secured photo/file attachments per message.
- Online and last-online presence is visible for support and customer users.
- Customers and admins place direct audio calls from the phone icon in a conversation.
- Calls use a direct lifecycle: Ringing, In call, Completed, Missed, Declined, or Cancelled. There is no request or waiting queue.
- Incoming and outgoing calls have ringtone/ringback feedback and a global call surface across portal pages.
- Accepted calls enter the browser audio room automatically.
- Admin has a separate call log for incoming, outgoing, answered, missed, declined, cancelled, time, person, and duration.
- WebRTC offer, answer, ICE, and media-state signaling runs over WebSocket with HTTP polling fallback.
- Call signals and history are persisted in JSON development storage and PostgreSQL.

## Remaining Realtime Hardening

- Add Redis fanout for multi-instance online state, messages, and call signaling.
- Add TURN server configuration for production call reliability outside local networks.
- Add push notifications and background incoming-call delivery.
- Add call diagnostics and explicit admin availability controls.
- Add native Flutter WebRTC calling after Android tooling is validated.

## Mobile Dependency

Flutter already shares authentication and workspace APIs. Its support screen must now adopt the direct-call lifecycle, then native WebRTC and push notifications can be connected without carrying forward the retired queue model.