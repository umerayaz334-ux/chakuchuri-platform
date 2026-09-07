import 'dart:async';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' show ImageFilter;

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart' show ScrollDirection;
import 'package:image_picker/image_picker.dart';
import 'package:open_filex/open_filex.dart';
import 'package:path_provider/path_provider.dart';

import '../api_client.dart';
import '../accounting.dart';
import '../app_tour.dart';
import '../customer_kyc.dart';
import '../direct_call.dart';
import '../job_codes.dart';
import '../models.dart';
import '../theme.dart';
import '../widgets.dart';
import 'profile_settings_screen.dart';
import 'shipping_page.dart';
import 'verify_identity_page.dart';

part 'customer_dashboard.dart';

typedef ActionRunner = Future<void> Function(
  Future<void> Function() action,
  String success,
);

class _PickedImage {
  const _PickedImage(this.bytes, this.name);

  final Uint8List bytes;
  final String name;
}

Future<_PickedImage?> _chooseCompressedImage({
  int maxBytes = 10 * 1024 * 1024,
  String tooLargeMessage = 'Choose an image smaller than 10 MB.',
}) async {
  final selected = await ImagePicker().pickImage(
    source: ImageSource.gallery,
    imageQuality: 82,
    maxWidth: 1800,
    maxHeight: 1800,
    requestFullMetadata: false,
  );
  if (selected == null) return null;
  final bytes = await selected.readAsBytes();
  if (bytes.isEmpty || bytes.length > maxBytes) {
    throw ApiException(tooLargeMessage, code: 'invalid_file');
  }
  return _PickedImage(bytes, selected.name);
}

void _showImageError(BuildContext context, Object error) {
  final message = error is ApiException
      ? error.message
      : 'The selected image could not be opened.';
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
}

Future<void> _showFilePreview(
  BuildContext context, {
  required ApiClient api,
  required String fileId,
  String title = 'Product photo',
}) async {
  showDialog<void>(
    context: context,
    barrierColor: Colors.black.withValues(alpha: 0.82),
    builder: (dialogContext) {
      return Dialog(
        backgroundColor: Colors.transparent,
        insetPadding: const EdgeInsets.all(18),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Align(
              alignment: Alignment.centerRight,
              child: IconButton(
                onPressed: () => Navigator.of(dialogContext).pop(),
                icon: const Icon(Icons.close_rounded, color: Colors.white),
              ),
            ),
            Flexible(
              child: FutureBuilder<Uint8List>(
                future: api.fetchFileBytes(fileId, thumbnail: false),
                builder: (context, snapshot) {
                  if (snapshot.connectionState != ConnectionState.done) {
                    return const Padding(
                      padding: EdgeInsets.all(48),
                      child: CircularProgressIndicator(color: Colors.white),
                    );
                  }
                  if (snapshot.hasError || snapshot.data == null) {
                    return const Padding(
                      padding: EdgeInsets.all(24),
                      child: Text(
                        'Preview unavailable',
                        style: TextStyle(color: Colors.white),
                      ),
                    );
                  }
                  return InteractiveViewer(
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(12),
                      child: Image.memory(
                        Uint8List.fromList(snapshot.data!),
                        fit: BoxFit.contain,
                        gaplessPlayback: true,
                      ),
                    ),
                  );
                },
              ),
            ),
            const SizedBox(height: 10),
            Text(
              title,
              style: const TextStyle(
                color: Colors.white70,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
      );
    },
  );
}

class _ProductPhotoThumb extends StatefulWidget {
  const _ProductPhotoThumb({
    required this.api,
    required this.fileId,
    this.name = '',
    this.size = 42,
  });

  final ApiClient api;
  final String fileId;
  final String name;
  final double size;

  @override
  State<_ProductPhotoThumb> createState() => _ProductPhotoThumbState();
}

class _ProductPhotoThumbState extends State<_ProductPhotoThumb> {
  Uint8List? _bytes;
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(covariant _ProductPhotoThumb oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.fileId != widget.fileId ||
        oldWidget.api.baseUrl != widget.api.baseUrl ||
        oldWidget.api.token != widget.api.token) {
      _load();
    }
  }

  Future<void> _load() async {
    setState(() {
      _bytes = null;
      _failed = false;
    });
    try {
      final bytes = await widget.api.fetchFileBytes(widget.fileId);
      if (!mounted) return;
      setState(() => _bytes = bytes);
    } catch (_) {
      if (!mounted) return;
      setState(() => _failed = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final pixelSize = (widget.size * MediaQuery.devicePixelRatioOf(context))
        .round();
    final child = ClipRRect(
      borderRadius: BorderRadius.circular(8),
      child: SizedBox(
        width: widget.size,
        height: widget.size,
        child: _bytes != null
            ? Image.memory(
                _bytes!,
                fit: BoxFit.cover,
                gaplessPlayback: true,
                cacheWidth: pixelSize,
                cacheHeight: pixelSize,
                filterQuality: FilterQuality.low,
              )
            : ColoredBox(
                color: AppColors.surfaceSoft,
                child: Icon(
                  _failed ? Icons.broken_image_outlined : Icons.image_outlined,
                  color: AppColors.muted,
                  size: widget.size * 0.42,
                ),
              ),
      ),
    );
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: () => _showFilePreview(
          context,
          api: widget.api,
          fileId: widget.fileId,
          title: widget.name.isEmpty ? 'Product photo' : widget.name,
        ),
        child: child,
      ),
    );
  }
}

class _ProductPhotoRow extends StatelessWidget {
  const _ProductPhotoRow({
    required this.api,
    required this.fileIds,
    this.names = const [],
    this.size = 52,
  });

  final ApiClient api;
  final List<String> fileIds;
  final List<String> names;
  final double size;

  @override
  Widget build(BuildContext context) {
    if (fileIds.isEmpty) return const SizedBox.shrink();
    return SizedBox(
      height: size,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: fileIds.length.clamp(0, 5),
        separatorBuilder: (_, _) => const SizedBox(width: 8),
        itemBuilder: (context, index) {
          final name = index < names.length
              ? names[index]
              : 'Photo ${index + 1}';
          return _ProductPhotoThumb(
            api: api,
            fileId: fileIds[index],
            name: name,
            size: size,
          );
        },
      ),
    );
  }
}

class _SelectedImagePreview extends StatelessWidget {
  const _SelectedImagePreview({
    required this.bytes,
    required this.name,
    required this.onRemove,
  });

  final Uint8List bytes;
  final String name;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        border: Border.all(color: Theme.of(context).dividerColor),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(6),
            child: Image.memory(
              bytes,
              width: 52,
              height: 52,
              fit: BoxFit.cover,
              gaplessPlayback: true,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              name,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.bodyMedium
                  ?.copyWith(fontWeight: FontWeight.w600),
            ),
          ),
          IconButton(
            tooltip: 'Remove image',
            onPressed: onRemove,
            icon: const Icon(Icons.close_rounded),
          ),
        ],
      ),
    );
  }
}

class CustomerShell extends StatefulWidget {
  const CustomerShell({
    super.key,
    required this.api,
    required this.session,
    required this.workspace,
    required this.loading,
    required this.busy,
    required this.notice,
    required this.onAction,
    required this.onLogout,
    required this.onRefresh,
    required this.onLoadMore,
    required this.onUserUpdated,
    this.customer,
    this.onCustomerUpdated,
    this.notificationRoute,
    this.onNotificationRouteHandled,
    this.dashboardDate,
  });

  final ApiClient api;
  final Session session;
  final Workspace workspace;
  final Customer? customer;
  final bool loading;
  final bool busy;
  final String notice;
  final String? notificationRoute;
  final VoidCallback? onNotificationRouteHandled;
  final DateTime? dashboardDate;
  final ActionRunner onAction;
  final VoidCallback onLogout;
  final Future<void> Function() onRefresh;
  final Future<void> Function(String scope) onLoadMore;
  final ValueChanged<User> onUserUpdated;
  final ValueChanged<Customer>? onCustomerUpdated;

  @override
  State<CustomerShell> createState() => _CustomerShellState();
}

class _NavPage {
  const _NavPage({
    required this.id,
    required this.label,
    required this.icon,
    required this.selectedIcon,
    this.inBottomBar = false,
  });

  final String id;
  final String label;
  final IconData icon;
  final IconData selectedIcon;
  final bool inBottomBar;
}

class _CustomerShellState extends State<CustomerShell> {
  static const _basePages = <_NavPage>[
    _NavPage(
      id: 'Home',
      label: 'Home',
      icon: Icons.home_outlined,
      selectedIcon: Icons.home_rounded,
      inBottomBar: true,
    ),
    _NavPage(
      id: 'Get Quote',
      label: 'Get Quote',
      icon: Icons.request_quote_outlined,
      selectedIcon: Icons.request_quote_rounded,
      inBottomBar: true,
    ),
    _NavPage(
      id: 'Orders',
      label: 'Orders',
      icon: Icons.assignment_outlined,
      selectedIcon: Icons.assignment_rounded,
      inBottomBar: true,
    ),
    _NavPage(
      id: 'Shipping',
      label: 'Shipping',
      icon: Icons.local_shipping_outlined,
      selectedIcon: Icons.local_shipping_rounded,
      inBottomBar: true,
    ),
    _NavPage(
      id: 'Payments',
      label: 'Payments',
      icon: Icons.account_balance_wallet_outlined,
      selectedIcon: Icons.account_balance_wallet_rounded,
      inBottomBar: true,
    ),
    _NavPage(
      id: 'Messages',
      label: 'Messages',
      icon: Icons.chat_bubble_outline_rounded,
      selectedIcon: Icons.chat_bubble_rounded,
    ),
    _NavPage(
      id: 'Products',
      label: 'Products',
      icon: Icons.inventory_2_outlined,
      selectedIcon: Icons.inventory_2_rounded,
    ),
    _NavPage(
      id: 'Settings',
      label: 'Settings',
      icon: Icons.settings_outlined,
      selectedIcon: Icons.settings_rounded,
    ),
  ];

  static const _verifyPage = _NavPage(
    id: verifyIdentityPageId,
    label: 'Verify',
    icon: Icons.shield_outlined,
    selectedIcon: Icons.shield_rounded,
  );

  String _page = 'Home';
  String _lastBottomPage = 'Home';
  final _scaffoldKey = GlobalKey<ScaffoldState>();
  bool _showTour = false;
  bool _tourChecked = false;
  bool _loadingMore = false;

  List<String> get _pageScopes =>
      const {
        'Get Quote': ['quotations'],
        'Orders': ['manufacturing'],
        'Shipping': ['shipping'],
        'Payments': ['payments', 'ledger', 'manufacturing'],
        'Products': ['products'],
      }[_page] ??
      const [];

  PageInfo? get _pageInfo {
    final available = _pageScopes
        .map((scope) => widget.workspace.pagination[scope])
        .whereType<PageInfo>()
        .where((info) => info.hasMore)
        .toList();
    if (available.isEmpty) return null;
    return available.reduce(
      (left, right) =>
          left.total - left.loaded >= right.total - right.loaded ? left : right,
    );
  }

  Future<void> _loadMore() async {
    final scopes = _pageScopes.where(
      (scope) => widget.workspace.pagination[scope]?.hasMore == true,
    );
    if (scopes.isEmpty || _loadingMore) return;
    setState(() => _loadingMore = true);
    try {
      for (final scope in scopes) {
        await widget.onLoadMore(scope);
      }
    } finally {
      if (mounted) setState(() => _loadingMore = false);
    }
  }

  Widget? _loadMoreFooter() {
    if (_pageInfo?.hasMore != true) return null;
    return Padding(
      padding: const EdgeInsets.only(top: 8, bottom: 4),
      child: SizedBox(
        width: double.infinity,
        child: OutlinedButton.icon(
          onPressed: _loadingMore ? null : _loadMore,
          icon: _loadingMore
              ? const SizedBox.square(
                  dimension: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.expand_more_rounded),
          label: Text(_loadingMore ? 'Loading...' : 'Load more'),
        ),
      ),
    );
  }

  bool get _needsIdentity => customerNeedsIdentityAction(widget.customer);

  List<_NavPage> get _pages {
    if (!_needsIdentity) return _basePages;
    final settingsIndex = _basePages.indexWhere(
      (page) => page.id == 'Settings',
    );
    if (settingsIndex < 0) return [..._basePages, _verifyPage];
    return [
      ..._basePages.take(settingsIndex),
      _verifyPage,
      ..._basePages.skip(settingsIndex),
    ];
  }

  List<_NavPage> get _bottomPages =>
      _pages.where((page) => page.inBottomBar).toList();

  _NavPage get _current =>
      _pages.firstWhere((page) => page.id == _page, orElse: () => _pages.first);

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _consumeNotificationRoute();
      unawaited(_maybeShowTour());
    });
  }

  Future<void> _maybeShowTour() async {
    if (_tourChecked) return;
    _tourChecked = true;
    final done = await const AppTourStore().hasCompleted();
    if (!mounted || done) return;
    setState(() => _showTour = true);
  }

  Future<void> _finishTour() async {
    await const AppTourStore().markCompleted();
    if (!mounted) return;
    setState(() => _showTour = false);
  }

  void _tourPageChanged(String page) {
    if (mounted && _pages.any((item) => item.id == page)) {
      setState(() {
        _page = page;
        if (_bottomPages.any((item) => item.id == page)) {
          _lastBottomPage = page;
        }
      });
    }
  }

  @override
  void didUpdateWidget(covariant CustomerShell oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_page == verifyIdentityPageId && !_needsIdentity) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        if (_page == verifyIdentityPageId && !_needsIdentity) {
          setState(() => _page = 'Home');
        }
      });
    }
    if (widget.notificationRoute != oldWidget.notificationRoute) {
      WidgetsBinding.instance.addPostFrameCallback(
        (_) => _consumeNotificationRoute(),
      );
    }
  }

  void _consumeNotificationRoute() {
    final route = widget.notificationRoute?.trim() ?? '';
    if (route.isEmpty) return;
    // Calls use the always-on overlay; land on Home so the sheet is visible.
    final page = route == 'Calls' ? 'Home' : route;
    if (_pages.any((item) => item.id == page)) {
      setState(() => _page = page);
    }
    widget.onNotificationRouteHandled?.call();
  }

  void _go(String page) {
    if (!_pages.any((item) => item.id == page)) return;
    setState(() {
      _page = page;
      if (_bottomPages.any((item) => item.id == page)) {
        _lastBottomPage = page;
      }
    });
  }

  int? get _bottomIndex {
    final index = _bottomPages.indexWhere((page) => page.id == _page);
    return index < 0 ? null : index;
  }

  int get _persistentBottomIndex {
    final current = _bottomIndex;
    if (current != null) return current;
    final previous = _bottomPages.indexWhere(
      (page) => page.id == _lastBottomPage,
    );
    return previous < 0 ? 0 : previous;
  }

  Widget _pageBody() {
    // If identity is done, never keep showing the verify page under a Home title.
    final page = (_page == verifyIdentityPageId && !_needsIdentity)
        ? 'Home'
        : _page;
    switch (page) {
      case 'Get Quote':
        return QuotesPage(
          api: widget.api,
          workspace: widget.workspace,
          onAction: widget.onAction,
          onNavigate: _go,
          footer: _loadMoreFooter(),
        );
      case 'Orders':
        return OrdersPage(
          api: widget.api,
          workspace: widget.workspace,
          footer: _loadMoreFooter(),
        );
      case 'Messages':
        return MessagesPage(
          api: widget.api,
          workspace: widget.workspace,
          customerId: widget.session.user.customerId,
          onAction: widget.onAction,
        );
      case 'Shipping':
        return ShippingPage(
          api: widget.api,
          workspace: widget.workspace,
          onAction: widget.onAction,
          footer: _loadMoreFooter(),
        );
      case 'Payments':
        return PaymentsPage(
          api: widget.api,
          workspace: widget.workspace,
          onAction: widget.onAction,
          footer: _loadMoreFooter(),
        );
      case 'Products':
        return ProductsPage(
          workspace: widget.workspace,
          footer: _loadMoreFooter(),
        );
      case 'Settings':
        return ProfileSettingsScreen(
          api: widget.api,
          session: widget.session,
          customer: widget.customer,
          embedded: true,
          onSaved: widget.onUserUpdated,
          onLogout: widget.onLogout,
          onNavigate: _go,
          onCustomerUpdated: widget.onCustomerUpdated,
          onAction: widget.onAction,
        );
      case verifyIdentityPageId:
        if (widget.customer == null) {
          return const EmptyState(
            'Customer profile missing',
            detail: 'Pull to refresh, or contact ChakuChuri support.',
            icon: Icons.shield_outlined,
          );
        }
        return VerifyIdentityPage(
          api: widget.api,
          customer: widget.customer!,
          onAction: widget.onAction,
          onUpdated: (customer) => widget.onCustomerUpdated?.call(customer),
          onDone: () => _go('Home'),
        );
      case 'Home':
      default:
        return DashboardPage(
          date: widget.dashboardDate,
          api: widget.api,
          workspace: widget.workspace,
          userName: widget.session.user.name,
          onNavigate: _go,
          onAction: widget.onAction,
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    final bottomIndex = _bottomIndex;
    final secondaryPage = bottomIndex == null;

    return Stack(
      children: [
        Scaffold(
          key: _scaffoldKey,
          drawer: _PagesDrawer(
            pages: _pages,
            current: _page,
            session: widget.session,
            api: widget.api,
            onSelect: (page) {
              Navigator.of(context).pop();
              _go(page);
            },
            onLogout: widget.onLogout,
          ),
          appBar: AppBar(
            leading: IconButton(
              tooltip: secondaryPage ? 'Back' : 'Pages menu',
              onPressed: secondaryPage
                  ? () => _go(_lastBottomPage)
                  : () => _scaffoldKey.currentState?.openDrawer(),
              icon: Icon(
                secondaryPage ? Icons.arrow_back_rounded : Icons.menu_rounded,
              ),
            ),
            title: Row(
              children: [
                const AppLogoMark(size: 36),
                const SizedBox(width: 11),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Text(
                        'ChakuChuri.pk',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                      Text(
                        _current.label,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: AppColors.muted,
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            actions: [
              if (secondaryPage)
                IconButton(
                  tooltip: 'All pages',
                  onPressed: () => _scaffoldKey.currentState?.openDrawer(),
                  icon: const Icon(Icons.menu_rounded),
                ),
              if (_needsIdentity)
                IdentityActionChip(onOpen: () => _go(verifyIdentityPageId)),
              IconButton(
                tooltip: 'Messages',
                onPressed: () => _go('Messages'),
                icon: Badge(
                  isLabelVisible:
                      widget.workspace.conversations.fold<int>(
                        0,
                        (total, row) => total + row.unreadForCustomer,
                      ) >
                      0,
                  label: Text(
                    '${widget.workspace.conversations.fold<int>(0, (total, row) => total + row.unreadForCustomer)}',
                    style: const TextStyle(fontSize: 10),
                  ),
                  child: Icon(
                    _page == 'Messages'
                        ? Icons.chat_bubble_rounded
                        : Icons.chat_bubble_outline_rounded,
                  ),
                ),
              ),
              const SizedBox(width: 4),
            ],
            bottom: const PreferredSize(
              preferredSize: Size.fromHeight(1),
              child: Divider(height: 1),
            ),
          ),
          body: SafeArea(
            top: false,
            child: Column(
              children: [
                if (widget.notice.isNotEmpty) _NoticeBar(widget.notice),
                if (widget.loading || widget.busy)
                  const LinearProgressIndicator(
                    minHeight: 2,
                    color: AppColors.primary,
                  ),
                Expanded(
                  child: AnimatedSwitcher(
                    duration: const Duration(milliseconds: 160),
                    switchInCurve: Curves.easeOut,
                    switchOutCurve: Curves.easeIn,
                    transitionBuilder: (child, animation) =>
                        FadeTransition(opacity: animation, child: child),
                    child: KeyedSubtree(
                      key: ValueKey(_page),
                      child: _pageBody(),
                    ),
                  ),
                ),
              ],
            ),
          ),
          bottomNavigationBar: DecoratedBox(
            decoration: const BoxDecoration(
              border: Border(top: BorderSide(color: AppColors.line)),
            ),
            child: NavigationBar(
              selectedIndex: _persistentBottomIndex,
              labelBehavior: NavigationDestinationLabelBehavior.alwaysShow,
              onDestinationSelected: (value) => _go(_bottomPages[value].id),
              destinations: [
                for (final page in _bottomPages)
                  NavigationDestination(
                    icon: Icon(page.icon),
                    selectedIcon: Icon(page.selectedIcon),
                    label: page.id == 'Get Quote'
                        ? 'Quote'
                        : page.id == 'Shipping'
                        ? 'Ship'
                        : page.id == 'Payments'
                        ? 'Pay'
                        : page.label,
                  ),
              ],
            ),
          ),
        ),
        Positioned.fill(
          child: DirectCallOverlay(
            api: widget.api,
            session: widget.session,
            workspace: widget.workspace,
            onRefresh: widget.onRefresh,
          ),
        ),
        if (_showTour)
          Positioned.fill(
            child: AppTourOverlay(
              onPageChanged: _tourPageChanged,
              onFinished: () => unawaited(_finishTour()),
            ),
          ),
      ],
    );
  }
}

class _PagesDrawer extends StatelessWidget {
  const _PagesDrawer({
    required this.pages,
    required this.current,
    required this.session,
    required this.api,
    required this.onSelect,
    required this.onLogout,
  });

  final List<_NavPage> pages;
  final String current;
  final Session session;
  final ApiClient api;
  final ValueChanged<String> onSelect;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return Drawer(
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 10),
              child: Row(
                children: [
                  _HeaderAvatar(api: api, user: session.user, radius: 24),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          session.user.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w900,
                          ),
                        ),
                        Text(
                          session.user.email,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: AppColors.muted,
                            fontSize: 12,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            const Padding(
              padding: EdgeInsets.fromLTRB(16, 4, 16, 8),
              child: Text(
                'Pages',
                style: TextStyle(
                  color: AppColors.muted,
                  fontSize: 12,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ),
            Expanded(
              child: ListView(
                children: [
                  for (final page in pages)
                    ListTile(
                      selected: page.id == current,
                      leading: Icon(
                        page.id == current ? page.selectedIcon : page.icon,
                      ),
                      title: Text(
                        page.label,
                        style: TextStyle(
                          fontWeight: page.id == current
                              ? FontWeight.w900
                              : FontWeight.w700,
                        ),
                      ),
                      onTap: () => onSelect(page.id),
                    ),
                ],
              ),
            ),
            const Divider(height: 1),
            ListTile(
              leading: const Icon(Icons.logout_rounded),
              title: const Text('Sign out'),
              onTap: () {
                Navigator.of(context).pop();
                onLogout();
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _HeaderAvatar extends StatefulWidget {
  const _HeaderAvatar({
    required this.api,
    required this.user,
    this.radius = 18,
  });

  final ApiClient api;
  final User user;
  final double radius;

  @override
  State<_HeaderAvatar> createState() => _HeaderAvatarState();
}

class _HeaderAvatarState extends State<_HeaderAvatar> {
  Uint8List? _bytes;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(covariant _HeaderAvatar oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.user.profileImageFileId != widget.user.profileImageFileId ||
        oldWidget.api.token != widget.api.token) {
      _load();
    }
  }

  Future<void> _load() async {
    final id = widget.user.profileImageFileId.trim();
    if (id.isEmpty) {
      setState(() => _bytes = null);
      return;
    }
    try {
      final bytes = await widget.api.fetchFileBytes(id);
      if (!mounted) return;
      setState(() => _bytes = bytes);
    } catch (_) {
      if (!mounted) return;
      setState(() => _bytes = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    return CircleAvatar(
      radius: widget.radius,
      backgroundColor: AppColors.surfaceSoft,
      backgroundImage: _bytes == null ? null : MemoryImage(_bytes!),
      child: _bytes == null
          ? Text(
              _initials(widget.user.name),
              style: TextStyle(
                color: AppColors.primaryDark,
                fontSize: widget.radius * 0.66,
                fontWeight: FontWeight.w900,
              ),
            )
          : null,
    );
  }
}

class WorkHubPage extends StatefulWidget {
  const WorkHubPage({
    super.key,
    required this.api,
    required this.workspace,
    required this.onAction,
  });

  final ApiClient api;
  final Workspace workspace;
  final ActionRunner onAction;

  @override
  State<WorkHubPage> createState() => _WorkHubPageState();
}

class _WorkHubPageState extends State<WorkHubPage> {
  int _view = 0;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(14, 16, 14, 12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const PageHeader(
                eyebrow: 'Production desk',
                title: 'Quotes & orders',
                subtitle:
                    'Request pricing or track work already in production.',
              ),
              const SizedBox(height: 14),
              SizedBox(
                width: double.infinity,
                child: SegmentedButton<int>(
                  expandedInsets: EdgeInsets.zero,
                  segments: const [
                    ButtonSegment(
                      value: 0,
                      icon: Icon(Icons.request_quote_outlined),
                      label: Text('Quotes'),
                    ),
                    ButtonSegment(
                      value: 1,
                      icon: Icon(Icons.assignment_outlined),
                      label: Text('Orders'),
                    ),
                  ],
                  selected: {_view},
                  onSelectionChanged: (value) =>
                      setState(() => _view = value.first),
                ),
              ),
            ],
          ),
        ),
        Expanded(
          child: IndexedStack(
            index: _view,
            children: [
              QuotesPage(
                api: widget.api,
                workspace: widget.workspace,
                onAction: widget.onAction,
                embedded: true,
                onNavigate: (page) {
                  if (page == 'Orders') {
                    setState(() => _view = 1);
                  }
                },
              ),
              OrdersPage(
                api: widget.api,
                workspace: widget.workspace,
                embedded: true,
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _DashSectionTitle extends StatelessWidget {
  const _DashSectionTitle({required this.title, this.action});

  final String title;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Text(
            title,
            style: const TextStyle(
              color: AppColors.ink,
              fontSize: 16,
              fontWeight: FontWeight.w800,
            ),
          ),
        ),
        if (action != null) action!,
      ],
    );
  }
}

class _FeaturedProductsSlider extends StatefulWidget {
  const _FeaturedProductsSlider({
    required this.api,
    required this.items,
    required this.onOpenCatalog,
  });

  final ApiClient api;
  final List<FeaturedProduct> items;
  final VoidCallback onOpenCatalog;

  @override
  State<_FeaturedProductsSlider> createState() =>
      _FeaturedProductsSliderState();
}

class _FeaturedProductsSliderState extends State<_FeaturedProductsSlider> {
  final _controller = ScrollController();
  Timer? _autoScroll;
  DateTime _pausedUntil = DateTime.fromMillisecondsSinceEpoch(0);

  @override
  void initState() {
    super.initState();
    _autoScroll = Timer.periodic(const Duration(seconds: 4), (_) {
      if (!mounted ||
          widget.items.length < 2 ||
          !_controller.hasClients ||
          _controller.position.isScrollingNotifier.value ||
          WidgetsBinding.instance.lifecycleState != AppLifecycleState.resumed ||
          ModalRoute.of(context)?.isCurrent == false ||
          !TickerMode.valuesOf(context).enabled ||
          DateTime.now().isBefore(_pausedUntil) ||
          MediaQuery.disableAnimationsOf(context)) {
        return;
      }
      final position = _controller.position;
      final atEnd = position.pixels >= position.maxScrollExtent - 12;
      final target = atEnd
          ? 0.0
          : (position.pixels + 180).clamp(0.0, position.maxScrollExtent);
      unawaited(
        _controller.animateTo(
          target,
          duration: const Duration(milliseconds: 520),
          curve: Curves.easeInOutCubic,
        ),
      );
    });
  }

  @override
  void dispose() {
    _autoScroll?.cancel();
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final items = widget.items;
    const cardWidth = 168.0;
    const imageSize = 168.0;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _DashSectionTitle(
          title: 'Featured',
          action: TextButton(
            onPressed: widget.onOpenCatalog,
            child: const Text('Catalog'),
          ),
        ),
        const SizedBox(height: 8),
        SizedBox(
          height: imageSize + 72,
          child: NotificationListener<UserScrollNotification>(
            onNotification: (notification) {
              if (notification.direction != ScrollDirection.idle) {
                _pausedUntil = DateTime.now().add(const Duration(seconds: 8));
              }
              return false;
            },
            child: ListView.separated(
              controller: _controller,
              scrollDirection: Axis.horizontal,
              itemCount: items.length,
              separatorBuilder: (_, __) => const SizedBox(width: 12),
              itemBuilder: (context, index) {
                return SizedBox(
                  width: cardWidth,
                  child: _FeaturedCard(
                    api: widget.api,
                    item: items[index],
                    imageSize: imageSize,
                  ),
                );
              },
            ),
          ),
        ),
      ],
    );
  }
}

class _FeaturedCard extends StatelessWidget {
  const _FeaturedCard({
    required this.api,
    required this.item,
    required this.imageSize,
  });

  final ApiClient api;
  final FeaturedProduct item;
  final double imageSize;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SizedBox(
          width: imageSize,
          height: imageSize,
          child: Container(
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: AppColors.line),
              color: const Color(0xFF12352F),
            ),
            clipBehavior: Clip.antiAlias,
            child: Stack(
              fit: StackFit.expand,
              children: [
                _FeaturedImage(api: api, fileId: item.imageFileId),
                Positioned(
                  left: 10,
                  top: 10,
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 9,
                      vertical: 5,
                    ),
                    decoration: BoxDecoration(
                      color: Colors.white.withValues(alpha: 0.94),
                      borderRadius: BorderRadius.circular(99),
                    ),
                    child: Text(
                      item.tag,
                      style: const TextStyle(
                        color: AppColors.primaryDark,
                        fontWeight: FontWeight.w900,
                        fontSize: 11,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 10),
        Text(
          item.name,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          textAlign: TextAlign.center,
          style: const TextStyle(
            color: AppColors.ink,
            fontWeight: FontWeight.w800,
            fontSize: 14,
            height: 1.2,
          ),
        ),
        if (item.caption.isNotEmpty) ...[
          const SizedBox(height: 3),
          Text(
            item.caption,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.center,
            style: const TextStyle(
              color: AppColors.muted,
              fontSize: 12,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ],
    );
  }
}

class _FeaturedImage extends StatefulWidget {
  const _FeaturedImage({required this.api, required this.fileId});

  final ApiClient api;
  final String fileId;

  @override
  State<_FeaturedImage> createState() => _FeaturedImageState();
}

class _FeaturedImageState extends State<_FeaturedImage> {
  Uint8List? _bytes;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  @override
  void didUpdateWidget(covariant _FeaturedImage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.fileId != widget.fileId) unawaited(_load());
  }

  Future<void> _load() async {
    final id = widget.fileId.trim();
    if (id.isEmpty) {
      if (mounted) setState(() => _bytes = null);
      return;
    }
    try {
      final bytes = await widget.api.fetchFileBytes(id);
      if (!mounted) return;
      setState(() => _bytes = bytes);
    } catch (_) {
      if (mounted) setState(() => _bytes = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_bytes == null) {
      return const ColoredBox(
        color: Color(0xFF12352F),
        child: Center(
          child: Icon(Icons.image_outlined, color: Colors.white54, size: 36),
        ),
      );
    }
    return Image.memory(
      _bytes!,
      fit: BoxFit.cover,
      cacheWidth: 720,
      filterQuality: FilterQuality.low,
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

class ProductsPage extends StatelessWidget {
  const ProductsPage({super.key, required this.workspace, this.footer});

  final Workspace workspace;
  final Widget? footer;

  @override
  Widget build(BuildContext context) {
    final products = workspace.products;
    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 16, 14, 24),
      children: [
        const PageHeader(
          eyebrow: 'Catalog',
          title: 'Products',
          subtitle: 'Your product records and stock levels.',
        ),
        const SizedBox(height: 16),
        if (products.isEmpty)
          const EmptyState(
            'No products yet',
            detail: 'Catalog items linked to your account will appear here.',
            icon: Icons.inventory_2_outlined,
          )
        else
          ...products.map((product) {
            return Container(
              margin: const EdgeInsets.only(bottom: 10),
              padding: const EdgeInsets.all(14),
              decoration: tileDecoration(),
              child: Row(
                children: [
                  Container(
                    width: 44,
                    height: 44,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: AppColors.surfaceSoft,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: const Icon(
                      Icons.inventory_2_outlined,
                      color: AppColors.primary,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          product.sku,
                          style: const TextStyle(
                            color: AppColors.muted,
                            fontSize: 11,
                            fontWeight: FontWeight.w800,
                          ),
                        ),
                        const SizedBox(height: 3),
                        Text(
                          product.name,
                          style: const TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w900,
                          ),
                        ),
                      ],
                    ),
                  ),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text(
                        '${product.stock}',
                        style: const TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                      Text(
                        '${product.reserved} reserved',
                        style: const TextStyle(
                          color: AppColors.muted,
                          fontSize: 11,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            );
          }),
        if (footer != null) footer!,
      ],
    );
  }
}

class QuotesPage extends StatefulWidget {
  const QuotesPage({
    super.key,
    required this.api,
    required this.workspace,
    required this.onAction,
    this.onNavigate,
    this.embedded = false,
    this.footer,
  });

  final ApiClient api;
  final Workspace workspace;
  final ActionRunner onAction;
  final ValueChanged<String>? onNavigate;
  final bool embedded;
  final Widget? footer;

  @override
  State<QuotesPage> createState() => _QuotesPageState();
}

class _QuotesPageState extends State<QuotesPage> {
  final _productName = TextEditingController();
  final _quantity = TextEditingController();
  final _notes = TextEditingController();
  Uint8List? _imageBytes;
  String _imageName = '';
  bool _submitting = false;
  String _filter = 'All';

  @override
  void dispose() {
    _productName.dispose();
    _quantity.dispose();
    _notes.dispose();
    super.dispose();
  }

  Future<void> _pickImage() async {
    try {
      final selected = await _chooseCompressedImage(
        maxBytes: 2 * 1024 * 1024,
        tooLargeMessage:
            'Quotation photos must be 2 MB or smaller — about a normal phone screenshot. Compress or choose a smaller image.',
      );
      if (selected == null || !mounted) return;
      setState(() {
        _imageBytes = selected.bytes;
        _imageName = selected.name;
      });
    } catch (error) {
      if (mounted) _showImageError(context, error);
    }
  }

  Future<void> _openQuoteSheet() async {
    _productName.clear();
    _quantity.clear();
    _notes.clear();
    _imageBytes = null;
    _imageName = '';
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: false,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(8)),
      ),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) {
          return Padding(
            padding: EdgeInsets.fromLTRB(
              16,
              10,
              16,
              16 + MediaQuery.viewInsetsOf(sheetContext).bottom,
            ),
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Center(
                    child: Container(
                      width: 42,
                      height: 4,
                      decoration: BoxDecoration(
                        color: AppColors.line,
                        borderRadius: BorderRadius.circular(99),
                      ),
                    ),
                  ),
                  const SizedBox(height: 14),
                  Row(
                    children: [
                      Container(
                        width: 38,
                        height: 38,
                        alignment: Alignment.center,
                        decoration: BoxDecoration(
                          color: AppColors.surfaceSoft,
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: const Icon(
                          Icons.request_quote_outlined,
                          color: AppColors.primary,
                        ),
                      ),
                      const SizedBox(width: 10),
                      const Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'Request a quote',
                              style: TextStyle(
                                fontSize: 20,
                                fontWeight: FontWeight.w900,
                              ),
                            ),
                            Text(
                              'Add a photo and quantity.',
                              style: TextStyle(
                                color: AppColors.muted,
                                fontSize: 12,
                              ),
                            ),
                          ],
                        ),
                      ),
                      IconButton(
                        tooltip: 'Close',
                        onPressed: _submitting
                            ? null
                            : () => Navigator.pop(sheetContext),
                        icon: const Icon(Icons.close_rounded),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  if (_imageBytes == null)
                    Material(
                      color: AppColors.surfaceSoft,
                      borderRadius: BorderRadius.circular(8),
                      child: InkWell(
                        onTap: _submitting
                            ? null
                            : () async {
                                await _pickImage();
                                if (sheetContext.mounted) {
                                  setSheetState(() {});
                                }
                              },
                        borderRadius: BorderRadius.circular(8),
                        child: Container(
                          width: double.infinity,
                          height: 132,
                          alignment: Alignment.center,
                          decoration: BoxDecoration(
                            border: Border.all(color: AppColors.line),
                            borderRadius: BorderRadius.circular(8),
                          ),
                          child: const Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(
                                Icons.add_photo_alternate_outlined,
                                color: AppColors.primary,
                                size: 32,
                              ),
                              SizedBox(height: 8),
                              Text(
                                'Add product photo',
                                style: TextStyle(fontWeight: FontWeight.w800),
                              ),
                              SizedBox(height: 4),
                              Text(
                                'Max 2 MB — normal screenshot size',
                                style: TextStyle(
                                  color: AppColors.muted,
                                  fontSize: 12,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    )
                  else
                    _SelectedImagePreview(
                      bytes: _imageBytes!,
                      name: _imageName,
                      onRemove: () {
                        if (_submitting) return;
                        setSheetState(() {
                          _imageBytes = null;
                          _imageName = '';
                        });
                      },
                    ),
                  const SizedBox(height: 12),
                  TextField(
                    controller: _quantity,
                    enabled: !_submitting,
                    autofocus: false,
                    decoration: const InputDecoration(
                      labelText: 'Quantity',
                      suffixText: 'pieces',
                      prefixIcon: Icon(Icons.numbers_rounded),
                    ),
                    keyboardType: TextInputType.number,
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: _productName,
                    enabled: !_submitting,
                    textCapitalization: TextCapitalization.words,
                    decoration: const InputDecoration(
                      labelText: 'Product name (optional)',
                      prefixIcon: Icon(Icons.design_services_outlined),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: _notes,
                    enabled: !_submitting,
                    minLines: 2,
                    maxLines: 3,
                    textCapitalization: TextCapitalization.sentences,
                    decoration: const InputDecoration(
                      labelText: 'Note (optional)',
                      hintText: 'Anything we should know',
                    ),
                  ),
                  const SizedBox(height: 16),
                  SizedBox(
                    width: double.infinity,
                    height: 48,
                    child: FilledButton.icon(
                      onPressed: _submitting
                          ? null
                          : () async {
                              final created = await _submit(
                                () => setSheetState(() {}),
                              );
                              if (created && sheetContext.mounted) {
                                Navigator.pop(sheetContext);
                              }
                            },
                      icon: _submitting
                          ? const SizedBox.square(
                              dimension: 17,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: Colors.white,
                              ),
                            )
                          : const Icon(Icons.send_rounded),
                      label: Text(
                        _submitting ? 'Sending request...' : 'Send request',
                      ),
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Future<bool> _submit(VoidCallback refreshSheet) async {
    if (_submitting) return false;
    final name = _productName.text.trim().isEmpty
        ? 'Product consultation'
        : _productName.text.trim();
    final quantity = int.tryParse(_quantity.text.trim()) ?? 0;
    if (_imageBytes == null || quantity <= 0) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Add a product photo and enter the quantity.'),
        ),
      );
      return false;
    }
    setState(() => _submitting = true);
    refreshSheet();
    var created = false;
    try {
      await widget.onAction(() async {
        final uploaded = await widget.api.uploadFile(
          bytes: _imageBytes!,
          filename: _imageName,
          ownerType: 'quotation_photo',
        );
        await widget.api.createQuote(
          productId: '',
          productName: name,
          quantity: quantity,
          notes: _notes.text,
          imageName: uploaded.originalName,
          imageFileId: uploaded.id,
        );
        if (mounted) {
          setState(() {
            _productName.clear();
            _quantity.clear();
            _notes.clear();
            _imageBytes = null;
            _imageName = '';
          });
        }
        created = true;
      }, 'Quote request sent.');
    } finally {
      if (mounted) {
        setState(() => _submitting = false);
        refreshSheet();
      }
    }
    return created;
  }

  @override
  Widget build(BuildContext context) {
    final quotes = [...widget.workspace.quotations]
      ..sort((a, b) {
        final recent = _quoteLatestActivity(b)
            .compareTo(_quoteLatestActivity(a));
        return recent != 0 ? recent : b.id.compareTo(a.id);
      });
    final review = quotes.where((quote) => quote.status == 'Requested').length;
    final priced = quotes.where((quote) => quote.status == 'Priced').length;
    final accepted = quotes.where((quote) => quote.status == 'Accepted').length;
    final visible = quotes.where((quote) {
      switch (_filter) {
        case 'Accepted':
          return quote.status == 'Accepted';
        case 'Rejected / cancelled':
          return quote.status == 'Rejected' || quote.status == 'Cancelled';
        default:
          return true;
      }
    }).toList();
    const filters = ['All', 'Accepted', 'Rejected / cancelled'];
    return ListView(
      padding: widget.embedded
          ? const EdgeInsets.fromLTRB(14, 0, 14, 24)
          : const EdgeInsets.fromLTRB(14, 16, 14, 24),
      children: [
        if (!widget.embedded) ...[
          const PageHeader(
            eyebrow: 'Sourcing',
            title: 'Quotations',
            subtitle: 'Send a product photo and quantity to get your price.',
          ),
          const SizedBox(height: 14),
        ],
        SizedBox(
          width: double.infinity,
          child: FilledButton.icon(
            onPressed: _openQuoteSheet,
            icon: const Icon(Icons.add_rounded),
            label: const Text('Request a quote'),
          ),
        ),
        const SizedBox(height: 18),
        Container(
          decoration: tileDecoration(),
          child: Row(
            children: [
              Expanded(
                child: _QuoteSummaryMetric(
                  label: 'In review',
                  value: review,
                  color: AppColors.blue,
                ),
              ),
              const SizedBox(height: 48, child: VerticalDivider()),
              Expanded(
                child: _QuoteSummaryMetric(
                  label: 'Ready',
                  value: priced,
                  color: AppColors.amber,
                ),
              ),
              const SizedBox(height: 48, child: VerticalDivider()),
              Expanded(
                child: _QuoteSummaryMetric(
                  label: 'Orders',
                  value: accepted,
                  color: AppColors.primary,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        Row(
          children: [
            const Expanded(
              child: Text(
                'Your requests',
                style: TextStyle(fontSize: 17, fontWeight: FontWeight.w900),
              ),
            ),
            Text(
              '${visible.length}',
              style: const TextStyle(
                color: AppColors.muted,
                fontSize: 13,
                fontWeight: FontWeight.w800,
              ),
            ),
          ],
        ),
        const SizedBox(height: 9),
        SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              for (final item in filters)
                Padding(
                  padding: const EdgeInsets.only(right: 8),
                  child: ChoiceChip(
                    label: Text(item),
                    selected: _filter == item,
                    onSelected: (_) => setState(() => _filter = item),
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        if (quotes.isEmpty)
          const EmptyState(
            'No quotes yet',
            detail: 'Request pricing with a product photo and quantity.',
            icon: Icons.request_quote_outlined,
          )
        else if (visible.isEmpty)
          const EmptyState(
            'Nothing here',
            detail: 'No quotations match this filter.',
            icon: Icons.filter_alt_off_outlined,
          )
        else
          ...visible.map((quote) {
            return QuoteTile(
              api: widget.api,
              quote: quote,
              onAccept: quote.status == 'Priced'
                  ? () async {
                      await widget.onAction(
                        () => widget.api.acceptQuote(quote.id),
                        'Order created from quote.',
                      );
                      widget.onNavigate?.call('Orders');
                    }
                  : null,
              onReject: quote.status == 'Priced'
                  ? () => widget.onAction(
                      () => widget.api.rejectQuote(quote.id),
                      'Quote rejected.',
                    )
                  : null,
              onOpenOrder: quote.status == 'Accepted'
                  ? () => widget.onNavigate?.call('Orders')
                  : null,
            );
          }),
        if (!widget.embedded && widget.footer != null) widget.footer!,
      ],
    );
  }
}

class _QuoteSummaryMetric extends StatelessWidget {
  const _QuoteSummaryMetric({
    required this.label,
    required this.value,
    required this.color,
  });

  final String label;
  final int value;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '$value',
            style: TextStyle(
              color: color,
              fontSize: 20,
              height: 1,
              fontWeight: FontWeight.w900,
            ),
          ),
          const SizedBox(height: 5),
          Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(
              color: AppColors.muted,
              fontSize: 11,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ),
    );
  }
}

class OrdersPage extends StatefulWidget {
  const OrdersPage({
    super.key,
    required this.api,
    required this.workspace,
    this.embedded = false,
    this.footer,
  });

  final ApiClient api;
  final Workspace workspace;
  final bool embedded;
  final Widget? footer;

  @override
  State<OrdersPage> createState() => _OrdersPageState();
}

class _OrdersPageState extends State<OrdersPage> {
  String _filter = 'All';

  String _latestActivity(ManufacturingOrder order) {
    var latest = '';
    for (final update in order.history) {
      if (update.createdAt.compareTo(latest) > 0) latest = update.createdAt;
    }
    return latest.isEmpty ? order.id : latest;
  }

  List<ManufacturingOrder> get _visible {
    final orders = [...widget.workspace.manufacturing];
    orders.sort((a, b) {
      final recent = _latestActivity(b).compareTo(_latestActivity(a));
      return recent != 0 ? recent : b.id.compareTo(a.id);
    });
    return orders.where((order) {
      final status = order.status.toLowerCase();
      switch (_filter) {
        case 'Active':
          return !isClosedStatus(order.status);
        case 'Ready':
          return status.contains('ready');
        case 'Completed':
          return status == 'completed';
        case 'Cancelled':
          return status == 'cancelled';
        default:
          return true;
      }
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final filters = const ['All', 'Active', 'Ready', 'Completed', 'Cancelled'];
    return ListView(
      padding: widget.embedded
          ? const EdgeInsets.fromLTRB(14, 0, 14, 24)
          : const EdgeInsets.all(14),
      children: [
        SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              for (final item in filters) ...[
                Padding(
                  padding: const EdgeInsets.only(right: 8),
                  child: ChoiceChip(
                    label: Text(item),
                    selected: _filter == item,
                    onSelected: (_) => setState(() => _filter = item),
                  ),
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 12),
        if (widget.workspace.manufacturing.isEmpty)
          const EmptyState('No manufacturing orders yet')
        else if (_visible.isEmpty)
          EmptyState('No $_filter orders', detail: 'Try another filter.')
        else
          ..._visible.map(
            (order) => Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: _ModernOrderCard(
                api: widget.api,
                order: order,
                quotes: widget.workspace.quotations,
              ),
            ),
          ),
        if (!widget.embedded && widget.footer != null) widget.footer!,
      ],
    );
  }
}

class PaymentsPage extends StatefulWidget {
  const PaymentsPage({
    super.key,
    required this.api,
    required this.workspace,
    required this.onAction,
    this.footer,
  });

  final ApiClient api;
  final Workspace workspace;
  final ActionRunner onAction;
  final Widget? footer;

  @override
  State<PaymentsPage> createState() => _PaymentsPageState();
}

class _PaymentsPageState extends State<PaymentsPage> {
  final _amount = TextEditingController();
  final _note = TextEditingController();
  Uint8List? _proofBytes;
  String _proofName = '';
  String _view = 'proofs';

  @override
  void dispose() {
    _amount.dispose();
    _note.dispose();
    super.dispose();
  }

  Future<void> _pickProof() async {
    try {
      final selected = await _chooseCompressedImage();
      if (selected == null || !mounted) return;
      setState(() {
        _proofBytes = selected.bytes;
        _proofName = selected.name;
      });
    } catch (error) {
      if (mounted) _showImageError(context, error);
    }
  }

  Future<void> _submit() async {
    final amount = int.tryParse(_amount.text.replaceAll(',', '').trim()) ?? 0;
    if (amount <= 0) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Enter the payment amount in rupees.')),
      );
      return;
    }
    if (_proofBytes == null) {
      _showImageError(
        context,
        const ApiException('Choose a receipt screenshot first.'),
      );
      return;
    }
    var ok = false;
    await widget.onAction(
      () async {
        final uploaded = await widget.api.uploadFile(
          bytes: _proofBytes!,
          filename: _proofName,
          ownerType: 'payment_proof',
          ownerId: '',
        );
        await widget.api.createPayment(
          type: 'Account payment',
          amount: amount,
          manufacturingId: '',
          shippingId: '',
          proofName: uploaded.originalName,
          proofFileId: uploaded.id,
          note: _note.text,
        );
        if (mounted) {
          setState(() {
            _amount.clear();
            _note.clear();
            _proofBytes = null;
            _proofName = '';
          });
        }
        ok = true;
      },
      'Payment proof submitted. After approval it clears open balances automatically.',
    );
    if (mounted && ok && Navigator.of(context).canPop()) {
      Navigator.of(context).pop();
    }
  }

  Future<void> _openProofSheet(int accountDue) async {
    _amount.clear();
    _note.clear();
    _proofBytes = null;
    _proofName = '';
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(8)),
      ),
      builder: (context) {
        return StatefulBuilder(
          builder: (context, setModalState) {
            return Padding(
              padding: EdgeInsets.fromLTRB(
                16,
                12,
                16,
                16 + MediaQuery.viewInsetsOf(context).bottom,
              ),
              child: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Center(
                      child: Container(
                        width: 42,
                        height: 4,
                        decoration: BoxDecoration(
                          color: const Color(0xFFD7E2DC),
                          borderRadius: BorderRadius.circular(99),
                        ),
                      ),
                    ),
                    const SizedBox(height: 14),
                    Row(
                      children: [
                        const Expanded(
                          child: Text(
                            'Upload payment proof',
                            style: TextStyle(
                              fontSize: 20,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                        ),
                        IconButton(
                          onPressed: () => Navigator.pop(context),
                          icon: const Icon(Icons.close_rounded),
                        ),
                      ],
                    ),
                    if (accountDue > 0) ...[
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: const Color(0xFFFFF7ED),
                          borderRadius: BorderRadius.circular(8),
                          border: Border.all(color: const Color(0xFFF0DCC4)),
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Text(
                              'BALANCE DUE',
                              style: TextStyle(
                                color: Color(0xFF9A6700),
                                fontSize: 10,
                                fontWeight: FontWeight.w800,
                              ),
                            ),
                            Text(
                              money(accountDue),
                              style: const TextStyle(
                                fontSize: 22,
                                fontWeight: FontWeight.w900,
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 14),
                    ],
                    TextField(
                      controller: _amount,
                      keyboardType: TextInputType.number,
                      decoration: InputDecoration(
                        labelText: 'Amount paid',
                        prefixText: 'Rs ',
                        hintText: accountDue > 0 ? '$accountDue' : '0',
                      ),
                    ),
                    const SizedBox(height: 12),
                    SizedBox(
                      width: double.infinity,
                      child: OutlinedButton.icon(
                        onPressed: () async {
                          await _pickProof();
                          setModalState(() {});
                        },
                        icon: const Icon(Icons.add_photo_alternate_outlined),
                        label: Text(
                          _proofBytes == null
                              ? 'Choose transfer screenshot'
                              : 'Change screenshot',
                        ),
                      ),
                    ),
                    if (_proofBytes != null) ...[
                      const SizedBox(height: 10),
                      _SelectedImagePreview(
                        bytes: _proofBytes!,
                        name: _proofName,
                        onRemove: () => setModalState(() {
                          _proofBytes = null;
                          _proofName = '';
                        }),
                      ),
                    ],
                    const SizedBox(height: 12),
                    TextField(
                      controller: _note,
                      decoration: const InputDecoration(
                        labelText: 'Note (optional)',
                        hintText: 'Bank reference or transfer date',
                      ),
                    ),
                    const SizedBox(height: 16),
                    SizedBox(
                      width: double.infinity,
                      height: 48,
                      child: FilledButton.icon(
                        onPressed: _submit,
                        icon: const Icon(Icons.upload_rounded),
                        label: const Text('Submit proof'),
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final dueOrders = widget.workspace.manufacturing
        .where(
          (order) =>
              order.balanceDue > 0 &&
              order.status != 'Cancelled' &&
              order.status != 'Completed',
        )
        .toList();
    final position = AccountPosition(widget.workspace);
    final accountDue = position.due;
    final credit = position.credit;
    final pendingProofs = position.pending;
    final confirmed = position.received;
    final charges = position.charges - position.adjustments;
    final recent = [...widget.workspace.payments]
      ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
    final statement = [...widget.workspace.ledger]
      ..sort((a, b) => b.postedAt.compareTo(a.postedAt));

    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 16, 14, 24),
      children: [
        _PaymentHero(
          due: accountDue,
          credit: credit,
          charges: charges,
          paid: confirmed,
          pending: pendingProofs,
          onPay: () => _openProofSheet(accountDue),
        ),
        const SizedBox(height: 12),
        Container(
          decoration: tileDecoration(),
          child: Row(
            children: [
              Expanded(
                child: _PaymentMetric(label: 'Charges', value: money(charges)),
              ),
              const SizedBox(height: 48, child: VerticalDivider()),
              Expanded(
                child: _PaymentMetric(label: 'Paid', value: money(confirmed)),
              ),
              const SizedBox(height: 48, child: VerticalDivider()),
              Expanded(
                child: _PaymentMetric(
                  label: 'Pending',
                  value: money(pendingProofs),
                ),
              ),
            ],
          ),
        ),
        if (dueOrders.isNotEmpty && accountDue > 0) ...[
          const SizedBox(height: 12),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(14),
            decoration: tileDecoration(),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Open order allocation',
                  style: TextStyle(fontWeight: FontWeight.w800),
                ),
                const SizedBox(height: 8),
                ...dueOrders
                    .take(6)
                    .map(
                      (order) => Padding(
                        padding: const EdgeInsets.only(bottom: 6),
                        child: Row(
                          children: [
                            Expanded(
                              child: Text(
                                order.productName.isEmpty
                                    ? order.id
                                    : order.productName,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                            ),
                            Text(
                              money(order.balanceDue),
                              style: const TextStyle(
                                color: AppColors.primary,
                                fontWeight: FontWeight.w800,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
              ],
            ),
          ),
        ],
        const SizedBox(height: 18),
        SizedBox(
          width: double.infinity,
          child: SegmentedButton<String>(
            expandedInsets: EdgeInsets.zero,
            segments: const [
              ButtonSegment(
                value: 'proofs',
                icon: Icon(Icons.receipt_long_outlined),
                label: Text('Proofs'),
              ),
              ButtonSegment(
                value: 'statement',
                icon: Icon(Icons.account_balance_outlined),
                label: Text('Statement'),
              ),
            ],
            selected: {_view},
            onSelectionChanged: (value) => setState(() => _view = value.first),
          ),
        ),
        const SizedBox(height: 12),
        if (_view == 'proofs') ...[
          Row(
            children: [
              const Expanded(
                child: Text(
                  'Payment proofs',
                  style: TextStyle(fontSize: 17, fontWeight: FontWeight.w900),
                ),
              ),
              Text(
                '${recent.length}',
                style: const TextStyle(
                  color: AppColors.muted,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ],
          ),
          const SizedBox(height: 9),
          if (recent.isEmpty)
            const EmptyState(
              'No proofs yet',
              detail: 'Upload your transfer screenshot here.',
              icon: Icons.receipt_long_outlined,
            )
          else
            ...recent.take(12).map((payment) => PaymentTile(payment: payment)),
        ] else ...[
          Row(
            children: [
              const Expanded(
                child: Text(
                  'Account statement',
                  style: TextStyle(fontSize: 17, fontWeight: FontWeight.w900),
                ),
              ),
              Text(
                '${statement.length}',
                style: const TextStyle(
                  color: AppColors.muted,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ],
          ),
          const SizedBox(height: 9),
          if (statement.isEmpty)
            const EmptyState(
              'No statement entries yet',
              icon: Icons.account_balance_outlined,
            )
          else
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              decoration: tileDecoration(),
              child: Column(
                children: statement
                    .take(20)
                    .map((entry) => LedgerTile(entry: entry))
                    .toList(),
              ),
            ),
        ],
        if (widget.footer != null) widget.footer!,
      ],
    );
  }
}

class _PaymentHero extends StatelessWidget {
  const _PaymentHero({
    required this.due,
    required this.credit,
    required this.charges,
    required this.paid,
    required this.pending,
    required this.onPay,
  });

  final int due;
  final int credit;
  final int charges;
  final int paid;
  final int pending;
  final VoidCallback onPay;

  @override
  Widget build(BuildContext context) {
    final hasDue = due > 0;
    final hasCredit = credit > 0;
    final settled = charges <= 0 ? 0.0 : (paid / charges).clamp(0.0, 1.0);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.primaryDark,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 36,
                height: 36,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(
                  hasDue
                      ? Icons.account_balance_wallet_outlined
                      : Icons.verified_outlined,
                  color: Colors.white,
                  size: 20,
                ),
              ),
              const SizedBox(width: 10),
              Text(
                hasCredit
                    ? 'ACCOUNT CREDIT'
                    : hasDue
                    ? 'BALANCE DUE'
                    : 'ACCOUNT CLEAR',
                style: TextStyle(
                  color: Colors.white.withValues(alpha: 0.72),
                  fontSize: 10,
                  fontWeight: FontWeight.w900,
                  letterSpacing: 0,
                ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          Text(
            money(hasCredit ? credit : due),
            style: const TextStyle(
              color: Colors.white,
              fontSize: 30,
              fontWeight: FontWeight.w900,
              letterSpacing: 0,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            hasDue
                ? 'Clear this balance to keep orders moving.'
                : hasCredit
                ? 'Available credit on your account.'
                : 'Nothing is due right now.',
            style: TextStyle(
              color: Colors.white.withValues(alpha: 0.72),
              fontSize: 12,
            ),
          ),
          if (charges > 0) ...[
            const SizedBox(height: 15),
            Row(
              children: [
                Expanded(
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(99),
                    child: LinearProgressIndicator(
                      value: settled,
                      minHeight: 5,
                      backgroundColor: Colors.white.withValues(alpha: 0.14),
                      valueColor: const AlwaysStoppedAnimation<Color>(
                        Color(0xFF8DE0BE),
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: 10),
                Text(
                  '${(settled * 100).round()}% paid',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 11,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ],
            ),
          ],
          if (pending > 0) ...[
            const SizedBox(height: 12),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
              decoration: BoxDecoration(
                color: Colors.white.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(7),
              ),
              child: Row(
                children: [
                  const Icon(
                    Icons.schedule_rounded,
                    size: 16,
                    color: Colors.white,
                  ),
                  const SizedBox(width: 7),
                  Expanded(
                    child: Text(
                      '${money(pending)} proof under review',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
          const SizedBox(height: 15),
          SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              onPressed: onPay,
              style: FilledButton.styleFrom(
                backgroundColor: Colors.white,
                foregroundColor: AppColors.primaryDark,
              ),
              icon: const Icon(Icons.upload_rounded),
              label: Text(hasDue ? 'Upload payment proof' : 'Upload proof'),
            ),
          ),
        ],
      ),
    );
  }
}

class _PaymentMetric extends StatelessWidget {
  const _PaymentMetric({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label.toUpperCase(),
            style: const TextStyle(
              color: AppColors.muted,
              fontSize: 10,
              fontWeight: FontWeight.w800,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w800),
          ),
        ],
      ),
    );
  }
}

class MessagesPage extends StatefulWidget {
  const MessagesPage({
    super.key,
    required this.api,
    required this.workspace,
    required this.customerId,
    required this.onAction,
  });

  final ApiClient api;
  final Workspace workspace;
  final String customerId;
  final ActionRunner onAction;

  @override
  State<MessagesPage> createState() => _MessagesPageState();
}

class _PendingChatFile {
  const _PendingChatFile({required this.name, this.bytes, this.path});

  final Uint8List? bytes;
  final String? path;
  final String name;

  Future<Uint8List> readBytes() async {
    if (bytes != null && bytes!.isNotEmpty) return bytes!;
    final filePath = path;
    if (filePath == null || filePath.isEmpty) {
      throw const ApiException('Attachment is no longer available.');
    }
    final data = await File(filePath).readAsBytes();
    if (data.isEmpty) {
      throw const ApiException('Attachment is empty.');
    }
    return data;
  }
}

enum _ChatRowKind { date, unread, message }

class _ChatRow {
  const _ChatRow._(
    this.kind, {
    this.label = '',
    this.dayLabel = '',
    this.message,
  });

  const _ChatRow.date(String label)
    : this._(_ChatRowKind.date, label: label, dayLabel: label);

  const _ChatRow.unread({String dayLabel = ''})
    : this._(_ChatRowKind.unread, dayLabel: dayLabel);

  const _ChatRow.message(Message message, {required String dayLabel})
    : this._(_ChatRowKind.message, dayLabel: dayLabel, message: message);

  final _ChatRowKind kind;
  final String label;
  final String dayLabel;
  final Message? message;
}

class _ChatDateDivider extends StatelessWidget {
  const _ChatDateDivider({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 4),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 5),
        decoration: BoxDecoration(
          color: const Color(0xFFE8EEEB),
          borderRadius: BorderRadius.circular(999),
          border: Border.all(color: const Color(0xFFD5DFDA)),
        ),
        child: Text(
          label,
          style: const TextStyle(
            color: Color(0xFF4F635A),
            fontSize: 11,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
    );
  }
}

class _ChatUnreadDivider extends StatelessWidget {
  const _ChatUnreadDivider();

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        const Expanded(child: Divider(color: Color(0xFFB7CFC4), height: 1)),
        Container(
          margin: const EdgeInsets.symmetric(horizontal: 8),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
          decoration: BoxDecoration(
            color: const Color(0xFFE7F4EF),
            borderRadius: BorderRadius.circular(999),
            border: Border.all(color: const Color(0xFFCFE6DC)),
          ),
          child: const Text(
            'UNREAD MESSAGES',
            style: TextStyle(
              color: AppColors.primary,
              fontSize: 9,
              fontWeight: FontWeight.w800,
              letterSpacing: 0.4,
            ),
          ),
        ),
        const Expanded(child: Divider(color: Color(0xFFB7CFC4), height: 1)),
      ],
    );
  }
}

class _MessagesPageState extends State<MessagesPage> {
  final _body = TextEditingController();
  final List<_PendingChatFile> _pendingFiles = [];
  final _listKey = GlobalKey();
  final Map<int, GlobalKey> _rowKeys = {};
  String _markedReadKey = '';
  String _unreadAnchorId = '';
  int _unreadCount = 0;
  bool _sending = false;
  bool _startingCall = false;
  bool _loadingEarlier = false;
  String _historyConversationId = '';
  List<Message> _historyMessages = const [];
  PageInfo? _historyPage;
  String _stickyDate = '';
  bool _stickyVisible = false;
  int? _coveredDateIndex;
  Timer? _stickyHideTimer;
  Timer? _unreadHideTimer;

  @override
  void dispose() {
    _stickyHideTimer?.cancel();
    _unreadHideTimer?.cancel();
    _body.dispose();
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant MessagesPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    final nextId = _conversation?.id ?? '';
    if (_historyConversationId.isNotEmpty && _historyConversationId != nextId) {
      _historyConversationId = '';
      _historyMessages = const [];
      _historyPage = null;
    }
    _captureUnreadAnchor();
    _maybeMarkRead();
  }

  List<Message> _mergedMessages(Conversation? conversation) {
    if (conversation == null) return const [];
    final rows = <String, Message>{};
    for (final message in _historyMessages) {
      rows[message.id] = message;
    }
    for (final message in conversation.messages) {
      rows[message.id] = message;
    }
    final result = rows.values.toList()
      ..sort((left, right) => left.createdAt.compareTo(right.createdAt));
    return result;
  }

  PageInfo _effectiveMessagePage(Conversation conversation, int loaded) {
    final source = _historyPage ?? conversation.messagePagination;
    final total = source.total < loaded ? loaded : source.total;
    return PageInfo(loaded: loaded, total: total, hasMore: loaded < total);
  }

  Future<void> _loadEarlierMessages() async {
    final conversation = _conversation;
    if (conversation == null || _loadingEarlier) return;
    final current = _mergedMessages(conversation);
    final page = _effectiveMessagePage(conversation, current.length);
    if (!page.hasMore) return;
    setState(() => _loadingEarlier = true);
    try {
      final next = await widget.api.conversationMessages(
        conversation.id,
        page.loaded,
      );
      if (!mounted || _conversation?.id != conversation.id) return;
      setState(() {
        _historyConversationId = conversation.id;
        _historyMessages = [...current, ...next.messages];
        _historyPage = next.pagination;
      });
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(error.toString())));
      }
    } finally {
      if (mounted) setState(() => _loadingEarlier = false);
    }
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _captureUnreadAnchor();
      _maybeMarkRead();
    });
  }

  Conversation? get _conversation {
    final customerId = widget.customerId.trim();
    final rows = widget.workspace.conversations;
    if (rows.isEmpty) return null;
    if (customerId.isEmpty) return rows.first;

    for (final row in rows) {
      if (row.customerId == customerId) return row;
    }
    // Backend already scopes workspace; allow blank customerId rows for this user.
    final safe = rows
        .where((row) => row.customerId.isEmpty || row.customerId == customerId)
        .toList();
    return safe.isEmpty ? null : safe.first;
  }

  void _captureUnreadAnchor() {
    final conversation = _conversation;
    if (conversation == null) return;
    if (_unreadAnchorId == conversation.id) return;
    _unreadAnchorId = conversation.id;
    _unreadCount = conversation.unreadForCustomer;
    _scheduleUnreadHide();
  }

  void _scheduleUnreadHide() {
    _unreadHideTimer?.cancel();
    if (_unreadCount <= 0) return;
    _unreadHideTimer = Timer(const Duration(milliseconds: 1400), () {
      if (!mounted) return;
      setState(() => _unreadCount = 0);
    });
  }

  void _dismissUnreadBanner() {
    _unreadHideTimer?.cancel();
    if (_unreadCount == 0) return;
    setState(() => _unreadCount = 0);
  }

  void _maybeMarkRead() {
    final conversation = _conversation;
    if (conversation == null || conversation.unreadForCustomer <= 0) return;
    final key =
        '${conversation.id}:${conversation.lastMessageAt}:${conversation.unreadForCustomer}';
    if (_markedReadKey == key) return;
    _markedReadKey = key;
    unawaited(() async {
      try {
        await widget.api.markConversationRead(conversation.id);
      } catch (_) {
        _markedReadKey = '';
      }
    }());
  }

  List<_ChatRow> _buildChatRows(List<Message> chronological) {
    final rows = <_ChatRow>[];
    var previousDay = '';
    final unreadStart = _unreadIncomingStart(chronological, _unreadCount);
    for (var i = 0; i < chronological.length; i++) {
      final message = chronological[i];
      final day = messageDayKey(message.createdAt);
      final dayLabel = messageDayLabel(message.createdAt);
      if (day.isNotEmpty && day != previousDay) {
        rows.add(_ChatRow.date(dayLabel));
        previousDay = day;
      }
      if (i == unreadStart) {
        rows.add(_ChatRow.unread(dayLabel: dayLabel));
      }
      rows.add(_ChatRow.message(message, dayLabel: dayLabel));
    }
    return rows;
  }

  int _unreadIncomingStart(List<Message> chronological, int unreadCount) {
    if (unreadCount <= 0) return -1;
    var remaining = unreadCount;
    for (var i = chronological.length - 1; i >= 0; i--) {
      if (chronological[i].authorRole.toLowerCase() == 'customer') continue;
      remaining--;
      if (remaining == 0) return i;
    }
    return chronological.isEmpty ? -1 : 0;
  }

  Future<void> _pickAttachments() async {
    if (_pendingFiles.length >= 5) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('You can attach up to 5 files.')),
      );
      return;
    }
    final photo = await showModalBottomSheet<bool>(
      context: context,
      useSafeArea: true,
      builder: (sheetContext) => Padding(
        padding: const EdgeInsets.fromLTRB(12, 4, 12, 18),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.photo_library_outlined),
              title: const Text('Photos'),
              subtitle: const Text('Choose from your gallery'),
              onTap: () => Navigator.of(sheetContext).pop(true),
            ),
            ListTile(
              leading: const Icon(Icons.description_outlined),
              title: const Text('File'),
              subtitle: const Text('PDF, document or spreadsheet'),
              onTap: () => Navigator.of(sheetContext).pop(false),
            ),
          ],
        ),
      ),
    );
    if (photo == null || !mounted) return;
    if (photo) {
      final remaining = 5 - _pendingFiles.length;
      final photos = await ImagePicker().pickMultiImage(
        imageQuality: 82,
        maxWidth: 1800,
        maxHeight: 1800,
        limit: remaining,
        requestFullMetadata: false,
      );
      if (!mounted || photos.isEmpty) return;
      final next = <_PendingChatFile>[];
      for (final item in photos.take(remaining)) {
        final bytes = await item.readAsBytes();
        if (bytes.isEmpty || bytes.length > 10 * 1024 * 1024) continue;
        next.add(_PendingChatFile(name: item.name, bytes: bytes));
      }
      if (mounted && next.isNotEmpty) {
        setState(() => _pendingFiles.addAll(next));
      }
      return;
    }
    final picked = await FilePicker.pickFiles(
      allowMultiple: true,
      type: FileType.custom,
      allowedExtensions: const [
        'pdf',
        'txt',
        'csv',
        'doc',
        'docx',
        'xls',
        'xlsx',
        'ppt',
        'pptx',
        'zip',
      ],
    );
    if (picked.isEmpty || !mounted) return;
    final remaining = 5 - _pendingFiles.length;
    final selected = picked.take(remaining);
    final next = <_PendingChatFile>[];
    for (final file in selected) {
      try {
        final path = file.path;
        if (path != null && path.isNotEmpty) {
          final length = await File(path).length();
          if (length <= 0 || length > 10 * 1024 * 1024) continue;
          next.add(_PendingChatFile(name: file.name, path: path));
          continue;
        }
        final bytes = await file.readAsBytes();
        if (bytes.isEmpty || bytes.length > 10 * 1024 * 1024) continue;
        next.add(_PendingChatFile(name: file.name, bytes: bytes));
      } catch (_) {
        // Skip unreadable picks.
      }
    }
    if (!mounted || next.isEmpty) return;
    setState(() => _pendingFiles.addAll(next));
  }

  Future<void> _openAttachment(MessageAttachment attachment) async {
    try {
      if (attachment.isImage) {
        await _showFilePreview(
          context,
          api: widget.api,
          fileId: attachment.fileId,
          title: attachment.name.isEmpty ? 'Photo' : attachment.name,
        );
        return;
      }
      final bytes = await widget.api.fetchFileBytes(
        attachment.fileId,
        thumbnail: false,
      );
      final dir = await getTemporaryDirectory();
      final safeName = attachment.name.replaceAll(RegExp(r'[\\/:*?"<>|]'), '_');
      final path =
          '${dir.path}/${attachment.fileId}_${safeName.isEmpty ? 'attachment' : safeName}';
      final file = File(path);
      await file.writeAsBytes(bytes, flush: true);
      await OpenFilex.open(file.path);
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            error is ApiException
                ? error.message
                : 'Unable to open attachment.',
          ),
        ),
      );
    }
  }

  Future<void> _send() async {
    final text = _body.text.trim();
    if ((text.isEmpty && _pendingFiles.isEmpty) || _sending) return;
    setState(() => _sending = true);
    _dismissUnreadBanner();
    final files = List<_PendingChatFile>.from(_pendingFiles);
    try {
      await widget.onAction(() async {
        final attachmentIds = <String>[];
        for (var i = 0; i < files.length; i++) {
          final file = files[i];
          if (mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                content: Text('Uploading ${i + 1} of ${files.length}…'),
                duration: const Duration(seconds: 2),
              ),
            );
          }
          final bytes = await file.readBytes();
          if (bytes.length > 10 * 1024 * 1024) {
            throw const ApiException(
              'Each attachment must be 10 MB or smaller.',
            );
          }
          final uploaded = await widget.api.uploadFile(
            bytes: bytes,
            filename: file.name,
            ownerType: 'message',
            ownerId: _conversation?.id ?? 'new-conversation',
            customerId: widget.customerId,
          );
          attachmentIds.add(uploaded.id);
        }
        await widget.api.sendMessage(
          text,
          customerId: widget.customerId,
          attachmentIds: attachmentIds,
        );
        if (mounted) {
          _body.clear();
          setState(() => _pendingFiles.clear());
        }
      }, files.isEmpty ? 'Message sent.' : 'Message sent with attachment(s).');
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  Future<void> _startCall() async {
    if (_startingCall) return;
    setState(() => _startingCall = true);
    final conversation = _conversation;
    try {
      await widget.onAction(() async {
        await widget.api.startCall(
          conversationId: conversation?.id ?? '',
          recipientName: 'ChakuChuri Support',
        );
      }, 'Calling ChakuChuri support...');
    } finally {
      if (mounted) setState(() => _startingCall = false);
    }
  }

  void _updateStickyDate(List<_ChatRow> display) {
    final listCtx = _listKey.currentContext;
    if (listCtx == null) return;
    final listBox = listCtx.findRenderObject() as RenderBox?;
    if (listBox == null || !listBox.hasSize) return;
    final threshold = listBox.localToGlobal(Offset.zero).dy + 16;

    // Reverse chat list: pick the date marker closest to the top edge from above
    // (largest top still <= threshold). Matches web sticky behavior.
    var active = '';
    int? coveredIndex;
    var bestTop = double.negativeInfinity;

    for (var i = 0; i < display.length; i++) {
      final row = display[i];
      if (row.kind != _ChatRowKind.date || row.dayLabel.isEmpty) continue;
      final ctx = _rowKeys[i]?.currentContext;
      if (ctx == null) continue;
      final box = ctx.findRenderObject() as RenderBox?;
      if (box == null || !box.attached || !box.hasSize) continue;
      final top = box.localToGlobal(Offset.zero).dy;
      if (top <= threshold && top >= bestTop) {
        bestTop = top;
        active = row.dayLabel;
        coveredIndex = i;
      }
    }

    // If no divider has crossed the top yet, use the day of the topmost
    // visible message (or unread row) so the chip never stays stuck on "Today".
    if (active.isEmpty) {
      var nearestTop = double.infinity;
      for (var i = 0; i < display.length; i++) {
        final row = display[i];
        if (row.dayLabel.isEmpty) continue;
        if (row.kind != _ChatRowKind.message &&
            row.kind != _ChatRowKind.unread) {
          continue;
        }
        final ctx = _rowKeys[i]?.currentContext;
        if (ctx == null) continue;
        final box = ctx.findRenderObject() as RenderBox?;
        if (box == null || !box.attached || !box.hasSize) continue;
        final top = box.localToGlobal(Offset.zero).dy;
        final distance = (top - threshold).abs();
        if (distance < nearestTop) {
          nearestTop = distance;
          active = row.dayLabel;
          coveredIndex = null;
        }
      }
    }

    if (active.isEmpty ||
        (active == _stickyDate &&
            _stickyVisible &&
            coveredIndex == _coveredDateIndex)) {
      if (active.isNotEmpty) _scheduleStickyHide();
      return;
    }

    setState(() {
      _stickyDate = active;
      _stickyVisible = true;
      _coveredDateIndex = coveredIndex;
    });
    _scheduleStickyHide();
  }

  void _scheduleStickyHide() {
    _stickyHideTimer?.cancel();
    _stickyHideTimer = Timer(const Duration(milliseconds: 850), () {
      if (!mounted) return;
      setState(() {
        _stickyVisible = false;
        _coveredDateIndex = null;
      });
    });
  }

  Widget _buildMessageList(
    List<_ChatRow> display, {
    required List<Message> chronological,
    required Conversation? conversation,
  }) {
    if (_rowKeys.length > display.length) {
      _rowKeys.removeWhere((key, _) => key >= display.length);
    }
    final canLoadEarlier =
        conversation != null &&
        _effectiveMessagePage(conversation, chronological.length).hasMore;
    return Stack(
      clipBehavior: Clip.hardEdge,
      children: [
        NotificationListener<ScrollNotification>(
          onNotification: (notification) {
            if (notification is ScrollUpdateNotification ||
                notification is OverscrollNotification) {
              _updateStickyDate(display);
              if (notification is ScrollUpdateNotification &&
                  (notification.scrollDelta ?? 0).abs() > 4) {
                _dismissUnreadBanner();
              }
            }
            return false;
          },
          child: ListView.separated(
            key: _listKey,
            reverse: true,
            cacheExtent: 800,
            padding: const EdgeInsets.fromLTRB(14, 18, 14, 18),
            keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
            itemCount: display.length + (canLoadEarlier ? 1 : 0),
            separatorBuilder: (context, index) => const SizedBox(height: 8),
            itemBuilder: (context, index) {
              if (canLoadEarlier && index == display.length) {
                return Center(
                  child: TextButton.icon(
                    onPressed: _loadingEarlier ? null : _loadEarlierMessages,
                    icon: _loadingEarlier
                        ? const SizedBox.square(
                            dimension: 14,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.history, size: 17),
                    label: Text(
                      _loadingEarlier ? 'Loading...' : 'Load earlier messages',
                    ),
                  ),
                );
              }
              final row = display[index];
              final key = _rowKeys.putIfAbsent(index, GlobalKey.new);
              Widget child;
              if (row.kind == _ChatRowKind.date) {
                child = Opacity(
                  opacity: _stickyVisible && _coveredDateIndex == index ? 0 : 1,
                  child: _ChatDateDivider(label: row.label),
                );
              } else if (row.kind == _ChatRowKind.unread) {
                child = const _ChatUnreadDivider();
              } else {
                final message = row.message!;
                final mine = message.authorRole == 'Customer';
                child = MessengerBubble(
                  api: widget.api,
                  message: message,
                  mine: mine,
                  receipt: ownMessageReceipt(
                    chronological: chronological,
                    message: message,
                    mine: mine,
                    peerUnread: conversation?.unreadForAdmin ?? 0,
                  ),
                  onOpenAttachment: _openAttachment,
                );
              }
              return KeyedSubtree(key: key, child: child);
            },
          ),
        ),
        Positioned(
          top: 10,
          left: 0,
          right: 0,
          child: IgnorePointer(
            child: AnimatedOpacity(
              opacity: _stickyVisible && _stickyDate.isNotEmpty ? 1 : 0,
              duration: const Duration(milliseconds: 180),
              child: AnimatedSlide(
                offset: _stickyVisible ? Offset.zero : const Offset(0, -0.25),
                duration: const Duration(milliseconds: 180),
                child: Center(
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 5,
                    ),
                    decoration: BoxDecoration(
                      color: const Color(0xF7E8EEEB),
                      borderRadius: BorderRadius.circular(999),
                      border: Border.all(color: const Color(0xFFD5DFDA)),
                      boxShadow: const [
                        BoxShadow(
                          color: Color(0x1F0F172A),
                          blurRadius: 16,
                          offset: Offset(0, 6),
                        ),
                      ],
                    ),
                    child: Text(
                      _stickyDate,
                      style: const TextStyle(
                        color: Color(0xFF4F635A),
                        fontSize: 11,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final conversation = _conversation;
    final currentCall = firstCurrentCall(widget.workspace.calls);
    final support = _supportPresence(widget.workspace.presence);
    final messages = _mergedMessages(conversation);
    final chronological = List<Message>.from(messages)
      ..sort((a, b) => a.createdAt.compareTo(b.createdAt));
    final rows = _buildChatRows(chronological);
    final display = rows.reversed.toList();

    return Column(
      children: [
        _SupportHeader(
          support: support,
          callActive: currentCall != null,
          callStarting: _startingCall,
          onCall:
              !_startingCall &&
                  currentCall == null &&
                  isPresenceOnline(
                    support?.lastOnline ?? '',
                    onlineFlag: support?.online,
                  )
              ? _startCall
              : null,
        ),
        Expanded(
          child: conversation == null || messages.isEmpty
              ? const Center(
                  child: Padding(
                    padding: EdgeInsets.all(20),
                    child: EmptyState(
                      'Start a conversation',
                      detail:
                          'Messages from the support team will appear here.',
                      icon: Icons.forum_outlined,
                    ),
                  ),
                )
              : _buildMessageList(
                  display,
                  chronological: chronological,
                  conversation: conversation,
                ),
        ),
        if (_pendingFiles.isNotEmpty)
          Container(
            width: double.infinity,
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
            color: Colors.white,
            child: Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                for (var i = 0; i < _pendingFiles.length; i++)
                  Chip(
                    label: Text(
                      _pendingFiles[i].name,
                      overflow: TextOverflow.ellipsis,
                    ),
                    onDeleted: _sending
                        ? null
                        : () => setState(() => _pendingFiles.removeAt(i)),
                  ),
              ],
            ),
          ),
        _MessageComposer(
          controller: _body,
          sending: _sending,
          onAttach: _sending ? null : _pickAttachments,
          onSend: _sending ? null : _send,
        ),
      ],
    );
  }
}

class _SupportHeader extends StatelessWidget {
  const _SupportHeader({
    required this.support,
    required this.callActive,
    required this.callStarting,
    required this.onCall,
  });

  final UserPresence? support;
  final bool callActive;
  final bool callStarting;
  final VoidCallback? onCall;

  @override
  Widget build(BuildContext context) {
    final person = support;
    final online = isPresenceOnline(
      person?.lastOnline ?? '',
      onlineFlag: person?.online,
    );
    final subtitle = person == null
        ? 'Support desk'
        : lastSeenLabel(person.lastOnline, onlineFlag: person.online);
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 12, 10, 12),
      decoration: const BoxDecoration(
        color: Colors.white,
        border: Border(bottom: BorderSide(color: AppColors.line)),
      ),
      child: Row(
        children: [
          Stack(
            clipBehavior: Clip.none,
            children: [
              const CircleAvatar(
                radius: 22,
                backgroundColor: AppColors.surfaceSoft,
                child: Icon(
                  Icons.support_agent_rounded,
                  color: AppColors.primary,
                ),
              ),
              Positioned(
                right: -1,
                bottom: -1,
                child: Container(
                  width: 12,
                  height: 12,
                  decoration: BoxDecoration(
                    color: online ? AppColors.primary : AppColors.muted,
                    shape: BoxShape.circle,
                    border: Border.all(color: Colors.white, width: 2),
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(width: 11),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  support?.name.isNotEmpty == true
                      ? support!.name
                      : 'ChakuChuri support',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  callStarting
                      ? 'Starting call...'
                      : callActive
                      ? 'Call in progress'
                      : subtitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: callStarting || callActive
                        ? AppColors.amber
                        : AppColors.muted,
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            tooltip: callActive
                ? 'Call in progress'
                : !online
                ? 'Support is offline'
                : 'Start audio call',
            onPressed: onCall,
            icon: callStarting
                ? const SizedBox.square(
                    dimension: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(
                    Icons.call_rounded,
                    color: callActive
                        ? AppColors.amber
                        : online
                        ? AppColors.primary
                        : AppColors.muted,
                  ),
          ),
        ],
      ),
    );
  }
}

class _MessageComposer extends StatelessWidget {
  const _MessageComposer({
    required this.controller,
    required this.onSend,
    this.onAttach,
    this.sending = false,
  });

  final TextEditingController controller;
  final VoidCallback? onSend;
  final VoidCallback? onAttach;
  final bool sending;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Container(
        padding: const EdgeInsets.fromLTRB(6, 9, 10, 9),
        decoration: const BoxDecoration(
          color: Colors.white,
          border: Border(top: BorderSide(color: AppColors.line)),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            IconButton(
              tooltip: 'Attach photo or PDF',
              onPressed: onAttach,
              icon: const Icon(Icons.attach_file_rounded),
            ),
            Expanded(
              child: TextField(
                controller: controller,
                enabled: !sending,
                minLines: 1,
                maxLines: 4,
                textCapitalization: TextCapitalization.sentences,
                decoration: const InputDecoration(
                  hintText: 'Message support',
                  contentPadding: EdgeInsets.symmetric(
                    horizontal: 14,
                    vertical: 12,
                  ),
                ),
              ),
            ),
            const SizedBox(width: 8),
            SizedBox.square(
              dimension: 48,
              child: FilledButton(
                onPressed: onSend,
                style: FilledButton.styleFrom(padding: EdgeInsets.zero),
                child: sending
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : const Icon(Icons.send_rounded, size: 20),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

UserPresence? _supportPresence(List<UserPresence> presence) {
  final staff = presence
      .where((person) => person.role.toLowerCase() != 'customer')
      .toList();
  if (staff.isEmpty) return presence.isEmpty ? null : presence.first;
  staff.sort((a, b) {
    final aOnline = a.online ? 1 : 0;
    final bOnline = b.online ? 1 : 0;
    if (aOnline != bOnline) return bOnline.compareTo(aOnline);
    return b.lastOnline.compareTo(a.lastOnline);
  });
  return staff.first;
}

class QuoteTile extends StatelessWidget {
  const QuoteTile({
    super.key,
    required this.api,
    required this.quote,
    this.onAccept,
    this.onReject,
    this.onOpenOrder,
  });

  final ApiClient api;
  final Quotation quote;
  final VoidCallback? onAccept;
  final VoidCallback? onReject;
  final VoidCallback? onOpenOrder;

  @override
  Widget build(BuildContext context) {
    final hasPrice = quote.totalAmount > 0;
    final inReview = quote.status == 'Requested';
    final unitPrice = hasPrice && quote.quantity > 0
        ? (quote.totalAmount / quote.quantity).round()
        : 0;
    final photoIds = quote.resolvedImageFileIds;
    Update? latestUpdate;
    for (final update in quote.history) {
      if (latestUpdate == null ||
          update.createdAt.compareTo(latestUpdate.createdAt) > 0) {
        latestUpdate = update;
      }
    }
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      decoration: tileDecoration(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (photoIds.isNotEmpty)
                  _ProductPhotoThumb(
                    api: api,
                    fileId: photoIds.first,
                    name: quote.resolvedImageNames.isNotEmpty
                        ? quote.resolvedImageNames.first
                        : quote.productName,
                    size: 72,
                  )
                else
                  Container(
                    width: 72,
                    height: 72,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: AppColors.surfaceSoft,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: const Icon(
                      Icons.request_quote_outlined,
                      color: AppColors.primary,
                      size: 26,
                    ),
                  ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              quote.id,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(
                                color: AppColors.muted,
                                fontSize: 10,
                                fontWeight: FontWeight.w800,
                              ),
                            ),
                          ),
                          StatusPill(inReview ? 'In review' : quote.status),
                        ],
                      ),
                      const SizedBox(height: 5),
                      Text(
                        quote.productName.isEmpty ||
                                quote.productName == 'Product consultation'
                            ? 'Product request'
                            : quote.productName,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: AppColors.ink,
                          fontSize: 16,
                          height: 1.15,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                      const SizedBox(height: 6),
                      Row(
                        children: [
                          const Icon(
                            Icons.inventory_2_outlined,
                            size: 14,
                            color: AppColors.muted,
                          ),
                          const SizedBox(width: 5),
                          Text(
                            '${quote.quantity} pieces',
                            style: const TextStyle(
                              color: AppColors.muted,
                              fontSize: 12,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          if (photoIds.length > 1) ...[
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
              child: _ProductPhotoRow(
                api: api,
                fileIds: photoIds,
                names: quote.resolvedImageNames,
                size: 56,
              ),
            ),
          ],
          if (hasPrice)
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 11),
              decoration: const BoxDecoration(
                color: Color(0xFFF6F9F7),
                border: Border(
                  top: BorderSide(color: AppColors.line),
                  bottom: BorderSide(color: AppColors.line),
                ),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: _PlainFact(
                      label: 'PER UNIT',
                      value: money(unitPrice),
                    ),
                  ),
                  Expanded(
                    child: _PlainFact(
                      label: 'TOTAL',
                      value: money(quote.totalAmount),
                    ),
                  ),
                  Expanded(
                    child: _PlainFact(
                      label: 'READY',
                      value: quote.expectedDate.isEmpty
                          ? 'Pending'
                          : shortDate(quote.expectedDate),
                    ),
                  ),
                ],
              ),
            )
          else if (inReview)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 11),
              decoration: const BoxDecoration(
                color: Color(0xFFF1F6F9),
                border: Border(
                  top: BorderSide(color: AppColors.line),
                  bottom: BorderSide(color: AppColors.line),
                ),
              ),
              child: const Row(
                children: [
                  Icon(Icons.schedule_rounded, size: 18, color: AppColors.blue),
                  SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Our team is reviewing your request.',
                      style: TextStyle(
                        color: AppColors.blue,
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          if (latestUpdate != null)
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 10, 12, 0),
              child: Row(
                children: [
                  const Icon(
                    Icons.history_rounded,
                    size: 15,
                    color: AppColors.muted,
                  ),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      latestUpdate.detail,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: AppColors.muted,
                        fontSize: 11,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Text(
                    shortDate(latestUpdate.createdAt),
                    style: const TextStyle(
                      color: AppColors.muted,
                      fontSize: 10,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ),
            ),
          if (onAccept != null || onReject != null || onOpenOrder != null) ...[
            const SizedBox(height: 12),
            if (onOpenOrder != null)
              Padding(
                padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
                child: SizedBox(
                  width: double.infinity,
                  child: OutlinedButton.icon(
                    onPressed: onOpenOrder,
                    icon: const Icon(Icons.assignment_outlined),
                    label: const Text('View order'),
                  ),
                ),
              ),
            if (onAccept != null)
              Padding(
                padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
                child: Row(
                  children: [
                    Expanded(
                      child: FilledButton.icon(
                        onPressed: onAccept,
                        icon: const Icon(Icons.check_rounded),
                        label: const Text('Accept quote'),
                      ),
                    ),
                    if (onReject != null) ...[
                      const SizedBox(width: 8),
                      IconButton.outlined(
                        tooltip: 'Reject quotation',
                        onPressed: onReject,
                        style: IconButton.styleFrom(
                          foregroundColor: AppColors.red,
                        ),
                        icon: const Icon(Icons.close_rounded),
                      ),
                    ],
                  ],
                ),
              ),
          ],
          if (onAccept == null && onOpenOrder == null)
            const SizedBox(height: 12),
        ],
      ),
    );
  }
}

class _ModernOrderCard extends StatelessWidget {
  const _ModernOrderCard({
    required this.api,
    required this.order,
    required this.quotes,
  });

  final ApiClient api;
  final ManufacturingOrder order;
  final List<Quotation> quotes;

  ({List<String> ids, List<String> names}) get _photos {
    var ids = order.resolvedImageFileIds;
    var names = order.resolvedImageNames;
    if (ids.isEmpty && order.quotationId.isNotEmpty) {
      for (final quote in quotes) {
        if (quote.id != order.quotationId) continue;
        ids = quote.resolvedImageFileIds;
        names = quote.resolvedImageNames;
        break;
      }
    }
    return (ids: ids, names: names);
  }

  void _open(BuildContext context) {
    final photos = _photos;
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder: (sheetContext) => FractionallySizedBox(
        heightFactor: 0.9,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(18, 4, 18, 28),
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (photos.ids.isNotEmpty) ...[
                  _ProductPhotoThumb(
                    api: api,
                    fileId: photos.ids.first,
                    name: photos.names.isEmpty
                        ? order.productName
                        : photos.names.first,
                    size: 68,
                  ),
                  const SizedBox(width: 12),
                ],
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        manufacturingJobCode(order),
                        style: const TextStyle(
                          color: AppColors.primary,
                          fontSize: 11,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        order.productName,
                        style: Theme.of(sheetContext).textTheme.titleLarge,
                      ),
                      const SizedBox(height: 7),
                      StatusPill(order.status),
                    ],
                  ),
                ),
              ],
            ),
            if (photos.ids.length > 1) ...[
              const SizedBox(height: 16),
              _ProductPhotoRow(
                api: api,
                fileIds: photos.ids,
                names: photos.names,
                size: 62,
              ),
            ],
            const SizedBox(height: 20),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 12),
              decoration: BoxDecoration(
                color: AppColors.surfaceSoft,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: _PlainFact(
                      label: 'QUANTITY',
                      value: '${order.quantity} pcs',
                    ),
                  ),
                  Expanded(
                    child: _PlainFact(
                      label: 'EXPECTED',
                      value: shortDate(order.expectedDate),
                    ),
                  ),
                  Expanded(
                    child: _PlainFact(
                      label: 'BALANCE',
                      value: money(order.balanceDue),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            Row(
              children: [
                Expanded(
                  child: Text(
                    order.currentStage.isEmpty
                        ? 'Production update pending'
                        : order.currentStage,
                    style: const TextStyle(fontWeight: FontWeight.w900),
                  ),
                ),
                Text(
                  '${order.progress}%',
                  style: const TextStyle(
                    color: AppColors.primary,
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            ProgressLine(value: order.progress),
            const SizedBox(height: 24),
            const _DashSectionTitle(title: 'Production timeline'),
            const SizedBox(height: 10),
            if (order.stages.isEmpty)
              const EmptyState(
                'Timeline pending',
                detail: 'Production stages will appear here.',
                icon: Icons.route_outlined,
              )
            else
              ...order.stages.map((stage) => StageLine(stage: stage)),
            if (order.history.isNotEmpty) ...[
              const SizedBox(height: 20),
              const _DashSectionTitle(title: 'Latest update'),
              const SizedBox(height: 8),
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: AppColors.surfaceSoft,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      order.history.last.label,
                      style: const TextStyle(fontWeight: FontWeight.w900),
                    ),
                    if (order.history.last.detail.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text(
                        order.history.last.detail,
                        style: const TextStyle(color: AppColors.muted),
                      ),
                    ],
                    const SizedBox(height: 6),
                    Text(
                      shortDate(order.history.last.createdAt),
                      style: const TextStyle(
                        color: AppColors.muted,
                        fontSize: 11,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final photos = _photos;
    final stage = order.currentStage.isEmpty
        ? order.status
        : order.currentStage;
    return Material(
      color: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(8),
        side: const BorderSide(color: AppColors.line),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => _open(context),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (photos.ids.isNotEmpty) ...[
                    _ProductPhotoThumb(
                      api: api,
                      fileId: photos.ids.first,
                      name: photos.names.isEmpty
                          ? order.productName
                          : photos.names.first,
                      size: 62,
                    ),
                    const SizedBox(width: 11),
                  ],
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          manufacturingJobCode(order),
                          style: const TextStyle(
                            color: AppColors.primary,
                            fontSize: 10,
                            fontWeight: FontWeight.w900,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          order.productName,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 17,
                            height: 1.15,
                            fontWeight: FontWeight.w900,
                          ),
                        ),
                        const SizedBox(height: 5),
                        Text(
                          '${order.quantity} pieces',
                          style: const TextStyle(
                            color: AppColors.muted,
                            fontSize: 11,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const Icon(
                    Icons.chevron_right_rounded,
                    color: AppColors.muted,
                  ),
                ],
              ),
              const SizedBox(height: 13),
              Row(
                children: [
                  Flexible(child: StatusPill(stage)),
                  const SizedBox(width: 9),
                  Expanded(child: ProgressLine(value: order.progress)),
                  const SizedBox(width: 9),
                  Text(
                    '${order.progress}%',
                    style: const TextStyle(
                      color: AppColors.primary,
                      fontSize: 12,
                      fontWeight: FontWeight.w900,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  const Icon(
                    Icons.event_outlined,
                    size: 15,
                    color: AppColors.muted,
                  ),
                  const SizedBox(width: 5),
                  Expanded(
                    child: Text(
                      'Expected ${shortDate(order.expectedDate)}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: AppColors.muted,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                  Text(
                    order.balanceDue > 0
                        ? '${money(order.balanceDue)} due'
                        : 'Account clear',
                    style: TextStyle(
                      color: order.balanceDue > 0
                          ? AppColors.amber
                          : AppColors.primary,
                      fontSize: 11,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class OrderCard extends StatelessWidget {
  const OrderCard({
    super.key,
    required this.api,
    required this.order,
    this.quotes = const [],
    this.compact = false,
  });

  final ApiClient api;
  final ManufacturingOrder order;
  final List<Quotation> quotes;
  final bool compact;

  @override
  Widget build(BuildContext context) {
    var photoIds = order.resolvedImageFileIds;
    var photoNames = order.resolvedImageNames;
    if (photoIds.isEmpty && order.quotationId.isNotEmpty) {
      for (final quote in quotes) {
        if (quote.id == order.quotationId) {
          photoIds = quote.resolvedImageFileIds;
          photoNames = quote.resolvedImageNames;
          break;
        }
      }
    }
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(15),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (photoIds.isNotEmpty) ...[
                  _ProductPhotoThumb(
                    api: api,
                    fileId: photoIds.first,
                    name: photoNames.isNotEmpty
                        ? photoNames.first
                        : order.productName,
                    size: 54,
                  ),
                  const SizedBox(width: 11),
                ],
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        manufacturingJobCode(order),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: AppColors.primary,
                          fontSize: 11,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                      if (manufacturingLinkedQuoteCode(order).isNotEmpty) ...[
                        const SizedBox(height: 2),
                        Text(
                          'Quote ${manufacturingLinkedQuoteCode(order)}',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: AppColors.muted,
                            fontSize: 11,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ],
                      const SizedBox(height: 4),
                      Text(
                        order.productName,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          fontSize: 19,
                          height: 1.15,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 10),
                StatusPill(order.status),
              ],
            ),
            if (photoIds.length > 1) ...[
              const SizedBox(height: 12),
              _ProductPhotoRow(
                api: api,
                fileIds: photoIds,
                names: photoNames,
                size: 56,
              ),
            ],
            const SizedBox(height: 14),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              decoration: BoxDecoration(
                color: AppColors.surfaceSoft.withValues(alpha: 0.65),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: _PlainFact(
                      label: 'EXPECTED',
                      value: shortDate(order.expectedDate),
                    ),
                  ),
                  Container(width: 1, height: 30, color: AppColors.line),
                  Expanded(
                    child: _PlainFact(
                      label: 'BALANCE',
                      value: money(order.balanceDue),
                    ),
                  ),
                  Container(width: 1, height: 30, color: AppColors.line),
                  Expanded(
                    child: _PlainFact(
                      label: 'QUANTITY',
                      value: '${order.quantity} pcs',
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 15),
            Row(
              children: [
                Expanded(
                  child: Text(
                    order.currentStage.isEmpty
                        ? 'Production update pending'
                        : order.currentStage,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
                Text(
                  '${order.progress}%',
                  style: const TextStyle(
                    color: AppColors.primary,
                    fontSize: 13,
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            ProgressLine(value: order.progress),
            if (!compact) ...[
              const SizedBox(height: 7),
              Theme(
                data: Theme.of(context)
                    .copyWith(dividerColor: Colors.transparent),
                child: ExpansionTile(
                  tilePadding: EdgeInsets.zero,
                  childrenPadding: const EdgeInsets.only(bottom: 2),
                  leading: const Icon(
                    Icons.route_outlined,
                    color: AppColors.blue,
                    size: 20,
                  ),
                  title: const Text(
                    'Timeline & updates',
                    style: TextStyle(fontSize: 13, fontWeight: FontWeight.w800),
                  ),
                  children: [
                    Column(
                      children: order.stages
                          .map((stage) => StageLine(stage: stage))
                          .toList(),
                    ),
                    if (order.history.isNotEmpty) ...[
                      const Divider(height: 18),
                      Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Icon(
                            Icons.notifications_none_rounded,
                            size: 18,
                            color: AppColors.muted,
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  order.history.last.detail,
                                  style: const TextStyle(
                                    fontSize: 12,
                                    fontWeight: FontWeight.w700,
                                  ),
                                ),
                                const SizedBox(height: 3),
                                Text(
                                  shortDate(order.history.last.createdAt),
                                  style: const TextStyle(
                                    color: AppColors.muted,
                                    fontSize: 10,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _PlainFact extends StatelessWidget {
  const _PlainFact({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: const TextStyle(
              color: AppColors.muted,
              fontSize: 8,
              fontWeight: FontWeight.w900,
            ),
          ),
          const SizedBox(height: 4),
          FittedBox(
            fit: BoxFit.scaleDown,
            alignment: Alignment.centerLeft,
            child: Text(
              value,
              maxLines: 1,
              style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w900),
            ),
          ),
        ],
      ),
    );
  }
}

class StageLine extends StatelessWidget {
  const StageLine({super.key, required this.stage});

  final Stage stage;

  @override
  Widget build(BuildContext context) {
    final done =
        stage.status.toLowerCase() == 'done' ||
        stage.status.toLowerCase() == 'completed';
    final active =
        stage.status.toLowerCase() == 'active' ||
        stage.status.toLowerCase() == 'in progress';
    final cancelled =
        stage.status.toLowerCase() == 'cancelled' ||
        stage.status.toLowerCase() == 'canceled';
    final color = done
        ? AppColors.green
        : active
        ? AppColors.orange
        : cancelled
        ? Colors.blueGrey.shade300
        : Colors.blueGrey;
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          Icon(
            done
                ? Icons.check_circle_rounded
                : active
                ? Icons.radio_button_checked_rounded
                : cancelled
                ? Icons.cancel_outlined
                : Icons.radio_button_unchecked_rounded,
            color: color,
            size: 18,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              stage.name,
              style: const TextStyle(fontWeight: FontWeight.w700),
            ),
          ),
          Text(
            stage.date.isEmpty ? stage.status : shortDate(stage.date),
            style: const TextStyle(color: Colors.blueGrey, fontSize: 12),
          ),
        ],
      ),
    );
  }
}

class RateTile extends StatelessWidget {
  const RateTile({super.key, required this.rate});

  final RateSheet rate;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      dense: true,
      contentPadding: EdgeInsets.zero,
      leading: Container(
        width: 36,
        height: 36,
        alignment: Alignment.center,
        decoration: BoxDecoration(
          color: AppColors.blue.withValues(alpha: 0.08),
          borderRadius: BorderRadius.circular(7),
        ),
        child: const Icon(
          Icons.local_shipping_outlined,
          size: 18,
          color: AppColors.blue,
        ),
      ),
      title: Text(
        '${rate.courier} ${rate.service}',
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(fontWeight: FontWeight.w900),
      ),
      subtitle: Text('${rate.zone}  /  ${rate.weight}'),
      trailing: Text(
        money(rate.price),
        style: const TextStyle(fontWeight: FontWeight.w900),
      ),
    );
  }
}

class ShipmentTile extends StatelessWidget {
  const ShipmentTile({super.key, required this.shipment});

  final ShippingRequest shipment;

  @override
  Widget build(BuildContext context) {
    final tracking = shipment.tracking.trim();
    return Container(
      margin: const EdgeInsets.only(bottom: 9),
      padding: const EdgeInsets.all(13),
      decoration: tileDecoration(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 40,
                height: 40,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: AppColors.blue.withValues(alpha: 0.09),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: const Icon(
                  Icons.local_shipping_outlined,
                  color: AppColors.blue,
                  size: 20,
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      shipment.destination,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 16,
                        height: 1.15,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    const SizedBox(height: 3),
                    Text(
                      shipment.id,
                      style: const TextStyle(
                        color: AppColors.muted,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              StatusPill(shipment.status),
            ],
          ),
          const SizedBox(height: 12),
          Container(height: 1, color: AppColors.line),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: Text(
                  '${shipment.courier}  /  ${shipment.service}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                shipment.quotedAmount > 0
                    ? money(shipment.quotedAmount)
                    : 'Rate pending',
                style: TextStyle(
                  color: shipment.quotedAmount > 0
                      ? AppColors.ink
                      : AppColors.amber,
                  fontSize: 12,
                  fontWeight: FontWeight.w900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 5),
          Text(
            '${shipment.zone.isEmpty ? 'Zone pending' : shipment.zone}  /  ${shipment.weight}',
            style: const TextStyle(color: AppColors.muted, fontSize: 11),
          ),
          if (tracking.isNotEmpty) ...[
            const SizedBox(height: 9),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
              decoration: BoxDecoration(
                color: AppColors.surfaceSoft,
                borderRadius: BorderRadius.circular(6),
              ),
              child: Row(
                children: [
                  const Icon(
                    Icons.pin_drop_outlined,
                    size: 16,
                    color: AppColors.primary,
                  ),
                  const SizedBox(width: 7),
                  Expanded(
                    child: Text(
                      'Tracking $tracking',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class PaymentTile extends StatelessWidget {
  const PaymentTile({super.key, required this.payment});

  final Payment payment;

  @override
  Widget build(BuildContext context) {
    final confirmed = payment.status == 'Confirmed';
    final rejected = payment.status == 'Rejected';
    final color = confirmed
        ? AppColors.primary
        : rejected
        ? AppColors.red
        : AppColors.amber;
    final icon = confirmed
        ? Icons.task_alt_rounded
        : rejected
        ? Icons.error_outline_rounded
        : Icons.schedule_rounded;
    final status = confirmed
        ? 'Confirmed'
        : rejected
        ? 'Needs attention'
        : 'Under review';
    final detail = confirmed
        ? 'Applied to your account'
        : rejected
        ? 'Please upload a new proof'
        : 'We are checking this payment';
    return Container(
      margin: const EdgeInsets.only(bottom: 9),
      padding: const EdgeInsets.all(12),
      decoration: tileDecoration(),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 40,
            height: 40,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.09),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(icon, color: color, size: 20),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  payment.type,
                  style: const TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 3),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        money(payment.amount),
                        style: const TextStyle(
                          fontSize: 17,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                    ),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 8,
                        vertical: 4,
                      ),
                      decoration: BoxDecoration(
                        color: color.withValues(alpha: 0.09),
                        borderRadius: BorderRadius.circular(99),
                      ),
                      child: Text(
                        status,
                        style: TextStyle(
                          color: color,
                          fontSize: 10,
                          fontWeight: FontWeight.w800,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  '$detail  /  ${shortDate(payment.createdAt)}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(color: AppColors.muted, fontSize: 10),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class LedgerTile extends StatelessWidget {
  const LedgerTile({super.key, required this.entry});

  final LedgerEntry entry;

  @override
  Widget build(BuildContext context) {
    final debit = entry.debit > 0;
    final amount = debit ? entry.debit : entry.credit;
    final color = debit ? AppColors.red : AppColors.primary;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Container(
            width: 32,
            height: 32,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(7),
            ),
            child: Icon(
              debit ? Icons.north_east_rounded : Icons.south_west_rounded,
              color: color,
              size: 16,
            ),
          ),
          const SizedBox(width: 9),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  entry.note,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  shortDate(entry.postedAt),
                  style: const TextStyle(color: AppColors.muted, fontSize: 10),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Text(
            '${debit ? '-' : '+'}${money(amount)}',
            style: TextStyle(
              color: color,
              fontSize: 12,
              fontWeight: FontWeight.w900,
            ),
          ),
        ],
      ),
    );
  }
}

class MiniFact extends StatelessWidget {
  const MiniFact({super.key, required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: AppColors.paper,
        border: Border.all(color: AppColors.line),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            label.toUpperCase(),
            style: const TextStyle(
              fontSize: 9,
              color: AppColors.muted,
              fontWeight: FontWeight.w900,
            ),
          ),
          const SizedBox(height: 3),
          Text(
            value,
            style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w900),
          ),
        ],
      ),
    );
  }
}

class CallStatusPanel extends StatelessWidget {
  const CallStatusPanel({super.key, required this.call});

  final CallRequest call;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFFEFF6FF),
        border: Border.all(color: const Color(0xFFBFDBFE)),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  call.subject,
                  style: const TextStyle(fontWeight: FontWeight.w900),
                ),
              ),
              StatusPill(call.status),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            call.status.isEmpty
                ? call.subject
                : '${call.status} · ${call.subject}',
            style: const TextStyle(
              color: AppColors.navy,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ),
    );
  }
}

class _NoticeBar extends StatelessWidget {
  const _NoticeBar(this.message);

  final String message;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      color: const Color(0xFFEFF6FF),
      child: Text(
        message,
        style: const TextStyle(
          color: AppColors.navy,
          fontWeight: FontWeight.w800,
        ),
      ),
    );
  }
}

BoxDecoration tileDecoration({double radius = 8}) {
  return BoxDecoration(
    color: Colors.white,
    border: Border.all(color: AppColors.line),
    borderRadius: BorderRadius.circular(radius),
  );
}

Product? selectedProduct(List<Product> products, String id) {
  for (final product in products) {
    if (product.id == id) return product;
  }
  return null;
}

CallRequest? firstCurrentCall(List<CallRequest> calls) {
  for (final call in calls) {
    if (call.status == 'Ringing' || call.status == 'In call') return call;
  }
  return null;
}

bool isClosedStatus(String value) {
  final normalized = value.toLowerCase();
  return normalized.contains('complete') ||
      normalized.contains('cancel') ||
      normalized.contains('reject') ||
      normalized.contains('delivered');
}

String _orderLatestActivity(ManufacturingOrder order) {
  var latest = '';
  for (final update in order.history) {
    if (update.createdAt.compareTo(latest) > 0) latest = update.createdAt;
  }
  return latest.isEmpty ? order.id : latest;
}

String _quoteLatestActivity(Quotation quote) {
  var latest = '';
  for (final update in quote.history) {
    if (update.createdAt.compareTo(latest) > 0) latest = update.createdAt;
  }
  return latest.isEmpty ? quote.id : latest;
}
