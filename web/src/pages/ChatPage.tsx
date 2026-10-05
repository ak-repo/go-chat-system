/* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  Fragment,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import {
  conversationInitials,
  getConversation,
  hideConversation,
  listConversations,
  listGroupMembers,
  updateConversationPreferences,
  type Conversation,
  type GroupMember,
} from "../api/conversations";
import {
  compareMessagesChronologically,
  forwardMessage,
  getConversationMessages,
  markConversationRead,
  mergeMessagesChronologically,
  mergeMessagePages,
  mergeMessageStatus,
  optimisticReaction,
  setMessageReaction,
  sortMessagesChronologically,
  type Message,
  type MessageMention,
  type ReactionAggregate,
} from "../api/messages";
import { getPublicUser } from "../api/users";
import { useAuth } from "../context/AuthContext";
import { useSocket } from "../context/SocketContext";
import {
  applyTypingUpdate,
  expireTyping,
  type TypingByConversation,
} from "../context/realtimeState";
import MentionText from "../components/MentionText";
import Icon from "../components/Icon";
import MessageComposer from "../components/MessageComposer";
import MessageReceipt from "../components/MessageReceipt";
import { useConversations } from "../context/ConversationContext";
import { useDialogs } from "../context/DialogContext";

const PAGE_SIZE = 50;
const TYPING_IDLE_MS = 1000;
const REMOTE_TYPING_TTL_MS = 5000;
const QUICK_REACTIONS = ["👍", "❤️", "😂", "😮", "😢", "🎉"] as const;

export default function ChatPage() {
  const { conversationId } = useParams<{ conversationId: string }>();
  const navigate = useNavigate();
  const { refresh, recordMessages } = useConversations();
  const { confirm, askText } = useDialogs();
  const { user } = useAuth();
  const {
    isConnected,
    presence,
    sendConversationEvent,
    onMessage,
    onTyping,
    onAck,
    onError,
    onEvent,
    clearConversationUnread,
  } = useSocket();
  const [conversation, setConversation] = useState<Conversation | null>(null);
  const [title, setTitle] = useState("Conversation");
  const [memberNames, setMemberNames] = useState<Record<string, string>>({});
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [selectedMentions, setSelectedMentions] = useState<
    { kind: "user" | "everyone"; userId?: string; label: string }[]
  >([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const messagesRef = useRef<Message[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [typingUsers, setTypingUsers] = useState<TypingByConversation>({});
  const [replyTo, setReplyTo] = useState<Message | null>(null);
  const [forwarding, setForwarding] = useState<Message | null>(null);
  const [destinations, setDestinations] = useState<Conversation[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const localTyping = useRef(false);
  const typingTarget = useRef<{
    conversationId: string;
    receiverType: "user" | "group";
    receiverId: string;
  } | null>(null);
  const lastReadMessage = useRef<string | null>(null);
  const endRef = useRef<HTMLDivElement>(null);
  const historyRef = useRef<HTMLElement>(null);
  const followLatest = useRef(true);
  const olderScroll = useRef<{ height: number; top: number } | null>(null);
  const [showJump, setShowJump] = useState(false);
  const requestVersion = useRef(0);
  const paginationInFlight = useRef<object | null>(null);
  const offsetRef = useRef(0);
  const forwardDialogRef = useRef<HTMLDivElement>(null);
  const forwardTriggerRef = useRef<HTMLElement | null>(null);
  const [activeMention, setActiveMention] = useState(0);
  const currentUserId = user?.id;

  useEffect(() => {
    messagesRef.current = messages;
  }, [messages]);

  useEffect(() => {
    if (conversationId && conversation?.id === conversationId && !loading) recordMessages(conversationId, messages);
  }, [messages, conversationId, conversation?.id, loading, recordMessages]);
  useLayoutEffect(() => {
    const history = historyRef.current;
    if (!history) return;
    if (olderScroll.current) {
      history.scrollTop = olderScroll.current.top + history.scrollHeight - olderScroll.current.height;
      olderScroll.current = null;
    } else if (followLatest.current) history.scrollTop = history.scrollHeight;
  }, [messages, loading]);

  const peerId =
    conversation?.kind === "direct"
      ? conversation.user_one_id === currentUserId
        ? conversation.user_two_id
        : conversation.user_one_id
      : "";
  const receiverType = conversation?.kind === "group" ? "group" : "user";
  const receiverId = conversation?.kind === "group" ? conversation.id : peerId;

  const load = useCallback(
    async (older = false) => {
      if (!conversationId) return;
      if (older && paginationInFlight.current) return;
      const paginationRequest = older ? {} : null;
      if (paginationRequest) { paginationInFlight.current = paginationRequest; setLoadingOlder(true); }
      const version = requestVersion.current;
      setLoading(!older);
      setError("");
      try {
        const conversationResult = await getConversation(conversationId);
        if (version !== requestVersion.current) return;
        if (!conversationResult.success || !conversationResult.data)
          throw new Error(
            conversationResult.error || "Conversation unavailable",
          );
        const nextConversation = conversationResult.data;
        setConversation(nextConversation);

        if (nextConversation.kind === "group") {
          setTitle(nextConversation.name);
          const members = await listGroupMembers(conversationId);
          if (version !== requestVersion.current) return;
          const groupMembers = members.data?.members ?? [];
          setMembers(groupMembers);
          setMemberNames(
            Object.fromEntries(
              groupMembers.map((member) => [member.user_id, member.username]),
            ),
          );
        } else {
          const nextPeerId =
            nextConversation.user_one_id === currentUserId
              ? nextConversation.user_two_id
              : nextConversation.user_one_id;
          const profile = await getPublicUser(nextPeerId);
          if (version !== requestVersion.current) return;
          setTitle(profile.data?.username ?? nextPeerId);
          setMemberNames({
            [nextPeerId]: profile.data?.username ?? nextPeerId,
          });
          setMembers([
            {
              user_id: nextPeerId,
              username: profile.data?.username ?? nextPeerId,
              role: "member",
              joined_at: nextConversation.created_at,
              modified_at: nextConversation.modified_at,
            },
          ]);
        }

        const nextOffset = older ? offsetRef.current + PAGE_SIZE : 0;
        const result = await getConversationMessages(
          conversationId,
          PAGE_SIZE,
          nextOffset,
        );
        if (version !== requestVersion.current) return;
        if (!result.success || !result.data)
          throw new Error(result.error || "Failed to load messages");
        if (older && historyRef.current) olderScroll.current = { height: historyRef.current.scrollHeight, top: historyRef.current.scrollTop };
        setMessages((current) =>
          older
            ? mergeMessagePages(current, result.data?.messages ?? [])
            : sortMessagesChronologically(result.data?.messages ?? []),
        );
        offsetRef.current = nextOffset;
        setHasMore((result.data.messages?.length ?? 0) >= result.data.limit);
      } catch (cause) {
        if (version === requestVersion.current)
          setError(
            cause instanceof Error
              ? cause.message
              : "Failed to load conversation",
          );
      } finally {
        if (version === requestVersion.current) setLoading(false);
        if (paginationRequest && paginationInFlight.current === paginationRequest) {
          paginationInFlight.current = null;
          setLoadingOlder(false);
        }
      }
    },
    [conversationId, currentUserId],
  );

  // Route changes reset all state owned by the previous conversation.
  useEffect(() => {
    const previousTarget = typingTarget.current;
    if (localTyping.current && previousTarget)
      sendConversationEvent(
        "typing",
        previousTarget.conversationId,
        previousTarget.receiverType,
        previousTarget.receiverId,
        { state: false },
      );
    localTyping.current = false;
    if (typingTimer.current) clearTimeout(typingTimer.current);
    requestVersion.current += 1;
    lastReadMessage.current = null;
    followLatest.current = true;
    olderScroll.current = null;
    setShowJump(false);
    setInput("");
    setConversation(null);
    setMessages([]);
    setMemberNames({});
    setMembers([]);
    setSelectedMentions([]);
    setTypingUsers({});
    setReplyTo(null);
    setForwarding(null);
    setDestinations([]);
    forwardTriggerRef.current = null;
    offsetRef.current = 0;
    paginationInFlight.current = null;
    setLoadingOlder(false);
    setHasMore(false);
    setError("");
  }, [conversationId]);
  useEffect(() => {
    void load();
  }, [conversationId]);
  useEffect(() => {
    if (isConnected && conversationId) void load();
  }, [isConnected]);

  useEffect(() => {
    const timer = setInterval(
      () => setTypingUsers((current) => expireTyping(current, Date.now())),
      1000,
    );
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    typingTarget.current =
      conversationId && receiverId
        ? { conversationId, receiverType, receiverId }
        : null;
  }, [conversationId, receiverId, receiverType]);

  useEffect(
    () => () => {
      if (typingTimer.current) clearTimeout(typingTimer.current);
      const target = typingTarget.current;
      if (localTyping.current && target)
        sendConversationEvent(
          "typing",
          target.conversationId,
          target.receiverType,
          target.receiverId,
          { state: false },
        );
      localTyping.current = false;
    },
    [],
  );

  useEffect(() => {
    if (!isConnected) {
      if (typingTimer.current) clearTimeout(typingTimer.current);
      localTyping.current = false;
    }
  }, [isConnected]);

  useEffect(() => {
    if (!conversationId) return;
    const offMessage = onMessage((event) => {
      const data = event.data;
      if (data.conversation_id !== conversationId || !event.sender_id) return;
      setMessages((current) =>
        mergeMessagesChronologically(current, {
          id: data.message_id,
          sender_id: event.sender_id ?? "",
          receiver_id: event.receiver_id ?? "",
          content: data.content,
          is_group: event.receiver_type === "group",
          created_at: data.timestamp,
          modified_at: data.timestamp,
          conversation_id: data.conversation_id,
          client_message_id: data.client_message_id,
          status: "sent",
          mentions: data.mentions,
        }),
      );
      if (event.sender_id !== user?.id)
        sendConversationEvent(
          "message.delivered",
          conversationId,
          receiverType,
          receiverId,
          { message_id: data.message_id },
        );
    });
    const offTyping = onTyping((event) => {
      if (
        event.data.conversation_id !== conversationId ||
        !event.sender_id ||
        event.sender_id === user?.id
      )
        return;
      setTypingUsers((current) =>
        applyTypingUpdate(
          current,
          conversationId,
          event.sender_id!,
          event.data.state,
          Date.now(),
          REMOTE_TYPING_TTL_MS,
        ),
      );
    });
    const offAck = onAck((event) => {
      if (event.data.conversation_id && event.data.conversation_id !== conversationId)
        return;
      if (!event.data.client_message_id) return;
      setMessages((current) =>
        current.map((message) =>
          message.client_message_id === event.data.client_message_id
            ? {
                ...message,
                id: event.data.server_id ?? message.id,
                status: mergeMessageStatus(message.status, event.data.status),
              }
            : message,
        ),
      );
    });
    const offError = onError((event) => {
      if (
        event.data.conversation_id !== conversationId ||
        !event.data.client_message_id
      )
        return;
      const matched = messagesRef.current.some(
        (message) =>
          message.client_message_id === event.data.client_message_id &&
          message.status === "sending",
      );
      if (!matched) return;
      setError(event.data.message || "Message could not be sent");
      setMessages((current) =>
        current.map((message) =>
          message.client_message_id === event.data.client_message_id &&
          message.status === "sending"
            ? { ...message, status: "failed" }
            : message,
        ),
      );
    });
    const offEdited = onEvent("message.edited", (event) => {
      const data = event.data as {
        message_id?: string;
        content?: string;
        conversation_id?: string;
        mentions?: MessageMention[];
      };
      if (
        data.message_id &&
        data.content &&
        data.conversation_id === conversationId
      )
        setMessages((current) =>
          current.map((message) =>
            message.id === data.message_id
              ? {
                  ...message,
                   content: data.content!,
                   mentions: data.mentions ?? [],
                  edited_at: new Date().toISOString(),
                }
              : message,
          ),
        );
    });
    const offDeleted = onEvent("message.deleted", (event) => {
      const data = event.data as {
        message_id?: string;
        conversation_id?: string;
      };
      if (data.message_id && data.conversation_id === conversationId)
        setMessages((current) =>
          current.filter((message) => message.id !== data.message_id),
        );
    });
    const offReplied = onEvent("message.replied", (event) => {
      const data = event.data as {
        message_id?: string;
        server_id?: string;
        client_message_id?: string;
        content?: string;
        conversation_id?: string;
        mentions?: MessageMention[];
      };
      if (
        data.conversation_id !== conversationId ||
        !data.server_id ||
        !data.message_id ||
        !data.content
      )
        return;
      setMessages((current) =>
        mergeMessagesChronologically(current, {
          id: data.server_id!,
          client_message_id: data.client_message_id,
          sender_id: event.sender_id ?? "",
          receiver_id: event.receiver_id ?? "",
          content: data.content!,
          mentions: data.mentions,
          conversation_id: conversationId,
          reply_to_message_id: data.message_id,
          reply_to: current.find((item) => item.id === data.message_id)
            ? {
                id: data.message_id!,
                sender_id: current.find((item) => item.id === data.message_id)!
                  .sender_id,
                content: current.find((item) => item.id === data.message_id)!
                  .content,
              }
            : undefined,
          created_at: new Date().toISOString(),
          modified_at: new Date().toISOString(),
          reactions: [],
          is_group: event.receiver_type === "group",
          status: "sent",
        }),
      );
    });
    const offReactions = (
      ["message.reaction.added", "message.reaction.removed"] as const
    ).map((name) =>
      onEvent(name, (event) => {
        const data = event.data as {
          message_id?: string;
          conversation_id?: string;
          reactions?: ReactionAggregate[];
        };
        if (
          data.message_id &&
          data.conversation_id === conversationId &&
          data.reactions
        )
          setMessages((current) =>
            current.map((message) =>
              message.id === data.message_id
                ? { ...message, reactions: data.reactions }
                : message,
            ),
          );
      }),
    );
    const offStatus = (["message.delivered", "message.read"] as const).map(
      (name) =>
        onEvent(name, (event) => {
          const data = event.data as {
            message_id?: string;
            conversation_id?: string;
          };
          if (!data.message_id || data.conversation_id !== conversationId)
            return;
          setMessages((current) => {
            const target = current.find(
              (message) => message.id === data.message_id,
            );
            if (!target) return current;
            return current.map((message) =>
              message.sender_id === user?.id &&
              (name === "message.delivered"
                ? message.id === target.id
                : compareMessagesChronologically(message, target) <= 0)
                ? {
                    ...message,
                    status: mergeMessageStatus(
                      message.status,
                      name === "message.read" ? "read" : "delivered",
                    ),
                  }
                : message,
            );
          });
        }),
    );
    return () => {
      offMessage();
      offTyping();
      offAck();
      offError();
      offEdited();
      offDeleted();
      offReplied();
      offReactions.forEach((off) => off());
      offStatus.forEach((off) => off());
    };
  }, [
    conversationId,
    onAck,
    onError,
    onEvent,
    onMessage,
    onTyping,
    receiverId,
    receiverType,
    sendConversationEvent,
    user?.id,
  ]);

  useEffect(() => {
    if (!conversationId || !conversation) return;
    const incoming = [...messages]
      .reverse()
      .find((message) => message.sender_id !== user?.id);
    if (incoming && lastReadMessage.current !== incoming.id) {
      lastReadMessage.current = incoming.id;
      clearConversationUnread(conversationId);
      void markConversationRead(conversationId, incoming.id);
      if (isConnected)
        sendConversationEvent(
          "message.read",
          conversationId,
          receiverType,
          receiverId,
          { message_id: incoming.id },
        );
    }

  }, [
    conversation,
    conversationId,
    isConnected,
    messages,
    receiverId,
    receiverType,
    sendConversationEvent,
    user?.id,
  ]);

  useEffect(() => {
    if (!forwarding) return;
    const dialog = forwardDialogRef.current;
    dialog?.querySelector<HTMLElement>("button")?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setForwarding(null);
        requestAnimationFrame(() => forwardTriggerRef.current?.focus());
        return;
      }
      if (event.key !== "Tab" || !dialog) return;
      const focusable = [...dialog.querySelectorAll<HTMLElement>('button:not([disabled])')];
      if (!focusable.length) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [forwarding]);

  if (!conversationId) return <Navigate to="/conversations" replace />;
  if (loading)
    return (
      <div className="app-shell flex min-h-screen items-center justify-center text-secondary">
        Loading conversation...
      </div>
    );
  if (!conversation)
    return (
      <div className="app-shell flex min-h-screen flex-col items-center justify-center gap-4 text-danger">
        <p>{error || "Conversation unavailable"}</p>
        <Link to="/conversations" className="text-accent">
          Back to conversations
        </Link>
      </div>
    );

  const send = (
    event: "message" | "message.replied",
    content: string,
    messageId?: string,
    clientMessageId?: string,
    mentions?: MessageMention[],
  ) =>
    sendConversationEvent(event, conversationId, receiverType, receiverId, {
      content,
      message_id: messageId,
      client_message_id: clientMessageId,
      mentions,
    });
  const stopTyping = () => {
    if (typingTimer.current) clearTimeout(typingTimer.current);
    typingTimer.current = null;
    if (localTyping.current)
      sendConversationEvent(
        "typing",
        conversationId,
        receiverType,
        receiverId,
        { state: false },
      );
    localTyping.current = false;
  };
  const changeInput = (value: string) => {
    setInput(value);
    setActiveMention(0);
    if (!value) {
      stopTyping();
      return;
    }
    if (
      !localTyping.current &&
      sendConversationEvent(
        "typing",
        conversationId,
        receiverType,
        receiverId,
        { state: true },
      )
    )
      localTyping.current = true;
    if (typingTimer.current) clearTimeout(typingTimer.current);
    typingTimer.current = setTimeout(stopTyping, TYPING_IDLE_MS);
  };
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!input.trim()) return;
    const content = input.trim();
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    const reply = replyTo;
    const mentions = selectedMentions
      .flatMap((selected) => {
        const utf16Offset = content.indexOf(selected.label);
        return utf16Offset < 0
          ? []
          : [
              {
                kind: selected.kind,
                user_id: selected.userId,
                offset: Array.from(content.slice(0, utf16Offset)).length,
                length: Array.from(selected.label).length,
              } as MessageMention,
            ];
      })
      .sort((a, b) => a.offset - b.offset);
    setMessages((current) =>
      mergeMessagesChronologically(current, {
        id,
        client_message_id: id,
        sender_id: user?.id ?? "",
        receiver_id: receiverId,
        content,
        is_group: conversation.kind === "group",
        conversation_id: conversationId,
        reply_to_message_id: reply?.id,
        reply_to: reply
          ? { id: reply.id, sender_id: reply.sender_id, content: reply.content }
          : undefined,
        created_at: now,
        modified_at: now,
        reactions: [],
        mentions,
        status: "sending",
      }),
    );
    if (
      !send(
        reply ? "message.replied" : "message",
        content,
        reply?.id,
        id,
        mentions,
      )
    )
      setMessages((current) =>
        current.map((message) =>
          message.client_message_id === id
            ? { ...message, status: "failed" }
            : message,
        ),
      );
    setInput("");
    setReplyTo(null);
    setSelectedMentions([]);
    stopTyping();
  };
  const retry = (message: Message) => {
    if (!message.client_message_id) return;
    setMessages((current) =>
      current.map((item) =>
        item.client_message_id === message.client_message_id
          ? { ...item, status: "sending" }
          : item,
      ),
    );
    if (
      !send(
        message.reply_to_message_id ? "message.replied" : "message",
        message.content,
        message.reply_to_message_id,
        message.client_message_id,
        message.mentions,
      )
    )
      setMessages((current) =>
        current.map((item) =>
          item.client_message_id === message.client_message_id
            ? { ...item, status: "failed" }
            : item,
        ),
      );
  };
  const mutate = async (patch: {
    archived?: boolean;
    pinned?: boolean;
    mute_minutes?: number;
  }) => {
    const result = await updateConversationPreferences(conversationId, patch);
    if (result.data) { setConversation(result.data); refresh(); }
    else setError(result.error || "Could not update conversation");
  };
  const hideCurrent = async () => {
    if (!await confirm("Hide this conversation from your list?")) return;
    const result = await hideConversation(conversationId);
    if (result.success) { refresh(); navigate("/conversations"); }
    else setError(result.error || "Could not hide conversation");
  };
  const edit = async (message: Message) => {
    const content = (await askText("Edit message", message.content))?.trim();
    if (
      content &&
      sendConversationEvent(
        "message.edited",
        conversationId,
        receiverType,
        receiverId,
        { message_id: message.id, content, mentions: [] },
      )
    )
      setMessages((current) =>
        current.map((item) =>
          item.id === message.id
             ? { ...item, content, mentions: [], edited_at: new Date().toISOString() }
            : item,
        ),
      );
  };
  const remove = async (message: Message) => {
    if (
      await confirm("Delete this message?") &&
      sendConversationEvent(
        "message.deleted",
        conversationId,
        receiverType,
        receiverId,
        { message_id: message.id },
      )
    )
      setMessages((current) =>
        current.filter((item) => item.id !== message.id),
      );
  };
  const copy = async (message: Message) => {
    try {
      await navigator.clipboard.writeText(message.content);
    } catch {
      setError("Could not copy message");
    }
  };
  const openForward = async (message: Message) => {
    forwardTriggerRef.current = document.activeElement as HTMLElement | null;
    setForwarding(message);
    const result = await listConversations();
    setDestinations(
      (result.data?.conversations ?? []).filter(
        (item) => item.id !== conversationId,
      ),
    );
  };
  const closeForward = () => {
    setForwarding(null);
    requestAnimationFrame(() => forwardTriggerRef.current?.focus());
  };
  const forward = async (destination: string) => {
    if (!forwarding) return;
    const result = await forwardMessage(
      destination,
      forwarding.id,
      crypto.randomUUID(),
    );
    if (result.success) closeForward();
    else setError(result.error || "Could not forward message");
  };
  const react = async (message: Message, reaction: string) => {
    const previous = message.reactions ?? [];
    const add = !previous.find((item) => item.reaction === reaction)
      ?.reacted_by_me;
    const optimistic = optimisticReaction(previous, reaction, add);
    setMessages((current) =>
      current.map((item) =>
        item.id === message.id
          ? {
              ...item,
              reactions: optimistic,
            }
          : item,
      ),
    );
    try {
      const result = await setMessageReaction(message.id, reaction, add);
      if (!result.success || !result.data) throw new Error(result.error);
      setMessages((current) =>
        current.map((item) =>
          item.id === message.id
            ? { ...item, reactions: result.data!.reactions }
            : item,
        ),
      );
      sendConversationEvent(
        add ? "message.reaction.added" : "message.reaction.removed",
        conversationId,
        receiverType,
        receiverId,
        { message_id: message.id, reaction },
      );
    } catch {
      setMessages((current) =>
        current.map((item) =>
          item.id === message.id && item.reactions === optimistic ? { ...item, reactions: previous } : item,
        ),
      );
      setError("Could not update reaction");
    }
  };
  const activeTypers = Object.keys(typingUsers[conversationId] ?? {}).map(
    (id) => memberNames[id] ?? "Someone",
  );
  const mentionQuery = input.match(/(?:^|\s)@(\w*)$/)?.[1]?.toLowerCase();
  const mentionSuggestions: { kind: "user" | "everyone"; userId?: string; label: string }[] =
    mentionQuery === undefined
      ? []
      : [
          ...members
            .filter(
              (member) =>
                member.user_id !== user?.id &&
                member.username.toLowerCase().startsWith(mentionQuery),
            )
            .slice(0, 5)
            .map((member) => ({
              kind: "user" as const,
              userId: member.user_id,
              label: `@${member.username}`,
            })),
          ...(conversation.kind === "group" &&
          conversation.current_role !== "member" &&
          "everyone".startsWith(mentionQuery)
            ? [{ kind: "everyone" as const, label: "@everyone" }]
            : []),
        ];
  const chooseMention = (suggestion: {
    kind: "user" | "everyone";
    userId?: string;
    label: string;
  }) => {
    setInput((value) => value.replace(/@\w*$/, `${suggestion.label} `));
    setSelectedMentions((current) => [
      ...current.filter(
        (item) =>
          item.kind !== suggestion.kind || item.userId !== suggestion.userId,
      ),
      suggestion,
    ]);
  };

  return (
    <div className="chat-workspace">
      <div className="chat-column">
      <header className="app-header shrink-0">
        <div className="chat-topbar">
          <Link to="/conversations" className="chat-back-link" aria-label="Back to chats"><Icon name="back" /></Link>
          <div className="avatar h-10 w-10">{conversationInitials(title)}</div>
          <div className="chat-topbar-title"><p>{title}</p><span>{conversation.kind === "group" ? `${conversation.member_count} members` : presence[peerId]?.online ? "Online" : presence[peerId] ? "Offline" : ""}</span></div>
          <span className={`connection-pill${isConnected ? ' connected' : ''}`}><i />{isConnected ? "Connected" : "Reconnecting"}</span>
          <details className="chat-actions-menu"><summary className="icon-button" aria-label="Conversation actions"><Icon name="more" /></summary><div className="chat-header-actions">
            {conversation.kind === "group" && <Link to={`/conversations/${conversationId}/settings`} className="soft-button px-3 py-2 text-xs">Group details</Link>}
            <button onClick={() => void mutate({ pinned: !conversation.pinned })} className="soft-button px-3 py-2 text-xs">{conversation.pinned ? "Unpin" : "Pin"}</button>
            <button onClick={() => void mutate({ archived: !conversation.archived })} className="soft-button px-3 py-2 text-xs">{conversation.archived ? "Unarchive" : "Archive"}</button>
            <button onClick={() => void mutate({ mute_minutes: conversation.muted ? 0 : 1440 })} className="soft-button px-3 py-2 text-xs">{conversation.muted ? "Unmute" : "Mute"}</button>
            <button onClick={() => void hideCurrent()} className="soft-button px-3 py-2 text-xs text-danger">Hide</button>
          </div></details>
        </div>
      </header>
      {error && (
        <div className="mx-auto w-full max-w-5xl px-4 pt-4 text-sm text-danger sm:px-6">
          {error}
        </div>
      )}
      <main ref={historyRef} className="chat-pattern message-history scrollbar-thin flex-1 overflow-y-auto" onScroll={(event) => {
        const history = event.currentTarget;
        const nearBottom = history.scrollHeight - history.scrollTop - history.clientHeight < 100;
        followLatest.current = nearBottom;
        setShowJump(!nearBottom);
      }}>
        <div className="message-stack">
          {hasMore && (
            <button
              onClick={() => void load(true)}
              disabled={loadingOlder}
              className="self-center rounded-full bg-raised px-4 py-2 text-sm text-accent"
            >
              {loadingOlder ? "Loading..." : "Load older messages"}
            </button>
          )}
          {messages.length === 0 ? (
            <div className="py-16 text-center text-secondary">
              <p className="font-medium text-primary">No messages yet</p>
              <p className="mt-1 text-sm">Start the conversation below.</p>
            </div>
          ) : (
            messages.map((message, index) => (
              <Fragment key={message.id}>
              {(index === 0 || new Date(messages[index - 1].created_at).toDateString() !== new Date(message.created_at).toDateString()) && <div className="date-separator">{new Date(message.created_at).toDateString() === new Date().toDateString() ? 'Today' : new Date(message.created_at).toLocaleDateString([], { day: 'numeric', month: 'long', year: 'numeric' })}</div>}
              <div
                className={`message-row${index > 0 && messages[index - 1].sender_id === message.sender_id && new Date(messages[index - 1].created_at).toDateString() === new Date(message.created_at).toDateString() ? ' continuation' : ''} ${message.sender_id === user?.id ? "justify-end" : "justify-start"}`}
              >
                <div
                  className={`${message.sender_id === user?.id ? "message-out" : "message-in"}`}
                >
                  {conversation.kind === "group" &&
                    message.sender_id !== user?.id && (
                      <p className="mb-1 text-xs font-semibold text-accent">
                        {memberNames[message.sender_id] ?? "Member"}
                      </p>
                    )}
                  {message.forwarded_from_message_id && (
                    <p className="mb-1 text-xs italic opacity-70">
                      Forwarded
                      {message.forwarded_from
                        ? ` from ${memberNames[message.forwarded_from.sender_id] ?? "a member"}`
                        : ""}
                    </p>
                  )}
                  <div className="message-body">
                    {message.reply_to_message_id && (
                      <div className="mb-2 border-l-2 border-accent pl-2 text-xs opacity-75">
                        <span className="font-semibold">
                          {message.reply_to
                            ? (memberNames[message.reply_to.sender_id] ??
                              (message.reply_to.sender_id === user?.id
                                ? "You"
                                : "Member"))
                            : "Original message"}
                        </span>
                        <span className="block truncate">
                          {message.reply_to?.content ?? "Message unavailable"}
                        </span>
                      </div>
                    )}
                    <MentionText content={message.content} mentions={message.mentions} />
                  </div>
                  {(message.reactions?.length ?? 0) > 0 && (
                    <div className="mt-2 flex flex-wrap gap-1">
                      {message.reactions?.map((item) => (
                        <button
                          key={item.reaction}
                          aria-label={`${item.reaction}, ${item.count} reactions`}
                          aria-pressed={item.reacted_by_me}
                          onClick={() => void react(message, item.reaction)}
                          className={`rounded-full px-2 py-1 text-xs ${item.reacted_by_me ? "bg-accent-soft" : "bg-raised"}`}
                        >
                          {item.reaction} {item.count}
                        </button>
                      ))}
                    </div>
                  )}
                  <div className="message-meta">
                    {new Date(message.created_at).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}{" "}
                    {message.sender_id === user?.id && <MessageReceipt status={message.status} />}
                    {message.edited_at && <span className="message-edited">edited</span>}
                    {message.status === "failed" && (
                      <button
                        onClick={() => retry(message)}
                        className="ml-2 underline"
                      >
                        Retry
                      </button>
                    )}
                  </div>
                  <details className="message-actions text-xs">
                    <summary
                      className="icon-button"
                      aria-label="Message actions"
                    >
                      <Icon name="more" width={18} height={18} />
                    </summary>
                    <div className="absolute right-0 z-10 mt-1 min-w-48 rounded-xl border border-theme bg-raised p-2 text-left shadow-xl">
                      <div className="mb-2 flex gap-1 border-b border-theme pb-2">
                        {QUICK_REACTIONS.map((emoji) => (
                          <button
                            key={emoji}
                            onClick={() => void react(message, emoji)}
                            className="rounded p-1 text-base hover-surface"
                            aria-label={`React ${emoji}`}
                          >
                            {emoji}
                          </button>
                        ))}
                      </div>
                      <button
                        onClick={() => {
                          setReplyTo(message);
                          document
                            .querySelector<HTMLTextAreaElement>("#message-input")
                            ?.focus();
                        }}
                        className="block w-full rounded px-3 py-2 text-left hover-surface"
                      >
                        Reply
                      </button>
                      <button
                        onClick={() => void copy(message)}
                        className="block w-full rounded px-3 py-2 text-left hover-surface"
                      >
                        Copy
                      </button>
                      <button
                        onClick={() => void openForward(message)}
                        className="block w-full rounded px-3 py-2 text-left hover-surface"
                      >
                        Forward
                      </button>
                      {message.sender_id === user?.id && (
                        <>
                          <button
                            onClick={() => edit(message)}
                            className="block w-full rounded px-3 py-2 text-left hover-surface"
                          >
                            Edit
                          </button>
                          <button
                            onClick={() => remove(message)}
                            className="block w-full rounded px-3 py-2 text-left text-danger hover-surface"
                          >
                            Delete
                          </button>
                        </>
                      )}
                    </div>
                  </details>
                </div>
              </div>
              </Fragment>
            ))
          )}
          {activeTypers.length > 0 && (
            <div className="text-sm italic text-secondary">
              {activeTypers.slice(0, 2).join(", ")}{" "}
              {activeTypers.length === 1 ? "is" : "are"} typing...
            </div>
          )}
          <div ref={endRef} />
        </div>
      </main>
      {forwarding && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Forward message"
          className="fixed inset-0 z-30 flex items-end justify-center bg-black/60 p-4 sm:items-center"
        >
          <div ref={forwardDialogRef} className="w-full max-w-md rounded-2xl border border-theme bg-raised p-4">
            <div className="flex items-center justify-between">
              <h2 className="font-semibold">Forward to</h2>
              <button
                onClick={closeForward}
                aria-label="Close forward picker"
                className="p-2"
              >
                ✕
              </button>
            </div>
            <div className="mt-3 max-h-72 space-y-2 overflow-y-auto">
              {destinations.map((item) => (
                <button
                  key={item.id}
                  onClick={() => void forward(item.id)}
                  className="soft-button block w-full px-4 py-3 text-left"
                >
                  {item.kind === "group" ? item.name : "Direct conversation"}
                </button>
              ))}
              {destinations.length === 0 && (
                <p className="py-6 text-center text-sm text-secondary">
                  No other conversations available.
                </p>
              )}
            </div>
          </div>
        </div>
      )}
      {showJump && <button className="jump-to-latest" onClick={() => { followLatest.current = true; if (historyRef.current) historyRef.current.scrollTop = historyRef.current.scrollHeight; setShowJump(false); }}>Latest messages ↓</button>}
      <div className="chat-composer relative">
        {mentionSuggestions.length > 0 && (
          <div id="mention-suggestions" role="listbox" aria-label="Mention suggestions" className="absolute bottom-full left-1/2 w-full max-w-3xl -translate-x-1/2 rounded-t-xl border border-theme bg-raised p-2 shadow-xl">
            {mentionSuggestions.map((suggestion, index) => (
              <button
                type="button"
                key={`${suggestion.kind}:${suggestion.userId ?? ""}`}
                id={`mention-option-${index}`}
                role="option"
                aria-selected={index === activeMention}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => chooseMention(suggestion)}
                className={`block w-full rounded-lg px-3 py-2 text-left text-sm hover-surface ${index === activeMention ? "bg-accent-soft" : ""}`}
              >
                {suggestion.label}
              </button>
            ))}
          </div>
        )}
        {replyTo && (
          <div className="composer-reply">
            <span className="min-w-0">
              <strong className="block">
                Replying to{" "}
                {replyTo.sender_id === user?.id
                  ? "yourself"
                  : (memberNames[replyTo.sender_id] ?? "a member")}
              </strong>
              <span className="block truncate">
                {replyTo.content || "Message unavailable"}
              </span>
            </span>
            <button onClick={() => setReplyTo(null)} className="underline">
              Cancel
            </button>
          </div>
        )}
        <MessageComposer value={input} disabled={!isConnected} onChange={changeInput} onBlur={stopTyping} mentionCount={mentionSuggestions.length} activeMention={activeMention} onSubmit={(event) => { followLatest.current = true; setShowJump(false); submit(event); }} onKeyDown={(event) => {
          if (event.nativeEvent.isComposing || !mentionSuggestions.length) return;
          if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); setActiveMention((current) => (current + (event.key === "ArrowDown" ? 1 : -1) + mentionSuggestions.length) % mentionSuggestions.length); }
          else if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); chooseMention(mentionSuggestions[activeMention]); }
          else if (event.key === "Escape") { event.preventDefault(); setInput((value) => value.replace(/@\w*$/, "")); }
        }} />
      </div>
      </div>
    </div>
  );
}
