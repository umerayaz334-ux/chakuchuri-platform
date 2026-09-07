import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:open_filex/open_filex.dart';

import '../api_client.dart';
import '../customer_kyc.dart';
import '../models.dart';
import '../theme.dart';
import '../widgets.dart';

/// Keep in sync with mobile/pubspec.yaml version (name+build).
const String kAppVersionName = '0.1.21';
const int kAppVersionCode = 2018;

typedef SettingsActionRunner = Future<void> Function(
  Future<void> Function() action,
  String success,
);

class ProfileSettingsScreen extends StatefulWidget {
  const ProfileSettingsScreen({
    super.key,
    required this.api,
    required this.session,
    required this.onSaved,
    required this.onLogout,
    this.embedded = false,
    this.customer,
    this.onNavigate,
    this.onCustomerUpdated,
    this.onAction,
  });

  final ApiClient api;
  final Session session;
  final ValueChanged<User> onSaved;
  final VoidCallback onLogout;
  final bool embedded;
  final Customer? customer;
  final ValueChanged<String>? onNavigate;
  final ValueChanged<Customer>? onCustomerUpdated;
  final SettingsActionRunner? onAction;

  @override
  State<ProfileSettingsScreen> createState() => _ProfileSettingsScreenState();
}

class _ProfileSettingsScreenState extends State<ProfileSettingsScreen> {
  late final TextEditingController _name;
  late final TextEditingController _email;
  late final TextEditingController _baseUrl;
  final _currentPassword = TextEditingController();
  final _newPassword = TextEditingController();
  final _confirmPassword = TextEditingController();

  Uint8List? _photoBytes;
  String _photoName = '';
  bool _removePhoto = false;
  bool _showPasswords = false;
  bool _busy = false;
  bool _inviteBusy = false;
  bool _updateBusy = false;
  double? _updateProgress;
  String _error = '';
  String _connectionNotice = '';
  String _updateNotice = '';
  int? _serverVersionCode;
  String? _serverVersionName;

  @override
  void initState() {
    super.initState();
    _name = TextEditingController(text: widget.session.user.name);
    _email = TextEditingController(text: widget.session.user.email);
    _baseUrl = TextEditingController(text: widget.api.baseUrl);
    unawaited(_refreshAndroidUpdateInfo());
  }

  @override
  void didUpdateWidget(covariant ProfileSettingsScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.api.baseUrl != widget.api.baseUrl &&
        _baseUrl.text.trim() == oldWidget.api.baseUrl) {
      _baseUrl.text = widget.api.baseUrl;
    }
  }

  @override
  void dispose() {
    _name.dispose();
    _email.dispose();
    _baseUrl.dispose();
    _currentPassword.dispose();
    _newPassword.dispose();
    _confirmPassword.dispose();
    super.dispose();
  }

  bool get _sensitiveChange {
    final emailChanged =
        _email.text.trim().toLowerCase() !=
        widget.session.user.email.toLowerCase();
    return emailChanged || _newPassword.text.trim().isNotEmpty;
  }

  Future<void> _pickPhoto() async {
    try {
      final selected = await ImagePicker().pickImage(
        source: ImageSource.gallery,
        imageQuality: 85,
        maxWidth: 1200,
        maxHeight: 1200,
        requestFullMetadata: false,
      );
      if (selected == null) return;
      final bytes = await selected.readAsBytes();
      if (bytes.isEmpty || bytes.length > 5 * 1024 * 1024) {
        setState(() => _error = 'Choose a JPG, PNG or WebP image up to 5 MB.');
        return;
      }
      setState(() {
        _photoBytes = bytes;
        _photoName = selected.name;
        _removePhoto = false;
        _error = '';
      });
    } catch (_) {
      setState(() => _error = 'The selected image could not be opened.');
    }
  }

  void _reset() {
    setState(() {
      _name.text = widget.session.user.name;
      _email.text = widget.session.user.email;
      _currentPassword.clear();
      _newPassword.clear();
      _confirmPassword.clear();
      _photoBytes = null;
      _photoName = '';
      _removePhoto = false;
      _error = '';
      _connectionNotice = '';
    });
  }

  void _saveBackendUrl() {
    final value = _baseUrl.text.trim();
    if (value.isEmpty) {
      setState(() {
        _connectionNotice = '';
        _error = 'Backend URL is required (e.g. http://192.168.10.2:8002).';
      });
      return;
    }
    final uri = Uri.tryParse(value);
    if (uri == null || !uri.hasScheme || uri.host.isEmpty) {
      setState(() {
        _connectionNotice = '';
        _error = 'Enter a valid URL like http://192.168.10.2:8002';
      });
      return;
    }
    try {
      widget.api.updateBaseUrl(value);
      setState(() {
        _baseUrl.text = widget.api.baseUrl;
        _error = '';
        _connectionNotice = 'Backend URL saved. Pull to refresh or reopen chats.';
      });
      unawaited(_refreshAndroidUpdateInfo());
    } on ApiException catch (error) {
      setState(() {
        _connectionNotice = '';
        _error = error.message;
      });
    }
  }

  int? _asVersionCode(Object? value) {
    if (value is int) return value;
    if (value is num) return value.toInt();
    if (value is String) return int.tryParse(value.trim());
    return null;
  }

  Future<void> _refreshAndroidUpdateInfo() async {
    try {
      final info = await widget.api.androidBuildInfo();
      if (!mounted) return;
      final ready = info['ready'] == true;
      final code = _asVersionCode(info['versionCode']);
      final name = (info['versionName'] as String?)?.trim();
      setState(() {
        _serverVersionCode = code;
        _serverVersionName = (name != null && name.isNotEmpty) ? name : null;
        if (!ready) {
          _updateNotice =
              'Update package not on server yet. Keep Backend URL pointed at your PC API.';
        } else if (code != null && code > kAppVersionCode) {
          _updateNotice =
              'Update available: ${_serverVersionName ?? 'build'} ($code).';
        } else {
          _updateNotice = 'You are on the latest build available from this server.';
        }
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _serverVersionCode = null;
        _serverVersionName = null;
        _updateNotice =
            'Could not check updates. Confirm Backend URL reaches the API on this Wi‑Fi.';
      });
    }
  }

  Future<void> _downloadAndInstallUpdate() async {
    setState(() {
      _updateBusy = true;
      _updateProgress = 0;
      _error = '';
      _updateNotice = 'Downloading update…';
    });
    try {
      final file = await widget.api.downloadAndroidApk(
        onProgress: (received, total) {
          if (!mounted) return;
          setState(() {
            _updateProgress = total != null && total > 0
                ? (received / total).clamp(0.0, 1.0)
                : null;
          });
        },
      );
      if (!mounted) return;
      setState(() {
        _updateNotice = 'Opening installer…';
        _updateProgress = 1;
      });
      final result = await OpenFilex.open(file.path);
      if (!mounted) return;
      if (result.type != ResultType.done) {
        setState(() {
          _updateNotice =
              'Download saved. Open the APK from Files if Install did not start (${result.message}).';
        });
      } else {
        setState(() {
          _updateNotice =
              'Installer opened. Allow install from this app if Android asks.';
        });
      }
    } on ApiException catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.message;
        _updateNotice = '';
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Update download failed. Check Wi‑Fi and Backend URL.';
        _updateNotice = '';
      });
    } finally {
      if (mounted) {
        setState(() {
          _updateBusy = false;
          _updateProgress = null;
        });
      }
    }
  }

  Future<void> _save() async {
    final name = _name.text.trim();
    final email = _email.text.trim().toLowerCase();
    final newPassword = _newPassword.text;
    final confirm = _confirmPassword.text;

    if (name.isEmpty || email.isEmpty) {
      setState(() => _error = 'Name and login email are required.');
      return;
    }
    if (newPassword.isNotEmpty && newPassword.length < 8) {
      setState(() => _error = 'New password must be at least 8 characters.');
      return;
    }
    if (newPassword != confirm) {
      setState(() => _error = 'New password and confirmation do not match.');
      return;
    }
    if (_sensitiveChange && _currentPassword.text.isEmpty) {
      setState(
        () => _error = 'Enter your current password to change login details.',
      );
      return;
    }

    setState(() {
      _busy = true;
      _error = '';
    });

    try {
      var profileImageFileId = widget.session.user.profileImageFileId;
      if (_photoBytes != null) {
        final uploaded = await widget.api.uploadFile(
          bytes: _photoBytes!,
          filename: _photoName.isEmpty ? 'profile.jpg' : _photoName,
          ownerType: 'user_profile',
          ownerId: widget.session.user.id,
          customerId: widget.session.user.customerId,
        );
        profileImageFileId = uploaded.id;
      }

      final user = await widget.api.updateProfile(
        name: name,
        email: email,
        currentPassword: _currentPassword.text,
        newPassword: newPassword,
        profileImageFileId: profileImageFileId,
        removeProfileImage: _removePhoto,
      );

      if (!mounted) return;
      widget.onSaved(user);
      setState(() {
        _currentPassword.clear();
        _newPassword.clear();
        _confirmPassword.clear();
        _photoBytes = null;
        _photoName = '';
        _removePhoto = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Personal account settings saved.')),
      );
    } on ApiException catch (error) {
      if (mounted) setState(() => _error = error.message);
    } catch (_) {
      if (mounted) {
        setState(() => _error = 'Account settings could not be saved.');
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _startVerification() async {
    final customer = widget.customer;
    if (customer == null || !customerCanStartVerification(customer)) return;
    if (customerNeedsIdentityAction(customer)) {
      widget.onNavigate?.call(verifyIdentityPageId);
      return;
    }
    setState(() {
      _inviteBusy = true;
      _error = '';
    });
    try {
      final runner = widget.onAction;
      if (runner != null) {
        await runner(() async {
          final next = await widget.api.openVerificationInvite(customer.id);
          widget.onCustomerUpdated?.call(next);
        }, 'Verification form opened.');
      } else {
        final next = await widget.api.openVerificationInvite(customer.id);
        widget.onCustomerUpdated?.call(next);
      }
      widget.onNavigate?.call(verifyIdentityPageId);
    } on ApiException catch (error) {
      if (error.message.toLowerCase().contains('in review')) {
        if (mounted) {
          setState(() => _error = '');
        }
        return;
      }
      if (mounted) setState(() => _error = error.message);
    } catch (_) {
      if (mounted) {
        setState(() => _error = 'Could not open verification form.');
      }
    } finally {
      if (mounted) setState(() => _inviteBusy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final user = widget.session.user;
    final customer = widget.customer;
    final hasPhoto =
        !_removePhoto &&
        (_photoBytes != null || user.profileImageFileId.isNotEmpty);
    final verified = isCustomerVerified(customer?.verificationStatus);
    final reviewPending = customerIdentityReviewPending(customer);
    final identityActionRequired = customerNeedsIdentityAction(customer);

    final body = ListView(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 28),
      children: [
        PageHeader(
          eyebrow: 'Personal settings',
          title: 'Your account',
          subtitle: verified
              ? 'Your account is verified.'
              : reviewPending
              ? 'We are verifying your documents. If anything further is needed, we will get back to you.'
              : 'Manage your profile photo, login name and password.',
          action: customer == null
              ? null
              : Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 10,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: verified || reviewPending
                        ? const Color(0xFFEDF7F3)
                        : identityActionRequired
                        ? const Color(0xFFFFF4E5)
                        : const Color(0xFFF3F4F6),
                    borderRadius: BorderRadius.circular(99),
                    border: Border.all(
                      color: verified || reviewPending
                          ? const Color(0xFFB7E0CF)
                          : identityActionRequired
                          ? const Color(0xFFF0C27A)
                          : const Color(0xFFD1D5DB),
                    ),
                  ),
                  child: Text(
                    verified
                        ? 'Verified'
                        : reviewPending
                        ? 'In review'
                        : identityActionRequired
                        ? 'Action required'
                        : verificationLabel(customer.verificationStatus),
                    style: TextStyle(
                      color: verified || reviewPending
                          ? AppColors.primaryDark
                          : identityActionRequired
                          ? const Color(0xFF9A3412)
                          : const Color(0xFF4B5563),
                      fontWeight: FontWeight.w800,
                      fontSize: 12,
                    ),
                  ),
                ),
        ),
        if (verified) ...[
          const SizedBox(height: 14),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: const Color(0xFFEDF7F3),
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: const Color(0xFFB7E0CF)),
            ),
            child: const Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(Icons.verified_rounded, color: AppColors.primary),
                SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Your account is verified',
                        style: TextStyle(
                          fontWeight: FontWeight.w900,
                          fontSize: 15,
                          color: AppColors.primaryDark,
                        ),
                      ),
                      SizedBox(height: 4),
                      Text(
                        'Identity checks are complete. This status lives here in Settings — not on Home.',
                        style: TextStyle(
                          color: AppColors.muted,
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
        if (customer != null && customerCanStartVerification(customer)) ...[
          const SizedBox(height: 14),
          Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: const Color(0xFFF7FBFA),
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: const Color(0xFFD7E5DD)),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Get verified',
                  style: TextStyle(fontWeight: FontWeight.w900, fontSize: 15),
                ),
                const SizedBox(height: 4),
                Text(
                  customerNeedsIdentityAction(customer)
                      ? 'Finish uploading CNIC and selfie, then submit once.'
                      : 'Open the one-time identity form. Upload your documents, then submit once.',
                  style: const TextStyle(
                    color: AppColors.muted,
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 12),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton.icon(
                    onPressed: _inviteBusy
                        ? null
                        : () {
                            if (customerNeedsIdentityAction(customer)) {
                              widget.onNavigate?.call(verifyIdentityPageId);
                              return;
                            }
                            _startVerification();
                          },
                    icon: const Icon(Icons.verified_outlined),
                    label: Text(
                      _inviteBusy
                          ? 'Opening…'
                          : customerNeedsIdentityAction(customer)
                          ? 'Continue verification'
                          : 'Get verified',
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],

        const SizedBox(height: 16),
        SectionCard(
          title: 'Profile photo',
          subtitle: 'JPG, PNG or WebP. A square image works best.',
          child: Row(
            children: [
              _ProfilePhotoPreview(
                api: widget.api,
                bytes: _photoBytes,
                fileId: _removePhoto ? '' : user.profileImageFileId,
                name: user.name,
                size: 78,
              ),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      user.email,
                      style: const TextStyle(
                        color: AppColors.muted,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        FilledButton.tonalIcon(
                          onPressed: _busy ? null : _pickPhoto,
                          icon: const Icon(Icons.photo_camera_outlined),
                          label: const Text('Choose photo'),
                        ),
                        if (hasPhoto)
                          OutlinedButton.icon(
                            onPressed: _busy
                                ? null
                                : () => setState(() {
                                    _photoBytes = null;
                                    _photoName = '';
                                    _removePhoto = true;
                                  }),
                            icon: const Icon(Icons.delete_outline_rounded),
                            label: const Text('Remove'),
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        SectionCard(
          title: 'Login details',
          subtitle: 'Display name and email used to sign in',
          child: Column(
            children: [
              TextField(
                controller: _name,
                enabled: !_busy,
                textCapitalization: TextCapitalization.words,
                decoration: const InputDecoration(
                  labelText: 'Display name',
                  prefixIcon: Icon(Icons.person_outline_rounded),
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _email,
                enabled: !_busy,
                keyboardType: TextInputType.emailAddress,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: 'Login email',
                  prefixIcon: Icon(Icons.mail_outline_rounded),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        SectionCard(
          title: 'Password',
          subtitle: 'Current password is required to change email or password',
          child: Column(
            children: [
              TextField(
                controller: _currentPassword,
                enabled: !_busy,
                obscureText: !_showPasswords,
                decoration: InputDecoration(
                  labelText: _sensitiveChange
                      ? 'Current password (required)'
                      : 'Current password',
                  prefixIcon: const Icon(Icons.key_rounded),
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _newPassword,
                enabled: !_busy,
                obscureText: !_showPasswords,
                decoration: const InputDecoration(
                  labelText: 'New password',
                  hintText: 'At least 8 characters',
                  prefixIcon: Icon(Icons.lock_outline_rounded),
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _confirmPassword,
                enabled: !_busy,
                obscureText: !_showPasswords,
                decoration: InputDecoration(
                  labelText: 'Confirm new password',
                  prefixIcon: const Icon(Icons.check_rounded),
                  suffixIcon: IconButton(
                    tooltip: _showPasswords
                        ? 'Hide passwords'
                        : 'Show passwords',
                    onPressed: () =>
                        setState(() => _showPasswords = !_showPasswords),
                    icon: Icon(
                      _showPasswords
                          ? Icons.visibility_off_outlined
                          : Icons.visibility_outlined,
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        SectionCard(
          title: 'Connection',
          subtitle: 'API endpoint for this device (saved securely with your session)',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                controller: _baseUrl,
                enabled: !_busy,
                decoration: InputDecoration(
                  labelText: 'Backend URL',
                  hintText: ApiClient.allowCleartext
                      ? 'http://192.168.x.x:8002'
                      : 'https://your-api-host',
                  prefixIcon: const Icon(Icons.cloud_outlined),
                  helperText: ApiClient.allowCleartext
                      ? 'Debug builds allow LAN HTTP. Use your PC Wi‑Fi IP on a phone.'
                      : 'Release builds need HTTPS (or a private LAN IP for demos).',
                ),
                keyboardType: TextInputType.url,
                autocorrect: false,
                onSubmitted: (_) => _saveBackendUrl(),
              ),
              const SizedBox(height: 10),
              OutlinedButton.icon(
                onPressed: _busy ? null : _saveBackendUrl,
                icon: const Icon(Icons.link_rounded),
                label: const Text('Save backend URL'),
              ),
              if (_connectionNotice.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text(
                  _connectionNotice,
                  style: const TextStyle(
                    color: AppColors.primary,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ],
              const SizedBox(height: 8),
              ListTile(
                contentPadding: EdgeInsets.zero,
                leading: Icon(
                  widget.api.storageAvailable
                      ? Icons.lock_rounded
                      : Icons.lock_open_rounded,
                  color: widget.api.storageAvailable
                      ? AppColors.primary
                      : AppColors.muted,
                ),
                title: Text(
                  widget.api.storageAvailable
                      ? 'Session stays signed in'
                      : 'Session may reset on restart',
                ),
                subtitle: Text(
                  widget.api.storageAvailable
                      ? 'Token and backend URL are stored securely on this device.'
                      : 'Secure storage is unavailable; you may need to sign in again.',
                ),
              ),
              const SizedBox(height: 8),
              ListTile(
                contentPadding: EdgeInsets.zero,
                leading: const Icon(Icons.system_update_alt_rounded),
                title: Text('App version $kAppVersionName (build $kAppVersionCode)'),
                subtitle: Text(
                  [
                    if (_serverVersionName != null && _serverVersionCode != null)
                      'Server has ${_serverVersionName!} (build $_serverVersionCode)',
                    if (_updateNotice.isNotEmpty) _updateNotice,
                  ].join('\n'),
                ),
              ),
              if (_updateProgress != null) ...[
                const SizedBox(height: 8),
                LinearProgressIndicator(value: _updateProgress),
              ],
              const SizedBox(height: 8),
              FilledButton.tonalIcon(
                onPressed: (_busy || _updateBusy)
                    ? null
                    : () => unawaited(_downloadAndInstallUpdate()),
                icon: _updateBusy
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.download_rounded),
                label: Text(_updateBusy ? 'Downloading…' : 'Download update'),
              ),
              TextButton(
                onPressed: (_busy || _updateBusy)
                    ? null
                    : () => unawaited(_refreshAndroidUpdateInfo()),
                child: const Text('Check for update'),
              ),
            ],
          ),
        ),
        if (user.loginActivity.isNotEmpty) ...[
          const SizedBox(height: 12),
          SectionCard(
            title: 'Recent account activity',
            child: Column(
              children: user.loginActivity.take(5).map((item) {
                return ListTile(
                  contentPadding: EdgeInsets.zero,
                  dense: true,
                  leading: Icon(
                    item.result.toLowerCase().contains('fail')
                        ? Icons.warning_amber_rounded
                        : Icons.history_rounded,
                    color: AppColors.muted,
                  ),
                  title: Text(item.result),
                  subtitle: Text(
                    [
                      if (item.detail.isNotEmpty) item.detail,
                      shortDate(item.at),
                    ].join(' · '),
                  ),
                );
              }).toList(),
            ),
          ),
        ],
        if (_error.isNotEmpty) ...[
          const SizedBox(height: 12),
          Text(
            _error,
            style: const TextStyle(
              color: AppColors.red,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
        const SizedBox(height: 16),
        SizedBox(
          width: double.infinity,
          child: FilledButton.icon(
            onPressed: _busy ? null : _save,
            icon: _busy
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: Colors.white,
                    ),
                  )
                : const Icon(Icons.save_rounded),
            label: Text(_busy ? 'Saving...' : 'Save changes'),
          ),
        ),
        const SizedBox(height: 10),
        Row(
          children: [
            Expanded(
              child: OutlinedButton(
                onPressed: _busy ? null : _reset,
                child: const Text('Reset'),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: OutlinedButton.icon(
                onPressed: _busy ? null : widget.onLogout,
                icon: const Icon(Icons.logout_rounded),
                label: const Text('Sign out'),
              ),
            ),
          ],
        ),
      ],
    );

    if (widget.embedded) return body;

    return Scaffold(
      appBar: AppBar(title: const Text('Settings')),
      body: body,
    );
  }
}

class _ProfilePhotoPreview extends StatefulWidget {
  const _ProfilePhotoPreview({
    required this.api,
    required this.name,
    required this.size,
    this.bytes,
    this.fileId = '',
  });

  final ApiClient api;
  final String name;
  final double size;
  final Uint8List? bytes;
  final String fileId;

  @override
  State<_ProfilePhotoPreview> createState() => _ProfilePhotoPreviewState();
}

class _ProfilePhotoPreviewState extends State<_ProfilePhotoPreview> {
  Uint8List? _remoteBytes;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(covariant _ProfilePhotoPreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.fileId != widget.fileId ||
        oldWidget.bytes != widget.bytes ||
        oldWidget.api.token != widget.api.token) {
      _load();
    }
  }

  Future<void> _load() async {
    if (widget.bytes != null || widget.fileId.trim().isEmpty) {
      setState(() => _remoteBytes = null);
      return;
    }
    try {
      final bytes = await widget.api.fetchFileBytes(widget.fileId);
      if (!mounted) return;
      setState(() => _remoteBytes = bytes);
    } catch (_) {
      if (!mounted) return;
      setState(() => _remoteBytes = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    final bytes = widget.bytes ?? _remoteBytes;
    return CircleAvatar(
      radius: widget.size / 2,
      backgroundColor: AppColors.surfaceSoft,
      backgroundImage: bytes == null ? null : MemoryImage(bytes),
      child: bytes == null
          ? Text(
              _initials(widget.name),
              style: TextStyle(
                color: AppColors.primaryDark,
                fontWeight: FontWeight.w900,
                fontSize: widget.size * 0.28,
              ),
            )
          : null,
    );
  }
}

String _initials(String value) {
  final words = value
      .trim()
      .split(RegExp(r'\s+'))
      .where((word) => word.isNotEmpty);
  final letters = words.take(2).map((word) => word[0].toUpperCase()).join();
  return letters.isEmpty ? 'CC' : letters;
}
