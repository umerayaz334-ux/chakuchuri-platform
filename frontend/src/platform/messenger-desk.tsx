import { type ChangeEvent, type FormEvent, type KeyboardEvent, useEffect, useMemo, useRef, useState } from "react";
import { CheckCheck, FileText, LoaderCircle, Paperclip, Phone, Search, SendHorizontal, X } from "lucide-react";
import { apiGet, apiPost, apiUploadFile } from "./api";
import { Empty, MessengerBubbles, conversationPreview, ownMessageReceipt, isOwnMessengerMessage } from "./shared";
import { ProfileAvatar } from "./profile-avatar";
import type { Call, Conversation, ConversationMessagePage, Customer, Session, Workspace } from "./types";
import { usePresenceNow } from "./use-presence-now";
import { findCustomerPresence, findSupportPresence, messageFromError, resolveCustomerPresence, resolvePresenceStatus } from "./utils";

type MessengerDeskProps = {
  mode: "admin" | "customer";
  workspace: Workspace;
  session: Session;
  customers?: Customer[];
  onAction: (message: string) => Promise<void>;
};

export function MessengerDesk({ mode, workspace, session, customers = [], onAction }: MessengerDeskProps) {
  const admin = mode === "admin";
  const [selectedId, setSelectedId] = useState("");
  const [search, setSearch] = useState("");
  const [callBusy, setCallBusy] = useState(false);
  const [callError, setCallError] = useState("");
  const [unreadCount, setUnreadCount] = useState(0);
  const [historyBusy, setHistoryBusy] = useState(false);
  const [messageHistory, setMessageHistory] = useState<Record<string, ConversationMessagePage>>({});
  const now = usePresenceNow();
  const markedRead = useRef(new Set<string>());
  const conversations = useMemo(
    () => [...workspace.conversations].sort((left, right) => (right.lastMessageAt || "").localeCompare(left.lastMessageAt || "")),
    [workspace.conversations]
  );

  useEffect(() => {
    if (!admin || conversations.length === 0) {
      if (admin && selectedId) setSelectedId("");
      return;
    }
    const requestedCustomerId = sessionStorage.getItem("cc_message_customer_id");
    const requestedConversation = conversations.find((conversation) => conversation.customerId === requestedCustomerId);
    if (requestedConversation) {
      sessionStorage.removeItem("cc_message_customer_id");
      setSelectedId(requestedConversation.id);
      return;
    }
    if (!selectedId || !conversations.some((conversation) => conversation.id === selectedId)) {
      setSelectedId(conversations[0].id);
    }
  }, [admin, conversations, selectedId]);

  const selectedBase = admin
    ? conversations.find((conversation) => conversation.id === selectedId) || conversations[0]
    : conversations[0];
  const selected = selectedBase ? conversationWithHistory(selectedBase, messageHistory[selectedBase.id]) : undefined;
  const selectedUnread = selected ? (admin ? selected.unreadForAdmin || 0 : selected.unreadForCustomer || 0) : 0;
  const customerName = (customerId?: string) => customers.find((customer) => customer.id === customerId)?.companyName || customerId || "Customer";
  const contactName = admin ? customerName(selected?.customerId) : "ChakuChuri support";
  const customerId = admin ? selected?.customerId : session.user.customerId;
  const customerFallback = customers.find((customer) => customer.id === customerId);
  const contactPresence = admin
    ? findCustomerPresence(workspace.presence, customerId, customerFallback ? { email: customerFallback.email, companyName: customerFallback.companyName } : undefined, now)
    : findSupportPresence(workspace.presence, now);
  const contactStatus = admin
    ? resolveCustomerPresence(workspace.presence, customerFallback || { id: customerId }, now)
    : resolvePresenceStatus(contactPresence, undefined, now);
  const contactOnline = contactStatus.online;
  const contactStatusLabel = contactStatus.label;
  const activeForContact = workspace.calls.find(
    (call) => call.customerId === customerId && (call.status === "Ringing" || call.status === "In call")
  );
  const visibleConversations = conversations.filter((conversation) => {
    const query = search.trim().toLowerCase();
    if (!query) return true;
    return customerName(conversation.customerId).toLowerCase().includes(query) || conversationPreview(conversation).toLowerCase().includes(query);
  });

  useEffect(() => {
    if (!selected) {
      setUnreadCount(0);
      return;
    }
    const unread = admin ? selected.unreadForAdmin || 0 : selected.unreadForCustomer || 0;
    setUnreadCount(unread > 0 ? unread : 0);
    if (unread <= 0) return;
    const timer = window.setTimeout(() => setUnreadCount(0), 1400);
    return () => window.clearTimeout(timer);
  }, [admin, selected?.id]);

  useEffect(() => {
    if (!selected || selectedUnread < 1) return;
    const readKey = `${selected.id}:${selected.lastMessageAt || ""}:${selectedUnread}`;
    if (markedRead.current.has(readKey)) return;
    markedRead.current.add(readKey);
    apiPost<Conversation>(`/api/workflow/messages/${selected.id}/read`, {}, session.token)
      .catch(() => markedRead.current.delete(readKey));
  }, [selected?.id, selected?.lastMessageAt, selectedUnread, session.token]);

  const startCall = async () => {
    if (!customerId || callBusy || activeForContact || !contactOnline) return;
    setCallBusy(true);
    setCallError("");
    try {
      await apiPost<Call>(
        "/api/workflow/calls",
        {
          customerId,
          conversationId: selected?.id || "",
          subject: "Audio call",
          recipientName: admin ? contactName : "ChakuChuri Support"
        },
        session.token
      );
      await onAction("Calling " + contactName + "...");
    } catch (error) {
      setCallError(messageFromError(error, "Call could not be started."));
    } finally {
      setCallBusy(false);
    }
  };

  const loadEarlierMessages = async () => {
    if (!selectedBase || !selected?.messagePagination?.hasMore || historyBusy) return;
    setHistoryBusy(true);
    try {
      const payload = await apiGet<ConversationMessagePage>(
        `/api/workflow/messages/${encodeURIComponent(selectedBase.id)}/page?offset=${selected.messages.length}&limit=20`,
        session.token
      );
      setMessageHistory((current) => ({
        ...current,
        [selectedBase.id]: {
          ...payload,
          messages: mergeMessages(current[selectedBase.id]?.messages || selectedBase.messages, payload.messages)
        }
      }));
    } catch (loadError) {
      setCallError(messageFromError(loadError, "Earlier messages could not be loaded."));
    } finally {
      setHistoryBusy(false);
    }
  };

  const callBlockedReason = activeForContact
    ? "Call already active"
    : !contactOnline
      ? (admin ? "Customer is offline" : "Support is offline")
      : "Start audio call";

  return (
    <section className={"cc-communication-workspace " + (admin ? "admin" : "customer")}>
      {admin && (
        <aside className="cc-conversation-rail">
          <header className="cc-conversation-rail-head">
            <div>
              <span>Inbox</span>
              <strong>Conversations</strong>
            </div>
            <b>{conversations.reduce((total, conversation) => total + (conversation.unreadForAdmin || 0), 0)} unread</b>
          </header>
          {conversations.length > 1 && (
            <label className="cc-conversation-search">
              <Search size={16} />
              <input aria-label="Search conversations" onChange={(event) => setSearch(event.target.value)} placeholder="Search conversations" value={search} />
            </label>
          )}
          <div className="cc-chat-list">
            {visibleConversations.length ? visibleConversations.map((conversation) => {
              const unread = conversation.unreadForAdmin || 0;
              const name = customerName(conversation.customerId);
              const person = findCustomerPresence(workspace.presence, conversation.customerId);
              return (
                <button
                  className={"cc-chat-list-item " + (selected?.id === conversation.id ? "selected " : "") + (unread > 0 ? "unread" : "")}
                  key={conversation.id}
                  onClick={() => setSelectedId(conversation.id)}
                  type="button"
                >
                  <ProfileAvatar className="cc-chat-avatar" token={session.token} user={{ name, profileImageFileId: person?.profileImageFileId }} />
                  <div>
                    <strong>{name}</strong>
                    <small>
                      <ConversationTicks conversation={conversation} perspective={mode} />
                      {conversationPreview(conversation)}
                    </small>
                  </div>
                  {unread > 0 ? <em>{unread}</em> : <time>{conversation.lastMessageAt ? compactTime(conversation.lastMessageAt) : ""}</time>}
                </button>
              );
            }) : <Empty title="No conversations" detail={search ? "No conversation matches your search." : "Customer messages will appear here."} />}
          </div>
        </aside>
      )}

      <div className="cc-messenger-thread cc-modern-thread">
        {admin && !selected ? (
          <Empty title="Select a conversation" detail="Choose a customer thread to view messages." />
        ) : (
          <>
            <header className="cc-messenger-thread-header cc-modern-thread-header">
              <ProfileAvatar className="cc-chat-avatar large" token={session.token} user={{ name: contactName, profileImageFileId: contactPresence?.profileImageFileId }} />
              <div>
                <strong>{contactName}</strong>
                <small>{contactStatusLabel}</small>
              </div>
              <div className="cc-thread-actions">
                <button
                  aria-label={callBlockedReason}
                  className={`cc-thread-icon-button ${!contactOnline && !activeForContact ? "is-offline" : ""}`}
                  disabled={!customerId || callBusy || Boolean(activeForContact) || !contactOnline}
                  onClick={startCall}
                  title={callBlockedReason}
                  type="button"
                >
                  {callBusy ? <LoaderCircle className="cc-spin" size={19} /> : <Phone size={19} />}
                </button>
              </div>
            </header>
            {callError && <div className="cc-messenger-inline-error">{callError}</div>}
            <MessengerBubbles
              conversation={selected}
              loadingEarlier={historyBusy}
              onLoadEarlier={selected?.messagePagination?.hasMore ? loadEarlierMessages : undefined}
              perspective={mode}
              token={session.token}
              unreadCount={unreadCount}
            />
            {customerId && (
              <MessageComposer
                conversationId={selected?.id}
                customerId={customerId}
                onAction={onAction}
                onSent={() => setUnreadCount(0)}
                placeholder={"Message " + contactName}
                session={session}
              />
            )}
          </>
        )}
      </div>
    </section>
  );
}

function MessageComposer({
  conversationId,
  customerId,
  placeholder,
  session,
  onAction,
  onSent
}: {
  conversationId?: string;
  customerId: string;
  placeholder: string;
  session: Session;
  onAction: (message: string) => Promise<void>;
  onSent?: () => void;
}) {
  const [body, setBody] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [sending, setSending] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [error, setError] = useState("");
  const picker = useRef<HTMLInputElement>(null);

  const chooseFiles = (event: ChangeEvent<HTMLInputElement>) => {
    const selected = Array.from(event.target.files || []);
    event.target.value = "";
    if (files.length + selected.length > 5) {
      setError("You can send up to 5 attachments in one message.");
      return;
    }
    const oversized = selected.find((file) => file.size > 10 * 1024 * 1024);
    if (oversized) {
      setError(oversized.name + " is larger than 10 MB.");
      return;
    }
    setError("");
    setFiles((current) => [...current, ...selected]);
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = body.trim();
    if ((!trimmed && files.length === 0) || sending) return;
    setSending(true);
    setUploadProgress(0);
    setError("");
    try {
      const attachmentIds: string[] = [];
      for (const [index, file] of files.entries()) {
        try {
          const uploaded = await apiUploadFile(
            file,
            { customerId, ownerType: "message", ownerId: conversationId || "new-conversation" },
            session.token
          );
          attachmentIds.push(uploaded.id);
          setUploadProgress(index + 1);
        } catch (uploadError) {
          throw new Error(file.name + ": " + messageFromError(uploadError, "Upload failed."));
        }
      }
      await apiPost<Conversation>("/api/workflow/messages", { customerId, body: trimmed, attachmentIds }, session.token);
      setBody("");
      setFiles([]);
      onSent?.();
      await onAction(files.length ? `Sent with ${files.length} attachment${files.length === 1 ? "" : "s"}.` : "Message sent.");
    } catch (caught) {
      setError(messageFromError(caught, "Message could not be sent."));
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      event.currentTarget.form?.requestSubmit();
    }
  };

  return (
    <form className="cc-modern-compose" onSubmit={submit}>
      {files.length > 0 && (
        <div className="cc-compose-attachments">
          <div className="cc-compose-attachment-summary">
            <span>{files.length === 1 ? "1 file ready" : `${files.length} files ready`}</span>
            <small aria-live="polite">{sending ? `Uploading ${uploadProgress} of ${files.length}` : `${files.length} / 5`}</small>
          </div>
          {files.map((file, index) => (
            <div className="cc-compose-file" key={file.name + file.lastModified + index}>
              <FileText size={16} />
              <span>
                <strong>{file.name}</strong>
                <small>{fileSize(file.size)}</small>
              </span>
              <button aria-label={"Remove " + file.name} disabled={sending} onClick={() => setFiles((current) => current.filter((_, itemIndex) => itemIndex !== index))} title="Remove attachment" type="button">
                <X size={15} />
              </button>
            </div>
          ))}
        </div>
      )}
      {error && <div className="cc-compose-error">{error}</div>}
      <div className="cc-compose-row">
        <input
          accept="image/jpeg,image/png,image/webp,application/pdf,text/plain,text/csv,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.zip"
          hidden
          multiple
          onChange={chooseFiles}
          ref={picker}
          type="file"
        />
        <button aria-label="Attach files" className="cc-compose-icon" disabled={sending} onClick={() => picker.current?.click()} title="Attach photos or files" type="button">
          <Paperclip size={19} />
        </button>
        <textarea
          aria-label="Message"
          onChange={(event) => setBody(event.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          rows={1}
          value={body}
        />
        <button aria-label="Send message" className="cc-compose-send" disabled={sending || (!body.trim() && files.length === 0)} title="Send message" type="submit">
          {sending ? <LoaderCircle className="cc-spin" size={18} /> : <SendHorizontal size={18} />}
        </button>
      </div>
    </form>
  );
}

function ConversationTicks({ conversation, perspective }: { conversation?: Conversation; perspective: "admin" | "customer" }) {
  if (!conversation || conversation.messages.length === 0) return null;
  const messages = [...conversation.messages].sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  const last = messages[messages.length - 1];
  if (!isOwnMessengerMessage(last.authorRole || last.author, perspective)) return null;
  const receipt = ownMessageReceipt(messages, last, perspective, conversation);
  return <CheckCheck aria-hidden className={"cc-msg-ticks " + receipt} size={14} strokeWidth={2.4} />;
}

function compactTime(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  const today = new Date();
  if (parsed.toDateString() === today.toDateString()) {
    return parsed.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });
  }
  return parsed.toLocaleDateString("en-GB", { day: "2-digit", month: "short" });
}

function conversationWithHistory(base: Conversation, history?: ConversationMessagePage): Conversation {
  if (!history) return base;
  const messages = mergeMessages(history.messages, base.messages);
  const total = Math.max(base.messagePagination?.total || 0, history.pagination.total, messages.length);
  return {
    ...base,
    messages,
    messagePagination: { loaded: messages.length, total, hasMore: messages.length < total }
  };
}

function mergeMessages(current: Conversation["messages"], incoming: Conversation["messages"]) {
  const byId = new Map(current.map((message) => [message.id, message]));
  incoming.forEach((message) => byId.set(message.id, message));
  return [...byId.values()].sort((left, right) => left.createdAt.localeCompare(right.createdAt));
}

function fileSize(bytes: number) {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return Math.round(bytes / 1024) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}
