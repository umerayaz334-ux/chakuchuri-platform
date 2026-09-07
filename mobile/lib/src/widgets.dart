import 'dart:typed_data';

import 'package:flutter/material.dart';

import 'api_client.dart';
import 'models.dart';
import 'theme.dart';

String money(int value) =>
    'Rs ${value.toString().replaceAllMapped(RegExp(r'(\d)(?=(\d{3})+$)'), (match) => '${match[1]},')}';

String shortDate(String value) {
  if (value.isEmpty) return 'Not set';
  final parsed = DateTime.tryParse(value);
  if (parsed == null) return value;
  const months = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ];
  return '${months[parsed.month - 1]} ${parsed.day}, ${parsed.year}';
}

String messageStamp(String value) {
  if (value.isEmpty) return '';
  final parsed = DateTime.tryParse(value)?.toLocal();
  if (parsed == null) return value;
  final hour = parsed.hour > 12
      ? parsed.hour - 12
      : (parsed.hour == 0 ? 12 : parsed.hour);
  return '$hour:${parsed.minute.toString().padLeft(2, '0')} ${parsed.hour >= 12 ? 'PM' : 'AM'}';
}

String messageDayKey(String value) {
  final parsed = DateTime.tryParse(value)?.toLocal();
  if (parsed == null) return '';
  return '${parsed.year}-${parsed.month}-${parsed.day}';
}

enum MessageReceipt { none, delivered, read }

MessageReceipt ownMessageReceipt({
  required List<Message> chronological,
  required Message message,
  required bool mine,
  required int peerUnread,
}) {
  if (!mine) return MessageReceipt.none;
  final own = chronological
      .where((item) => item.authorRole.toLowerCase() == 'customer')
      .toList();
  final index = own.indexWhere((item) => item.id == message.id);
  if (index < 0) return MessageReceipt.delivered;
  return own.length - index <= peerUnread
      ? MessageReceipt.delivered
      : MessageReceipt.read;
}

class MessageTicks extends StatelessWidget {
  const MessageTicks({super.key, required this.receipt});

  final MessageReceipt receipt;

  @override
  Widget build(BuildContext context) {
    if (receipt == MessageReceipt.none) return const SizedBox.shrink();
    final read = receipt == MessageReceipt.read;
    return Padding(
      padding: const EdgeInsets.only(left: 3),
      child: Icon(
        Icons.done_all_rounded,
        size: 15,
        color: read ? const Color(0xFF34B7F1) : const Color(0xFF7D9189),
      ),
    );
  }
}

String messageDayLabel(String value) {
  final parsed = DateTime.tryParse(value)?.toLocal();
  if (parsed == null) return value.isEmpty ? 'Unknown date' : value;
  final now = DateTime.now();
  final today = DateTime(now.year, now.month, now.day);
  final day = DateTime(parsed.year, parsed.month, parsed.day);
  final dayDiff = today.difference(day).inDays;
  if (dayDiff == 0) return 'Today';
  if (dayDiff == 1) return 'Yesterday';
  const months = [
    'January',
    'February',
    'March',
    'April',
    'May',
    'June',
    'July',
    'August',
    'September',
    'October',
    'November',
    'December',
  ];
  return '${parsed.day} ${months[parsed.month - 1]} ${parsed.year}';
}

bool isPresenceOnline(String lastOnline, {bool? onlineFlag, DateTime? now}) {
  if (onlineFlag == false) return false;
  final parsed = DateTime.tryParse(lastOnline);
  if (parsed == null) return false;
  final age = (now ?? DateTime.now()).toUtc().difference(parsed.toUtc());
  if (age.inMilliseconds < -5000) return false;
  final normalized = age.isNegative ? Duration.zero : age;
  return normalized <= const Duration(seconds: 90);
}

String lastSeenLabel(String lastOnline, {bool? onlineFlag, DateTime? now}) {
  final clock = now ?? DateTime.now();
  if (isPresenceOnline(lastOnline, onlineFlag: onlineFlag, now: clock)) {
    return 'online';
  }
  final parsed = DateTime.tryParse(lastOnline)?.toLocal();
  if (parsed == null) {
    if (lastOnline.isEmpty || lastOnline == 'Never' || lastOnline == 'Not tracked yet') {
      return 'last seen recently';
    }
    return 'last seen $lastOnline';
  }
  final diff = clock.difference(parsed);
  final seconds = diff.inSeconds;
  if (seconds < 8) return 'last seen just now';
  if (seconds < 60) return 'last seen a few seconds ago';
  if (diff.inMinutes < 60) {
    final mins = diff.inMinutes.clamp(1, 59);
    return 'last seen $mins minute${mins == 1 ? '' : 's'} ago';
  }
  if (diff.inHours < 24) {
    final hours = diff.inHours.clamp(1, 23);
    return 'last seen $hours hour${hours == 1 ? '' : 's'} ago';
  }
  final time =
      '${parsed.hour > 12
          ? parsed.hour - 12
          : (parsed.hour == 0 ? 12 : parsed.hour)}:${parsed.minute.toString().padLeft(2, '0')} ${parsed.hour >= 12 ? 'PM' : 'AM'}';
  final today = DateTime(clock.year, clock.month, clock.day);
  final day = DateTime(parsed.year, parsed.month, parsed.day);
  if (day == today) return 'last seen today at $time';
  if (day == today.subtract(const Duration(days: 1))) {
    return 'last seen yesterday at $time';
  }
  return 'last seen ${shortDate(lastOnline)} at $time';
}

String displayStageLabel(String value) {
  final stage = value.trim();
  if (stage.toLowerCase() == 'production ready') {
    return 'Starting Production';
  }
  return stage;
}

String readinessText(String value) {
  if (value.isEmpty) return 'Date pending';
  final date = DateTime.tryParse(value)?.toLocal();
  if (date == null) return value;
  final now = DateTime.now();
  final today = DateTime(now.year, now.month, now.day);
  final target = DateTime(date.year, date.month, date.day);
  final days = target.difference(today).inDays;
  if (days == 0) return 'Due today';
  if (days == 1) return '1 day left';
  if (days > 1) return '$days days left';
  if (days == -1) return '1 day overdue';
  return '${days.abs()} days overdue';
}

class AppLogoMark extends StatelessWidget {
  const AppLogoMark({super.key, this.size = 38});

  final double size;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: AppColors.charcoal,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Stack(
        alignment: Alignment.center,
        children: [
          Text(
            'CC',
            style: TextStyle(
              color: Colors.white,
              fontSize: size * 0.34,
              fontWeight: FontWeight.w900,
              letterSpacing: 0,
            ),
          ),
          Positioned(
            left: size * 0.2,
            right: size * 0.2,
            bottom: size * 0.17,
            child: Container(height: 2, color: const Color(0xFF53C59F)),
          ),
        ],
      ),
    );
  }
}

class PageHeader extends StatelessWidget {
  const PageHeader({
    super.key,
    required this.title,
    required this.subtitle,
    this.action,
    this.eyebrow = '',
  });

  final String title;
  final String subtitle;
  final String eyebrow;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (eyebrow.isNotEmpty) ...[
                Text(
                  eyebrow.toUpperCase(),
                  style: const TextStyle(
                    color: AppColors.primary,
                    fontSize: 11,
                    fontWeight: FontWeight.w900,
                    letterSpacing: 0,
                  ),
                ),
                const SizedBox(height: 5),
              ],
              Text(title, style: Theme.of(context).textTheme.headlineSmall),
              if (subtitle.trim().isNotEmpty) ...[
                const SizedBox(height: 5),
                Text(
                  subtitle,
                  style: Theme.of(context).textTheme.bodyMedium
                      ?.copyWith(color: AppColors.muted, height: 1.35),
                ),
              ],
            ],
          ),
        ),
        if (action != null) ...[const SizedBox(width: 12), action!],
      ],
    );
  }
}

class SectionCard extends StatelessWidget {
  const SectionCard({
    super.key,
    required this.title,
    this.action,
    required this.child,
    this.subtitle = '',
    this.padding = const EdgeInsets.all(16),
  });

  final String title;
  final String subtitle;
  final Widget? action;
  final Widget child;
  final EdgeInsetsGeometry padding;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: padding,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        title,
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      if (subtitle.isNotEmpty) ...[
                        const SizedBox(height: 3),
                        Text(
                          subtitle,
                          style: const TextStyle(
                            color: AppColors.muted,
                            fontSize: 12,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                ?action,
              ],
            ),
            const SizedBox(height: 14),
            child,
          ],
        ),
      ),
    );
  }
}

class SectionLabel extends StatelessWidget {
  const SectionLabel({
    super.key,
    required this.title,
    this.detail = '',
    this.action,
  });

  final String title;
  final String detail;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(title, style: Theme.of(context).textTheme.titleMedium),
              if (detail.isNotEmpty)
                Text(
                  detail,
                  style: const TextStyle(color: AppColors.muted, fontSize: 12),
                ),
            ],
          ),
        ),
        ?action,
      ],
    );
  }
}

class MetricTile extends StatelessWidget {
  const MetricTile({
    super.key,
    required this.label,
    required this.value,
    required this.detail,
    this.icon = Icons.insights_outlined,
    this.accent = AppColors.primary,
  });

  final String label;
  final String value;
  final String detail;
  final IconData icon;
  final Color accent;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(13),
      decoration: BoxDecoration(
        color: Colors.white,
        border: Border.all(color: const Color(0xFFDDE7E2)),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 30,
                height: 30,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: accent.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(7),
                ),
                child: Icon(icon, size: 17, color: accent),
              ),
              const Spacer(),
              Text(
                label.toUpperCase(),
                style: const TextStyle(
                  color: AppColors.muted,
                  fontSize: 10,
                  fontWeight: FontWeight.w900,
                  letterSpacing: 0,
                ),
              ),
            ],
          ),
          const Spacer(),
          FittedBox(
            fit: BoxFit.scaleDown,
            alignment: Alignment.centerLeft,
            child: Text(
              value,
              maxLines: 1,
              style: const TextStyle(
                color: AppColors.ink,
                fontSize: 20,
                fontWeight: FontWeight.w900,
                letterSpacing: 0,
              ),
            ),
          ),
          const SizedBox(height: 2),
          Text(
            detail,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(color: AppColors.muted, fontSize: 11),
          ),
        ],
      ),
    );
  }
}

class QuickAction extends StatelessWidget {
  const QuickAction({
    super.key,
    required this.icon,
    required this.label,
    required this.onTap,
    this.color = AppColors.primary,
  });

  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 46,
                height: 46,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(icon, color: color, size: 22),
              ),
              const SizedBox(height: 7),
              Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class StatusPill extends StatelessWidget {
  const StatusPill(this.value, {super.key});

  final String value;

  @override
  Widget build(BuildContext context) {
    final normalized = value.toLowerCase();
    final color =
        normalized.contains('reject') ||
            normalized.contains('cancel') ||
            normalized.contains('overdue')
        ? AppColors.red
        : normalized.contains('waiting') ||
              normalized.contains('pending') ||
              normalized.contains('requested') ||
              normalized.contains('priced')
        ? AppColors.amber
        : normalized.contains('confirm') ||
              normalized.contains('ready') ||
              normalized.contains('complete') ||
              normalized.contains('deliver') ||
              normalized.contains('paid')
        ? AppColors.primary
        : AppColors.blue;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.09),
        border: Border.all(color: color.withValues(alpha: 0.2)),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        value.isEmpty ? 'Pending' : value,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(
          color: color,
          fontSize: 10,
          fontWeight: FontWeight.w900,
          letterSpacing: 0,
        ),
      ),
    );
  }
}

class EmptyState extends StatelessWidget {
  const EmptyState(
    this.title, {
    super.key,
    this.detail = '',
    this.icon = Icons.inbox_outlined,
  });

  final String title;
  final String detail;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 20),
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.line),
        borderRadius: BorderRadius.circular(8),
        color: AppColors.surfaceSoft.withValues(alpha: 0.45),
      ),
      child: Column(
        children: [
          Icon(icon, color: AppColors.muted, size: 24),
          const SizedBox(height: 8),
          Text(
            title,
            textAlign: TextAlign.center,
            style: const TextStyle(fontWeight: FontWeight.w800),
          ),
          if (detail.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(
              detail,
              textAlign: TextAlign.center,
              style: const TextStyle(color: AppColors.muted, fontSize: 12),
            ),
          ],
        ],
      ),
    );
  }
}

class MessengerBubble extends StatelessWidget {
  const MessengerBubble({
    super.key,
    required this.message,
    required this.mine,
    this.api,
    this.onOpenAttachment,
    this.receipt = MessageReceipt.none,
  });

  final Message message;
  final bool mine;
  final ApiClient? api;
  final Future<void> Function(MessageAttachment attachment)? onOpenAttachment;
  final MessageReceipt receipt;

  @override
  Widget build(BuildContext context) {
    final hasBody = message.body.trim().isNotEmpty;
    final attachments = message.attachments;
    return Align(
      alignment: mine ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        constraints: const BoxConstraints(maxWidth: 304),
        padding: const EdgeInsets.fromLTRB(13, 10, 13, 7),
        decoration: BoxDecoration(
          color: mine ? const Color(0xFFE7F4EF) : Colors.white,
          border: Border.all(
            color: mine ? const Color(0xFFCFE6DC) : AppColors.line,
          ),
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(16),
            topRight: const Radius.circular(16),
            bottomLeft: Radius.circular(mine ? 16 : 5),
            bottomRight: Radius.circular(mine ? 5 : 16),
          ),
        ),
        child: Column(
          crossAxisAlignment: mine
              ? CrossAxisAlignment.end
              : CrossAxisAlignment.start,
          children: [
            if (hasBody)
              Text(
                message.body,
                style: const TextStyle(
                  color: AppColors.ink,
                  height: 1.4,
                  fontSize: 14.5,
                ),
              ),
            if (!hasBody && attachments.isEmpty)
              const Text(
                'Empty message',
                style: TextStyle(
                  color: AppColors.muted,
                  fontStyle: FontStyle.italic,
                ),
              ),
            if (attachments.isNotEmpty) ...[
              if (hasBody) const SizedBox(height: 8),
              ...attachments.map(
                (attachment) => Padding(
                  padding: const EdgeInsets.only(bottom: 6),
                  child: _MessageAttachmentChip(
                    attachment: attachment,
                    mine: mine,
                    api: api,
                    onOpen: onOpenAttachment == null
                        ? null
                        : () => onOpenAttachment!(attachment),
                  ),
                ),
              ),
            ],
            const SizedBox(height: 5),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  messageStamp(message.createdAt),
                  style: TextStyle(
                    color: mine ? const Color(0xFF4F6A5F) : AppColors.muted,
                    fontSize: 10,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                if (mine) MessageTicks(receipt: receipt),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _MessageAttachmentChip extends StatelessWidget {
  const _MessageAttachmentChip({
    required this.attachment,
    required this.mine,
    this.api,
    this.onOpen,
  });

  final MessageAttachment attachment;
  final bool mine;
  final ApiClient? api;
  final VoidCallback? onOpen;

  @override
  Widget build(BuildContext context) {
    final fg = AppColors.ink;
    final soft = mine ? const Color(0xFFD7EBE3) : AppColors.surfaceSoft;
    if (attachment.isImage && api != null) {
      return Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onOpen,
          borderRadius: BorderRadius.circular(10),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: SizedBox(
              width: 168,
              height: 120,
              child: FutureBuilder<Uint8List>(
                future: api!.fetchFileBytes(attachment.fileId),
                builder: (context, snapshot) {
                  if (snapshot.hasData) {
                    return Image.memory(
                      Uint8List.fromList(snapshot.data!),
                      fit: BoxFit.cover,
                      gaplessPlayback: true,
                    );
                  }
                  return ColoredBox(
                    color: soft,
                    child: Icon(
                      snapshot.hasError
                          ? Icons.broken_image_outlined
                          : Icons.image_outlined,
                      color: fg,
                    ),
                  );
                },
              ),
            ),
          ),
        ),
      );
    }

    return Material(
      color: soft,
      borderRadius: BorderRadius.circular(10),
      child: InkWell(
        onTap: onOpen,
        borderRadius: BorderRadius.circular(10),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                attachment.mimeType.contains('pdf')
                    ? Icons.picture_as_pdf_outlined
                    : Icons.attach_file_rounded,
                color: fg,
                size: 18,
              ),
              const SizedBox(width: 8),
              Flexible(
                child: Text(
                  attachment.name.isEmpty ? 'Attachment' : attachment.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: fg,
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class ProgressLine extends StatelessWidget {
  const ProgressLine({super.key, required this.value});

  final int value;

  @override
  Widget build(BuildContext context) {
    final progress = value.clamp(0, 100);
    final color = progress >= 80
        ? AppColors.primary
        : progress >= 40
        ? AppColors.blue
        : AppColors.amber;
    return ClipRRect(
      borderRadius: BorderRadius.circular(4),
      child: LinearProgressIndicator(
        minHeight: 7,
        value: progress / 100,
        backgroundColor: AppColors.surfaceSoft,
        color: color,
      ),
    );
  }
}
