import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'theme.dart';

class AppTourStore {
  const AppTourStore({
    FlutterSecureStorage storage = const FlutterSecureStorage(),
  }) : _storage = storage;

  final FlutterSecureStorage _storage;
  static const _key = 'chakuchuri.tour.v1.done';

  Future<bool> hasCompleted() async {
    try {
      final value = await _storage.read(key: _key);
      return value == '1';
    } catch (_) {
      return true;
    }
  }

  Future<void> markCompleted() async {
    try {
      await _storage.write(key: _key, value: '1');
    } catch (_) {
      // The tour must never block the app when secure storage is unavailable.
    }
  }
}

class AppTourOverlay extends StatefulWidget {
  const AppTourOverlay({
    super.key,
    required this.onFinished,
    required this.onPageChanged,
  });

  final VoidCallback onFinished;
  final ValueChanged<String> onPageChanged;

  @override
  State<AppTourOverlay> createState() => _AppTourOverlayState();
}

class _AppTourOverlayState extends State<AppTourOverlay> {
  static const _steps =
      <({IconData icon, String page, String title, String body})>[
        (
          icon: Icons.home_rounded,
          page: 'Home',
          title: 'Home',
          body: 'See what needs attention and continue your latest work.',
        ),
        (
          icon: Icons.request_quote_rounded,
          page: 'Get Quote',
          title: 'Get Quote',
          body: 'Send a product photo and quantity, then review your price.',
        ),
        (
          icon: Icons.assignment_rounded,
          page: 'Orders',
          title: 'Orders',
          body:
              'Track the current stage, progress, balance, and delivery date.',
        ),
        (
          icon: Icons.local_shipping_rounded,
          page: 'Shipping',
          title: 'Shipping',
          body: 'Compare available rates and follow booked shipments.',
        ),
        (
          icon: Icons.account_balance_wallet_rounded,
          page: 'Payments',
          title: 'Payments',
          body: 'Check your balance and send payment proof securely.',
        ),
        (
          icon: Icons.chat_bubble_rounded,
          page: 'Messages',
          title: 'Messages',
          body: 'Chat, share files, and start a support call.',
        ),
        (
          icon: Icons.inventory_2_rounded,
          page: 'Products',
          title: 'Products',
          body: 'Keep your product references ready for faster quotations.',
        ),
        (
          icon: Icons.settings_rounded,
          page: 'Settings',
          title: 'Settings',
          body: 'Manage your account, profile photo, security, and connection.',
        ),
      ];

  int _index = 0;

  void _next() {
    if (_index >= _steps.length - 1) {
      widget.onFinished();
      return;
    }
    setState(() => _index += 1);
    widget.onPageChanged(_steps[_index].page);
  }

  void _back() {
    if (_index == 0) return;
    setState(() => _index -= 1);
    widget.onPageChanged(_steps[_index].page);
  }

  @override
  Widget build(BuildContext context) {
    final step = _steps[_index];
    final last = _index == _steps.length - 1;
    return Material(
      color: Colors.black.withValues(alpha: 0.55),
      child: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            children: [
              Align(
                alignment: Alignment.topRight,
                child: TextButton(
                  onPressed: widget.onFinished,
                  child: const Text(
                    'Skip',
                    style: TextStyle(
                      color: Colors.white,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
              ),
              const Spacer(),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.fromLTRB(20, 22, 20, 18),
                decoration: BoxDecoration(
                  color: Colors.white,
                  borderRadius: BorderRadius.circular(8),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.primary.withValues(alpha: 0.18),
                      blurRadius: 28,
                      offset: const Offset(0, 14),
                    ),
                  ],
                ),
                child: Column(
                  children: [
                    Container(
                      width: 64,
                      height: 64,
                      decoration: BoxDecoration(
                        color: AppColors.primary.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Icon(
                        step.icon,
                        color: AppColors.primary,
                        size: 32,
                      ),
                    ),
                    const SizedBox(height: 18),
                    Text(
                      step.title,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.w900,
                        color: AppColors.ink,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Text(
                      step.body,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        fontSize: 14,
                        height: 1.45,
                        color: AppColors.muted,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                    const SizedBox(height: 22),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        for (var i = 0; i < _steps.length; i++)
                          Container(
                            width: i == _index ? 18 : 7,
                            height: 7,
                            margin: const EdgeInsets.symmetric(horizontal: 3),
                            decoration: BoxDecoration(
                              color: i == _index
                                  ? AppColors.primary
                                  : AppColors.line,
                              borderRadius: BorderRadius.circular(99),
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(height: 18),
                    Row(
                      children: [
                        if (_index > 0) ...[
                          Expanded(
                            child: OutlinedButton(
                              onPressed: _back,
                              child: const Text('Back'),
                            ),
                          ),
                          const SizedBox(width: 10),
                        ],
                        Expanded(
                          flex: 2,
                          child: FilledButton(
                            onPressed: _next,
                            child: Text(last ? 'Finish' : 'Next'),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const Spacer(flex: 2),
            ],
          ),
        ),
      ),
    );
  }
}
