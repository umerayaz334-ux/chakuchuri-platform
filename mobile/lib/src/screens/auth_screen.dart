import 'package:flutter/material.dart';

import '../api_client.dart';
import '../models.dart';
import '../theme.dart';
import '../widgets.dart';

class AuthScreen extends StatefulWidget {
  const AuthScreen({
    super.key,
    required this.api,
    required this.onBaseUrlChanged,
    required this.onLogin,
    this.startOnSignup = false,
  });

  final ApiClient api;
  final ValueChanged<String> onBaseUrlChanged;
  final Future<void> Function(Session session) onLogin;
  final bool startOnSignup;

  @override
  State<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends State<AuthScreen> {
  final _loginEmail = TextEditingController(text: 'customer@chakuchuri.pk');
  final _loginPassword = TextEditingController(text: 'customer123');
  final _baseUrl = TextEditingController();
  final _company = TextEditingController();
  final _contact = TextEditingController();
  final _signupEmail = TextEditingController();
  final _phone = TextEditingController();
  final _country = TextEditingController(text: 'Pakistan');
  final _signupPassword = TextEditingController();
  bool _loading = false;
  late bool _signup;
  bool _showPassword = false;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _signup = widget.startOnSignup;
    _baseUrl.text = widget.api.baseUrl;
  }

  @override
  void dispose() {
    _loginEmail.dispose();
    _loginPassword.dispose();
    _baseUrl.dispose();
    _company.dispose();
    _contact.dispose();
    _signupEmail.dispose();
    _phone.dispose();
    _country.dispose();
    _signupPassword.dispose();
    super.dispose();
  }

  Future<void> _submitLogin() async {
    await _guard(() async {
      final session = await widget.api.login(
        _loginEmail.text,
        _loginPassword.text,
      );
      await widget.onLogin(session);
    });
  }

  Future<void> _submitSignup() async {
    await _guard(() async {
      final session = await widget.api.signup(
        companyName: _company.text,
        contactName: _contact.text,
        email: _signupEmail.text,
        phone: _phone.text,
        country: _country.text,
        password: _signupPassword.text,
      );
      await widget.onLogin(session);
    });
  }

  Future<void> _guard(Future<void> Function() action) async {
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      widget.onBaseUrlChanged(_baseUrl.text);
      await action();
    } on ApiException catch (error) {
      if (mounted) setState(() => _error = error.message);
    } catch (_) {
      if (mounted) {
        setState(
          () => _error =
              'Could not connect to ChakuChuri backend. Check Wi‑Fi and the Backend URL below (use your PC LAN IP, like http://192.168.x.x:8002).',
        );
      }
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Container(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Color(0xFFF8FAF7), Color(0xFFEEF4F0)],
          ),
        ),
        child: SafeArea(
          child: ListView(
            padding: const EdgeInsets.fromLTRB(18, 18, 18, 28),
            children: [
              Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 520),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const _AuthTopBrand(),
                      const SizedBox(height: 18),
                      Container(
                        padding: const EdgeInsets.all(4),
                        decoration: BoxDecoration(
                          color: AppColors.surfaceSoft,
                          borderRadius: BorderRadius.circular(12),
                          border: Border.all(color: AppColors.line),
                        ),
                        child: Row(
                          children: [
                            Expanded(
                              child: _AuthTab(
                                label: 'Login',
                                active: !_signup,
                                onTap: _loading
                                    ? null
                                    : () => setState(() {
                                          _signup = false;
                                          _error = '';
                                        }),
                              ),
                            ),
                            Expanded(
                              child: _AuthTab(
                                label: 'Create account',
                                active: _signup,
                                onTap: _loading
                                    ? null
                                    : () => setState(() {
                                          _signup = true;
                                          _error = '';
                                        }),
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 14),
                      if (_error.isNotEmpty) ...[
                        _ErrorBox(_error),
                        const SizedBox(height: 12),
                      ],
                      Container(
                        padding: const EdgeInsets.all(18),
                        decoration: BoxDecoration(
                          color: Colors.white,
                          borderRadius: BorderRadius.circular(16),
                          border: Border.all(color: AppColors.line),
                          boxShadow: [
                            BoxShadow(
                              color: Colors.black.withValues(alpha: 0.04),
                              blurRadius: 24,
                              offset: const Offset(0, 10),
                            ),
                          ],
                        ),
                        child: AutofillGroup(
                          child: AnimatedSwitcher(
                            duration: const Duration(milliseconds: 220),
                            child: _signup ? _signupForm(context) : _loginForm(context),
                          ),
                        ),
                      ),
                      const SizedBox(height: 12),
                      Material(
                        color: Colors.white,
                        clipBehavior: Clip.antiAlias,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(14),
                          side: const BorderSide(color: AppColors.line),
                        ),
                        child: ExpansionTile(
                          initiallyExpanded: true,
                          tilePadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 2),
                          childrenPadding: const EdgeInsets.fromLTRB(14, 0, 14, 14),
                          leading: const Icon(Icons.wifi_tethering_rounded, size: 20, color: AppColors.primary),
                          title: const Text(
                            'Connection settings',
                            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w800),
                          ),
                          subtitle: const Text(
                            'Same Wi‑Fi as your computer',
                            style: TextStyle(fontSize: 11, color: AppColors.muted),
                          ),
                          children: [
                            TextField(
                              controller: _baseUrl,
                              decoration: const InputDecoration(
                                labelText: 'Backend URL',
                                hintText: 'http://192.168.x.x:8002',
                                prefixIcon: Icon(Icons.link_rounded),
                              ),
                              keyboardType: TextInputType.url,
                            ),
                            const SizedBox(height: 8),
                            const Text(
                              'On a real phone, use your PC LAN IP. Emulators can use http://10.0.2.2:8002.',
                              style: TextStyle(color: AppColors.muted, fontSize: 12, height: 1.35),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _loginForm(BuildContext context) {
    return Column(
      key: const ValueKey('login-form'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Text(
          'SECURE PORTAL ACCESS',
          style: TextStyle(
            color: AppColors.primary,
            fontSize: 11,
            fontWeight: FontWeight.w900,
            letterSpacing: 0.8,
          ),
        ),
        const SizedBox(height: 6),
        Text('Welcome back', style: Theme.of(context).textTheme.titleLarge),
        const SizedBox(height: 5),
        const Text(
          'Sign in to manage quotes, orders, shipping and payments.',
          style: TextStyle(color: AppColors.muted),
        ),
        const SizedBox(height: 18),
        TextField(
          controller: _loginEmail,
          autofillHints: const [AutofillHints.email],
          keyboardType: TextInputType.emailAddress,
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: 'Email',
            prefixIcon: Icon(Icons.alternate_email_rounded),
          ),
        ),
        const SizedBox(height: 11),
        TextField(
          controller: _loginPassword,
          autofillHints: const [AutofillHints.password],
          obscureText: !_showPassword,
          onSubmitted: (_) => _loading ? null : _submitLogin(),
          decoration: InputDecoration(
            labelText: 'Password',
            prefixIcon: const Icon(Icons.lock_outline_rounded),
            suffixIcon: IconButton(
              tooltip: _showPassword ? 'Hide password' : 'Show password',
              onPressed: () => setState(() => _showPassword = !_showPassword),
              icon: Icon(
                _showPassword ? Icons.visibility_off_outlined : Icons.visibility_outlined,
              ),
            ),
          ),
        ),
        const SizedBox(height: 16),
        FilledButton(
          onPressed: _loading ? null : _submitLogin,
          style: FilledButton.styleFrom(
            minimumSize: const Size.fromHeight(50),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          ),
          child: _loading
              ? const SizedBox.square(
                  dimension: 18,
                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                )
              : const Text('Sign in'),
        ),
      ],
    );
  }

  Widget _signupForm(BuildContext context) {
    return Column(
      key: const ValueKey('signup-form'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Text(
          'NEW CUSTOMER',
          style: TextStyle(
            color: AppColors.primary,
            fontSize: 11,
            fontWeight: FontWeight.w900,
            letterSpacing: 0.8,
          ),
        ),
        const SizedBox(height: 6),
        Text('Create your account', style: Theme.of(context).textTheme.titleLarge),
        const SizedBox(height: 5),
        const Text(
          'One account for manufacturing, shipping and support.',
          style: TextStyle(color: AppColors.muted),
        ),
        const SizedBox(height: 18),
        TextField(
          controller: _company,
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: 'Company name',
            prefixIcon: Icon(Icons.business_outlined),
          ),
        ),
        const SizedBox(height: 10),
        TextField(
          controller: _contact,
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: 'Contact person',
            prefixIcon: Icon(Icons.person_outline_rounded),
          ),
        ),
        const SizedBox(height: 10),
        TextField(
          controller: _signupEmail,
          autofillHints: const [AutofillHints.email],
          keyboardType: TextInputType.emailAddress,
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: 'Email',
            prefixIcon: Icon(Icons.alternate_email_rounded),
          ),
        ),
        const SizedBox(height: 10),
        TextField(
          controller: _phone,
          autofillHints: const [AutofillHints.telephoneNumber],
          keyboardType: TextInputType.phone,
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: 'Phone',
            prefixIcon: Icon(Icons.phone_outlined),
          ),
        ),
        const SizedBox(height: 10),
        TextField(
          controller: _country,
          autofillHints: const [AutofillHints.countryName],
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: 'Country',
            prefixIcon: Icon(Icons.public_rounded),
          ),
        ),
        const SizedBox(height: 10),
        TextField(
          controller: _signupPassword,
          autofillHints: const [AutofillHints.newPassword],
          obscureText: !_showPassword,
          decoration: InputDecoration(
            labelText: 'Password',
            prefixIcon: const Icon(Icons.lock_outline_rounded),
            suffixIcon: IconButton(
              tooltip: _showPassword ? 'Hide password' : 'Show password',
              onPressed: () => setState(() => _showPassword = !_showPassword),
              icon: Icon(
                _showPassword ? Icons.visibility_off_outlined : Icons.visibility_outlined,
              ),
            ),
          ),
        ),
        const SizedBox(height: 16),
        FilledButton(
          onPressed: _loading ? null : _submitSignup,
          style: FilledButton.styleFrom(
            minimumSize: const Size.fromHeight(50),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          ),
          child: _loading
              ? const SizedBox.square(
                  dimension: 18,
                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                )
              : const Text('Create account'),
        ),
      ],
    );
  }
}

class _AuthTopBrand extends StatelessWidget {
  const _AuthTopBrand();

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        const AppLogoMark(size: 48),
        const SizedBox(width: 12),
        const Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'ChakuChuri.pk',
                style: TextStyle(
                  color: AppColors.ink,
                  fontSize: 20,
                  fontWeight: FontWeight.w900,
                ),
              ),
              Text(
                'Exporter service platform',
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
    );
  }
}

class _AuthTab extends StatelessWidget {
  const _AuthTab({
    required this.label,
    required this.active,
    required this.onTap,
  });

  final String label;
  final bool active;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: active ? Colors.white : Colors.transparent,
      borderRadius: BorderRadius.circular(9),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(9),
        child: Container(
          alignment: Alignment.center,
          padding: const EdgeInsets.symmetric(vertical: 12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(9),
            border: active ? Border.all(color: AppColors.line) : null,
          ),
          child: Text(
            label,
            style: TextStyle(
              color: active ? AppColors.ink : AppColors.muted,
              fontWeight: FontWeight.w800,
              fontSize: 13,
            ),
          ),
        ),
      ),
    );
  }
}

class _ErrorBox extends StatelessWidget {
  const _ErrorBox(this.message);

  final String message;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF1F1),
        border: Border.all(color: const Color(0xFFF3CACA)),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.error_outline_rounded, color: AppColors.red),
          const SizedBox(width: 9),
          Expanded(
            child: Text(
              message,
              style: const TextStyle(
                color: AppColors.red,
                fontWeight: FontWeight.w700,
                height: 1.35,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
