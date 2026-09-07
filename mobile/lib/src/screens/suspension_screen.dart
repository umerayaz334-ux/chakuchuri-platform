import 'package:flutter/material.dart';

import '../api_client.dart';
import '../models.dart';

class SuspensionScreen extends StatefulWidget {
  const SuspensionScreen({
    super.key,
    required this.user,
    required this.onContact,
    required this.onCheckStatus,
    required this.onSignOut,
  });

  final User user;
  final Future<void> Function(String) onContact;
  final Future<void> Function() onCheckStatus;
  final VoidCallback onSignOut;

  @override
  State<SuspensionScreen> createState() => _SuspensionScreenState();
}

class _SuspensionScreenState extends State<SuspensionScreen> {
  late final _message = TextEditingController(text: widget.user.suspensionContactMessage);
  bool _busy = false;
  String _feedback = '';

  @override
  void dispose() {
    _message.dispose();
    super.dispose();
  }

  Future<void> _run(Future<void> Function() action, String success) async {
    if (_busy) return;
    setState(() { _busy = true; _feedback = ''; });
    try {
      await action();
      if (mounted) setState(() => _feedback = success);
    } on ApiException catch (error) {
      if (mounted) setState(() => _feedback = error.message);
    } catch (_) {
      if (mounted) setState(() => _feedback = 'Could not connect. Please try again.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Account access'), actions: [
        IconButton(tooltip: 'Sign out', onPressed: widget.onSignOut, icon: const Icon(Icons.logout)),
      ]),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 480),
              child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                const Icon(Icons.pause_circle_outline, size: 40),
                const SizedBox(height: 16),
                Text('Account temporarily paused', style: Theme.of(context).textTheme.titleLarge),
                const SizedBox(height: 12),
                Text(widget.user.suspensionNotice.trim().isEmpty
                    ? 'Contact your administrator to restore access.'
                    : widget.user.suspensionNotice),
                const SizedBox(height: 24),
                TextField(
                  controller: _message,
                  minLines: 3,
                  maxLines: 6,
                  maxLength: 2000,
                  enabled: !_busy,
                  onChanged: (_) => setState(() {}),
                  decoration: const InputDecoration(labelText: 'Message to administrator', alignLabelWithHint: true),
                ),
                if (widget.user.suspensionContactAt.isNotEmpty && _feedback.isEmpty)
                  const Padding(padding: EdgeInsets.symmetric(vertical: 12), child: Text('Your review request has been sent.')),
                if (_feedback.isNotEmpty)
                  Padding(padding: const EdgeInsets.symmetric(vertical: 12),
                    child: Semantics(liveRegion: true, child: Text(_feedback))),
                FilledButton.icon(
                  onPressed: _busy || _message.text.trim().isEmpty ? null
                      : () => _run(() => widget.onContact(_message.text.trim()), 'Message sent to administrator.'),
                  icon: const Icon(Icons.send_outlined),
                  label: const Text('Contact administrator'),
                ),
                const SizedBox(height: 12),
                OutlinedButton.icon(
                  onPressed: _busy ? null : () => _run(widget.onCheckStatus, 'Account status checked.'),
                  icon: const Icon(Icons.refresh),
                  label: const Text('Check status'),
                ),
                if (_busy) const Padding(
                  padding: EdgeInsets.only(top: 16),
                  child: Center(child: CircularProgressIndicator()),
                ),
              ]),
            ),
          ),
        ),
      ),
    );
  }
}
