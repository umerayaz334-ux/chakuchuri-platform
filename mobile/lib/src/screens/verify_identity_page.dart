import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';

import '../api_client.dart';
import '../customer_kyc.dart';
import '../theme.dart';
import '../widgets.dart';

typedef KycActionRunner = Future<void> Function(
  Future<void> Function() action,
  String success,
);

class VerifyIdentityPage extends StatefulWidget {
  const VerifyIdentityPage({
    super.key,
    required this.api,
    required this.customer,
    required this.onAction,
    required this.onUpdated,
    required this.onDone,
  });

  final ApiClient api;
  final Customer customer;
  final KycActionRunner onAction;
  final ValueChanged<Customer> onUpdated;
  final VoidCallback onDone;

  @override
  State<VerifyIdentityPage> createState() => _VerifyIdentityPageState();
}

class _VerifyIdentityPageState extends State<VerifyIdentityPage> {
  String _busy = '';
  String _error = '';

  Customer get customer => widget.customer;

  Map<String, CustomerDocument> get _byKind {
    final map = <String, CustomerDocument>{};
    for (final doc in customer.documents) {
      if (doc.kind != 'other') map[doc.kind] = doc;
    }
    return map;
  }

  Map<String, CustomerDocument> get _askDocs {
    final map = <String, CustomerDocument>{};
    for (final doc in customer.documents) {
      if (doc.requestId.isNotEmpty) map[doc.requestId] = doc;
    }
    return map;
  }

  List<RequestedDocument> get _activeAsks => customer.requestedDocuments
      .where(isActiveDocumentRequest)
      .toList(growable: false);

  List<RequestedDocument> get _actionAsks => _activeAsks
      .where((ask) => slotNeedsAction(_askDocs[ask.id]))
      .toList(growable: false);

  List<({String kind, String label})> get _customerSlots =>
      kycSlots.where((slot) => slotNeedsAction(_byKind[slot.kind])).toList();

  int get _rejectedCount => customer.documents
      .where((doc) => doc.normalizedStatus == 'rejected')
      .length;

  int get _remainingCount {
    final askRemaining = _actionAsks.length;
    return _customerSlots.length + askRemaining;
  }

  bool get _requiredReady => kycSlots.every((slot) {
    final doc = _byKind[slot.kind];
    if (doc == null) return false;
    final status = doc.normalizedStatus;
    if (status == 'accepted' || status == 'submitted' || status == 'uploaded') {
      return true;
    }
    if (status == 'rejected') return false;
    return status == 'draft' && doc.fileId.isNotEmpty;
  });

  bool get _asksReady => _activeAsks.every((ask) {
    final doc = _askDocs[ask.id];
    final status = doc?.normalizedStatus ?? '';
    if (status == 'accepted' || status == 'submitted' || status == 'uploaded') {
      return true;
    }
    return doc != null && doc.fileId.isNotEmpty && status == 'draft';
  });

  bool get _canSubmit =>
      _requiredReady &&
      _asksReady &&
      customer.documents.any((doc) => doc.normalizedStatus == 'draft');

  Future<void> _pickAndUpload({
    required String kind,
    String label = '',
    String requestId = '',
  }) async {
    try {
      final selected = await ImagePicker().pickImage(
        source: ImageSource.gallery,
        imageQuality: 85,
        maxWidth: 2200,
        maxHeight: 2200,
        requestFullMetadata: false,
      );
      if (selected == null) return;
      final bytes = await selected.readAsBytes();
      if (bytes.isEmpty || bytes.length > 10 * 1024 * 1024) {
        setState(() => _error = 'Choose an image smaller than 10 MB.');
        return;
      }
      setState(() {
        _busy = requestId.isNotEmpty ? requestId : kind;
        _error = '';
      });
      await widget.onAction(
        () async {
          final uploaded = await widget.api.uploadFile(
            bytes: bytes,
            filename: selected.name,
            ownerType: 'customer_document',
            ownerId: customer.id,
            customerId: customer.id,
          );
          final next = await widget.api.attachCustomerDocument(
            customerId: customer.id,
            fileId: uploaded.id,
            kind: kind,
            label: label,
            originalName: uploaded.originalName.isNotEmpty
                ? uploaded.originalName
                : selected.name,
            requestId: requestId,
          );
          widget.onUpdated(next);
        },
        '${label.isEmpty ? kind : label} added. Replace anytime before Submit.',
      );
      if (!mounted) return;
      setState(() => _busy = '');
    } on ApiException catch (error) {
      if (!mounted) return;
      setState(() {
        _busy = '';
        _error = error.message;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _busy = '';
        _error = 'Document could not be uploaded.';
      });
    }
  }

  Future<void> _remove(CustomerDocument doc) async {
    setState(() {
      _busy = doc.id;
      _error = '';
    });
    try {
      await widget.onAction(() async {
        final next = await widget.api.deleteCustomerDocument(
          customerId: customer.id,
          documentId: doc.id,
        );
        widget.onUpdated(next);
      }, 'Document removed.');
      if (!mounted) return;
      setState(() => _busy = '');
    } on ApiException catch (error) {
      if (!mounted) return;
      setState(() {
        _busy = '';
        _error = error.message;
      });
    }
  }

  Future<void> _submit() async {
    setState(() {
      _busy = 'submit';
      _error = '';
    });
    try {
      await widget.onAction(
        () async {
          final next = await widget.api.submitCustomerDocuments(customer.id);
          widget.onUpdated(next);
        },
        'We are verifying your documents. If anything further is needed, we will get back to you.',
      );
      if (!mounted) return;
      setState(() => _busy = '');
      widget.onDone();
    } on ApiException catch (error) {
      if (!mounted) return;
      setState(() {
        _busy = '';
        _error = error.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (!customerNeedsIdentityAction(customer)) {
      return ListView(
        padding: const EdgeInsets.fromLTRB(14, 16, 14, 24),
        children: [
          PageHeader(
            eyebrow: 'Identity',
            title: isCustomerVerified(customer.verificationStatus)
                ? 'Account verified'
                : 'In review',
            subtitle: isCustomerVerified(customer.verificationStatus)
                ? 'Your account is verified.'
                : 'We are verifying your documents. If anything further is needed, we will get back to you.',
          ),
        ],
      );
    }

    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 16, 14, 28),
      children: [
        PageHeader(
          eyebrow: 'Identity check',
          title: _rejectedCount > 0 ? 'Replace a document' : 'Documents needed',
          subtitle: _rejectedCount > 0
              ? 'Add a new copy of the item below.'
              : 'Upload the items shown below.',
          action: Text(
            _remainingCount > 0 ? '$_remainingCount remaining' : 'Ready',
            style: const TextStyle(
              color: AppColors.primary,
              fontWeight: FontWeight.w800,
              fontSize: 12,
            ),
          ),
        ),
        if (_error.isNotEmpty) ...[
          const SizedBox(height: 10),
          Text(
            _error,
            style: const TextStyle(
              color: Color(0xFFB42318),
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
        const SizedBox(height: 14),
        for (final slot in _customerSlots) ...[
          _DocSlotCard(
            api: widget.api,
            label: slot.label,
            doc: _byKind[slot.kind],
            busy: _busy == slot.kind || _busy == (_byKind[slot.kind]?.id ?? ''),
            onUpload: isEditableDoc(_byKind[slot.kind])
                ? () => _pickAndUpload(kind: slot.kind)
                : null,
            onRemove: () {
              final doc = _byKind[slot.kind];
              if (doc != null && doc.normalizedStatus == 'draft') {
                return () => _remove(doc);
              }
              return null;
            }(),
          ),
          const SizedBox(height: 10),
        ],
        if (_actionAsks.isNotEmpty) ...[
          const SizedBox(height: 6),
          const Text(
            'Requested by ChakuChuri',
            style: TextStyle(fontWeight: FontWeight.w900, fontSize: 15),
          ),
          const SizedBox(height: 4),
          const Text(
            'Additional document',
            style: TextStyle(color: AppColors.muted, fontSize: 12),
          ),
          const SizedBox(height: 10),
          for (final ask in _actionAsks) ...[
            _DocSlotCard(
              api: widget.api,
              label: ask.label,
              doc: _askDocs[ask.id],
              busy: _busy == ask.id || _busy == (_askDocs[ask.id]?.id ?? ''),
              onUpload: isEditableDoc(_askDocs[ask.id])
                  ? () => _pickAndUpload(
                      kind: 'other',
                      label: ask.label,
                      requestId: ask.id,
                    )
                  : null,
              onRemove: () {
                final doc = _askDocs[ask.id];
                if (doc != null && doc.normalizedStatus == 'draft') {
                  return () => _remove(doc);
                }
                return null;
              }(),
            ),
            const SizedBox(height: 10),
          ],
        ],
        const SizedBox(height: 8),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(
            color: const Color(0xFFF8FAF9),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: const Color(0xFFD7E5DD)),
          ),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  _canSubmit
                      ? 'Ready for review'
                      : 'Add the required file to continue',
                  style: const TextStyle(
                    color: AppColors.muted,
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const SizedBox(width: 10),
              FilledButton(
                onPressed: _busy.isNotEmpty || !_canSubmit ? null : _submit,
                child: Text(_busy == 'submit' ? 'Submitting...' : 'Submit'),
              ),
            ],
          ),
        ),
        if (customer.identityNote.isNotEmpty && _rejectedCount > 0) ...[
          const SizedBox(height: 12),
          Text(
            customer.identityNote,
            style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 13),
          ),
        ],
      ],
    );
  }
}

class _DocSlotCard extends StatefulWidget {
  const _DocSlotCard({
    required this.api,
    required this.label,
    required this.doc,
    required this.busy,
    this.onUpload,
    this.onRemove,
  });

  final ApiClient api;
  final String label;
  final CustomerDocument? doc;
  final bool busy;
  final VoidCallback? onUpload;
  final VoidCallback? onRemove;

  @override
  State<_DocSlotCard> createState() => _DocSlotCardState();
}

class _DocSlotCardState extends State<_DocSlotCard> {
  Uint8List? _thumb;

  @override
  void initState() {
    super.initState();
    _loadThumb();
  }

  @override
  void didUpdateWidget(covariant _DocSlotCard oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.doc?.fileId != widget.doc?.fileId ||
        oldWidget.doc?.status != widget.doc?.status) {
      _loadThumb();
    }
  }

  Future<void> _loadThumb() async {
    final doc = widget.doc;
    if (doc == null || doc.fileId.isEmpty || doc.normalizedStatus != 'draft') {
      if (mounted) setState(() => _thumb = null);
      return;
    }
    try {
      final bytes = await widget.api.fetchFileBytes(doc.fileId);
      if (!mounted) return;
      setState(() => _thumb = bytes);
    } catch (_) {
      if (mounted) setState(() => _thumb = null);
    }
  }

  String get _statusText {
    final doc = widget.doc;
    if (doc == null) return 'Required';
    switch (doc.normalizedStatus) {
      case 'rejected':
        return 'Needs replacement';
      case 'draft':
        return 'Attached - not submitted';
      case 'accepted':
        return 'Accepted';
      default:
        return 'Submitted - in review';
    }
  }

  @override
  Widget build(BuildContext context) {
    final doc = widget.doc;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(
          color: doc?.normalizedStatus == 'rejected'
              ? const Color(0xFFFECACA)
              : AppColors.line,
        ),
      ),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: Container(
              width: 64,
              height: 64,
              color: const Color(0xFFF0F4F2),
              child: _thumb != null
                  ? Image.memory(_thumb!, fit: BoxFit.cover)
                  : Icon(
                      Icons.badge_outlined,
                      color: AppColors.muted.withValues(alpha: 0.7),
                    ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  widget.label,
                  style: const TextStyle(fontWeight: FontWeight.w900),
                ),
                const SizedBox(height: 2),
                Text(
                  _statusText,
                  style: TextStyle(
                    color: doc?.normalizedStatus == 'rejected'
                        ? const Color(0xFFB42318)
                        : AppColors.muted,
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                if (doc != null &&
                    doc.normalizedStatus == 'draft' &&
                    doc.originalName.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(
                    doc.originalName,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 11,
                      color: AppColors.muted,
                    ),
                  ),
                ],
              ],
            ),
          ),
          if (widget.busy)
            const SizedBox(
              width: 22,
              height: 22,
              child: CircularProgressIndicator(strokeWidth: 2.4),
            )
          else ...[
            if (widget.onUpload != null)
              TextButton(
                onPressed: widget.onUpload,
                child: Text(doc == null ? 'Upload' : 'Replace'),
              ),
            if (widget.onRemove != null)
              IconButton(
                tooltip: 'Remove',
                onPressed: widget.onRemove,
                icon: const Icon(Icons.delete_outline_rounded),
              ),
          ],
        ],
      ),
    );
  }
}

class IdentityActionChip extends StatelessWidget {
  const IdentityActionChip({super.key, required this.onOpen});

  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(right: 4),
      child: TextButton.icon(
        onPressed: onOpen,
        style: TextButton.styleFrom(
          foregroundColor: const Color(0xFFB45309),
          backgroundColor: const Color(0xFFFFF4E5),
          padding: const EdgeInsets.symmetric(horizontal: 10),
          visualDensity: VisualDensity.compact,
        ),
        icon: const Icon(Icons.shield_outlined, size: 16),
        label: const Text(
          'Action required',
          style: TextStyle(fontWeight: FontWeight.w800, fontSize: 12),
        ),
      ),
    );
  }
}
