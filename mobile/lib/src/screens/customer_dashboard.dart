part of 'customer_shell.dart';

class DashboardPage extends StatelessWidget {
  const DashboardPage({
    super.key,
    required this.api,
    required this.workspace,
    required this.userName,
    required this.onNavigate,
    required this.onAction,
    this.date,
  });

  final ApiClient api;
  final Workspace workspace;
  final String userName;
  final ValueChanged<String> onNavigate;
  final ActionRunner onAction;
  final DateTime? date;

  @override
  Widget build(BuildContext context) {
    final orders =
        workspace.manufacturing
            .where((row) => !isClosedStatus(row.status))
            .toList()
          ..sort(
            (a, b) =>
                _orderLatestActivity(b).compareTo(_orderLatestActivity(a)),
          );
    // Only quotes the customer can Proceed / Reject (priced).
    final actionableQuotes =
        workspace.quotations.where((row) => row.status == 'Priced').toList()
          ..sort(
            (a, b) =>
                _quoteLatestActivity(b).compareTo(_quoteLatestActivity(a)),
          );
    final shipments =
        workspace.shipping.where((row) => !isClosedStatus(row.status)).toList()
          ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
    final position = AccountPosition(workspace);
    final due = position.due;
    final orderCount = workspace.metrics.containsKey('activeOrders')
        ? readInt(workspace.metrics, 'activeOrders')
        : orders.length;
    final quotePending = workspace.metrics.containsKey('pendingQuotes')
        ? readInt(workspace.metrics, 'pendingQuotes')
        : workspace.quotations
              .where((q) => q.status == 'Requested' || q.status == 'Priced')
              .length;
    final featured = workspace.featuredProducts
        .where((row) => row.active)
        .toList();
    final notices = workspace.notices.where((row) => row.active).toList();
    final now = date ?? DateTime.now();
    final greeting = now.hour < 12
        ? 'Good morning'
        : now.hour < 17
        ? 'Good afternoon'
        : 'Good evening';
    final name = userName.trim().split(RegExp(r'\s+')).first;
    final greetingLine = name.isEmpty ? greeting : '$greeting, $name';

    return Stack(
      children: [
        const Positioned.fill(child: _HomeAtmosphere()),
        ListView(
          key: const PageStorageKey('customer-dashboard'),
          padding: EdgeInsets.zero,
          children: [
            _CommandHero(
              greeting: greetingLine,
              due: position.credit > 0 ? -position.credit : due,
              orderCount: orderCount,
              quoteCount: quotePending,
              shipCount: shipments.length,
              onPay: () => onNavigate('Payments'),
              onOrders: () => onNavigate('Orders'),
              onQuotes: () => onNavigate('Get Quote'),
              onShipping: () => onNavigate('Shipping'),
            ),
            if (notices.isNotEmpty)
              _HomeNotices(notices: notices.take(5).toList()),
            Padding(
              padding: const EdgeInsets.fromLTRB(18, 14, 18, 28),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _HomeSectionHeader(
                    title: orders.isEmpty
                        ? 'Open orders'
                        : 'Open orders \u00B7 ${orders.length}',
                    actionLabel: 'See all',
                    onAction: () => onNavigate('Orders'),
                  ),
                  const SizedBox(height: 10),
                  if (orders.isEmpty)
                    _HomeEmptyOrders(onQuote: () => onNavigate('Get Quote'))
                  else
                    _OrdersCarousel(
                      api: api,
                      orders: orders.take(20).toList(),
                      quotations: workspace.quotations,
                      onOpen: (order) => _ModernOrderCard(
                        api: api,
                        order: order,
                        quotes: workspace.quotations,
                      )._open(context),
                    ),
                  if (actionableQuotes.isNotEmpty) ...[
                    const SizedBox(height: 22),
                    _HomeSectionHeader(
                      title: actionableQuotes.length == 1
                          ? 'Quote ready'
                          : 'Quotes ready \u00B7 ${actionableQuotes.length}',
                      actionLabel: 'See all',
                      onAction: () => onNavigate('Get Quote'),
                    ),
                    const SizedBox(height: 10),
                    _QuotesActionCarousel(
                      api: api,
                      quotes: actionableQuotes.take(12).toList(),
                      onAction: onAction,
                    ),
                  ],
                  if (featured.isNotEmpty) ...[
                    const SizedBox(height: 22),
                    _FeaturedProductsSlider(
                      api: api,
                      items: featured.take(20).toList(),
                      onOpenCatalog: () => onNavigate('Products'),
                    ),
                  ],
                  if (shipments.isNotEmpty) ...[
                    const SizedBox(height: 22),
                    _HomeSectionHeader(
                      title: 'Shipments',
                      actionLabel: 'See all',
                      onAction: () => onNavigate('Shipping'),
                    ),
                    const SizedBox(height: 10),
                    for (final shipment in shipments.take(2))
                      Padding(
                        padding: const EdgeInsets.only(bottom: 10),
                        child: _DashboardShipment(
                          shipment: shipment,
                          onTap: () => showModalBottomSheet<void>(
                            context: context,
                            isScrollControlled: true,
                            useSafeArea: true,
                            builder: (_) => SingleChildScrollView(
                              padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
                              child: ShipmentTile(shipment: shipment),
                            ),
                          ),
                        ),
                      ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _HomeAtmosphere extends StatelessWidget {
  const _HomeAtmosphere();

  @override
  Widget build(BuildContext context) {
    return const DecoratedBox(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            Color(0xFFE8F1EE),
            Color(0xFFF5F8F7),
            Color(0xFFDDEAE5),
            Color(0xFFF3F7F5),
          ],
          stops: [0.0, 0.35, 0.7, 1.0],
        ),
      ),
      child: Stack(
        children: [
          Positioned(
            top: -90,
            right: -50,
            child: _SoftBlob(size: 260, color: Color(0x330B6B53)),
          ),
          Positioned(
            top: 220,
            left: -80,
            child: _SoftBlob(size: 220, color: Color(0x28216483)),
          ),
          Positioned(
            bottom: 80,
            right: -30,
            child: _SoftBlob(size: 180, color: Color(0x245ECF9A)),
          ),
        ],
      ),
    );
  }
}

class _SoftBlob extends StatelessWidget {
  const _SoftBlob({required this.size, required this.color});

  final double size;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return ImageFiltered(
      imageFilter: ImageFilter.blur(sigmaX: 28, sigmaY: 28),
      child: Container(
        width: size,
        height: size,
        decoration: BoxDecoration(shape: BoxShape.circle, color: color),
      ),
    );
  }
}

class _CommandHero extends StatelessWidget {
  const _CommandHero({
    required this.greeting,
    required this.due,
    required this.orderCount,
    required this.quoteCount,
    required this.shipCount,
    required this.onPay,
    required this.onOrders,
    required this.onQuotes,
    required this.onShipping,
  });

  final String greeting;
  final int due;
  final int orderCount;
  final int quoteCount;
  final int shipCount;
  final VoidCallback onPay;
  final VoidCallback onOrders;
  final VoidCallback onQuotes;
  final VoidCallback onShipping;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(18, 14, 18, 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            greeting,
            style: const TextStyle(
              color: AppColors.muted,
              fontSize: 13,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 10),
          _FrostBalanceCapsule(due: due, onPay: onPay),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: _SummaryGauge(
                  label: 'Orders',
                  value: orderCount,
                  progress: (orderCount / 12).clamp(0.08, 1),
                  color: const Color(0xFF5ECF9A),
                  onTap: onOrders,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: _SummaryGauge(
                  label: 'Quotes',
                  value: quoteCount,
                  progress: (quoteCount / 8).clamp(0.08, 1),
                  color: const Color(0xFF6BA8C9),
                  onTap: onQuotes,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: _SummaryGauge(
                  label: 'Shipping',
                  value: shipCount,
                  progress: (shipCount / 6).clamp(0.08, 1),
                  color: const Color(0xFFB7C978),
                  onTap: onShipping,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _TealCard extends StatelessWidget {
  const _TealCard({
    required this.child,
    this.padding,
    this.onTap,
    this.radius = 16,
  });

  final Widget child;
  final EdgeInsetsGeometry? padding;
  final VoidCallback? onTap;
  final double radius;

  @override
  Widget build(BuildContext context) {
    final body = ClipRRect(
      borderRadius: BorderRadius.circular(radius),
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 10, sigmaY: 10),
        child: DecoratedBox(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(radius),
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [
                const Color(0xFF0B6B53).withValues(alpha: 0.82),
                const Color(0xFF0A5C48).withValues(alpha: 0.78),
                const Color(0xFF1A5A6E).withValues(alpha: 0.8),
              ],
            ),
            border: Border.all(color: Colors.white.withValues(alpha: 0.38)),
            boxShadow: [
              BoxShadow(
                color: const Color(0xFF0B6B53).withValues(alpha: 0.16),
                blurRadius: 18,
                offset: const Offset(0, 8),
              ),
            ],
          ),
          child: padding == null
              ? child
              : Padding(padding: padding!, child: child),
        ),
      ),
    );
    if (onTap == null) return body;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(radius),
        child: body,
      ),
    );
  }
}

class _FrostBalanceCapsule extends StatelessWidget {
  const _FrostBalanceCapsule({required this.due, required this.onPay});

  final int due;
  final VoidCallback onPay;

  @override
  Widget build(BuildContext context) {
    return _TealCard(
      radius: 16,
      padding: const EdgeInsets.fromLTRB(16, 14, 12, 14),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  due < 0 ? 'ACCOUNT CREDIT' : due > 0 ? 'BALANCE DUE' : 'ACCOUNT SETTLED',
                  style: TextStyle(
                    color: Colors.white.withValues(alpha: 0.7),
                    fontSize: 10,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 0.7,
                  ),
                ),
                const SizedBox(height: 4),
                FittedBox(
                  fit: BoxFit.scaleDown,
                  alignment: Alignment.centerLeft,
                  child: Text(
                    money(due.abs()),
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 28,
                      height: 1.05,
                      fontWeight: FontWeight.w800,
                      letterSpacing: -0.4,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          FilledButton(
            onPressed: onPay,
            style: FilledButton.styleFrom(
              backgroundColor: Colors.white,
              foregroundColor: AppColors.primaryDark,
              elevation: 0,
              minimumSize: const Size(0, 40),
              padding: const EdgeInsets.symmetric(horizontal: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(999),
              ),
            ),
            child: Text(
              due > 0 ? 'Pay' : 'Ledger',
              style: const TextStyle(fontWeight: FontWeight.w800),
            ),
          ),
        ],
      ),
    );
  }
}

class _SummaryGauge extends StatelessWidget {
  const _SummaryGauge({
    required this.label,
    required this.value,
    required this.progress,
    required this.color,
    required this.onTap,
  });

  final String label;
  final int value;
  final double progress;
  final Color color;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return _TealCard(
      radius: 14,
      onTap: onTap,
      padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 6),
      child: Column(
        children: [
          SizedBox(
            width: 42,
            height: 42,
            child: CustomPaint(
              painter: _ArcPainter(progress: progress, color: color),
              child: Center(
                child: Text(
                  '$value',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 14,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
            ),
          ),
          const SizedBox(height: 5),
          Text(
            label,
            style: TextStyle(
              color: Colors.white.withValues(alpha: 0.85),
              fontSize: 11,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }
}

class _ArcPainter extends CustomPainter {
  _ArcPainter({required this.progress, required this.color});

  final double progress;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final center = Offset(size.width / 2, size.height / 2);
    final radius = size.shortestSide / 2 - 2;
    final bg = Paint()
      ..color = Colors.white.withValues(alpha: 0.18)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 3.5
      ..strokeCap = StrokeCap.round;
    final fg = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 3.5
      ..strokeCap = StrokeCap.round;
    canvas.drawArc(
      Rect.fromCircle(center: center, radius: radius),
      -1.2,
      5.0,
      false,
      bg,
    );
    canvas.drawArc(
      Rect.fromCircle(center: center, radius: radius),
      -1.2,
      5.0 * progress.clamp(0.0, 1.0),
      false,
      fg,
    );
  }

  @override
  bool shouldRepaint(covariant _ArcPainter oldDelegate) =>
      oldDelegate.progress != progress || oldDelegate.color != color;
}

class _HomeNotices extends StatelessWidget {
  const _HomeNotices({required this.notices});

  final List<CustomerNotice> notices;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(18, 10, 18, 0),
      child: Column(
        children: [
          for (final notice in notices)
            Container(
              width: double.infinity,
              margin: const EdgeInsets.only(bottom: 8),
              padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
              decoration: BoxDecoration(
                color: switch (notice.tone) {
                  'success' => const Color(0xFFEDF7F3),
                  'warning' => const Color(0xFFFFF7E8),
                  _ => const Color(0xFFEFF6FF),
                },
                borderRadius: BorderRadius.circular(12),
                border: Border.all(
                  color: switch (notice.tone) {
                    'success' => const Color(0xFFB7E0CF),
                    'warning' => const Color(0xFFF0C27A),
                    _ => const Color(0xFFBFDBFE),
                  },
                ),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(
                    switch (notice.tone) {
                      // Announcement / success: megaphone (same look as prior builds).
                      'success' => Icons.campaign_rounded,
                      'warning' => Icons.warning_amber_rounded,
                      _ => Icons.info_rounded,
                    },
                    size: 22,
                    color: switch (notice.tone) {
                      'success' => AppColors.primary,
                      'warning' => const Color(0xFFB45309),
                      _ => AppColors.blue,
                    },
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          notice.title,
                          style: const TextStyle(
                            fontWeight: FontWeight.w800,
                            fontSize: 14,
                          ),
                        ),
                        if (notice.body.trim().isNotEmpty) ...[
                          const SizedBox(height: 4),
                          Text(
                            notice.body,
                            style: const TextStyle(
                              color: AppColors.muted,
                              fontSize: 12,
                              height: 1.35,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _HomeSectionHeader extends StatelessWidget {
  const _HomeSectionHeader({
    required this.title,
    required this.actionLabel,
    required this.onAction,
  });

  final String title;
  final String actionLabel;
  final VoidCallback onAction;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Text(
            title,
            style: const TextStyle(
              color: AppColors.ink,
              fontSize: 17,
              height: 1.2,
              fontWeight: FontWeight.w800,
            ),
          ),
        ),
        TextButton(onPressed: onAction, child: Text(actionLabel)),
      ],
    );
  }
}

class _HomeEmptyOrders extends StatelessWidget {
  const _HomeEmptyOrders({required this.onQuote});

  final VoidCallback onQuote;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 10),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.line),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'No open orders yet',
            style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 4),
          const Text(
            'Request a quote to start your next project.',
            style: TextStyle(fontSize: 12, color: AppColors.muted),
          ),
          TextButton(
            onPressed: onQuote,
            child: const Text('Request a quote'),
          ),
        ],
      ),
    );
  }
}

class _OrdersCarousel extends StatefulWidget {
  const _OrdersCarousel({
    required this.api,
    required this.orders,
    required this.quotations,
    required this.onOpen,
  });

  final ApiClient api;
  final List<ManufacturingOrder> orders;
  final List<Quotation> quotations;
  final ValueChanged<ManufacturingOrder> onOpen;

  @override
  State<_OrdersCarousel> createState() => _OrdersCarouselState();
}

class _OrdersCarouselState extends State<_OrdersCarousel> {
  late final PageController _controller = PageController(viewportFraction: 0.9);
  int _index = 0;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  String _photoFor(ManufacturingOrder order) {
    if (order.resolvedImageFileIds.isNotEmpty) {
      return order.resolvedImageFileIds.first;
    }
    for (final quote in widget.quotations) {
      if (quote.id == order.quotationId &&
          quote.resolvedImageFileIds.isNotEmpty) {
        return quote.resolvedImageFileIds.first;
      }
    }
    return '';
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        SizedBox(
          height: 112,
          child: PageView.builder(
            controller: _controller,
            itemCount: widget.orders.length,
            onPageChanged: (value) => setState(() => _index = value),
            itemBuilder: (context, index) {
              final order = widget.orders[index];
              return Padding(
                padding: const EdgeInsets.only(right: 10),
                child: _OrderCarouselCard(
                  api: widget.api,
                  order: order,
                  photoId: _photoFor(order),
                  onTap: () => widget.onOpen(order),
                ),
              );
            },
          ),
        ),
        if (widget.orders.length > 1) ...[
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              for (var i = 0; i < widget.orders.length.clamp(0, 8); i++)
                Container(
                  margin: const EdgeInsets.symmetric(horizontal: 3),
                  width: i == _index.clamp(0, 7) ? 14 : 6,
                  height: 6,
                  decoration: BoxDecoration(
                    color: i == _index.clamp(0, 7)
                        ? AppColors.primary
                        : AppColors.line,
                    borderRadius: BorderRadius.circular(3),
                  ),
                ),
            ],
          ),
        ],
      ],
    );
  }
}

class _OrderCarouselCard extends StatelessWidget {
  const _OrderCarouselCard({
    required this.api,
    required this.order,
    required this.photoId,
    required this.onTap,
  });

  final ApiClient api;
  final ManufacturingOrder order;
  final String photoId;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final progress = order.progress.clamp(0, 100);
    final stage = displayStageLabel(
      order.currentStage.isEmpty ? order.status : order.currentStage,
    );
    final code = manufacturingJobCode(order);
    final eta = readinessText(order.expectedDate);

    return Material(
      color: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: const BorderSide(color: AppColors.line),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(10),
          child: Row(
            children: [
              _DashboardPhoto(
                api: api,
                fileId: photoId,
                size: 88,
                backgroundColor: AppColors.surfaceSoft,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      code,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: AppColors.ink,
                        fontSize: 14,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    const SizedBox(height: 6),
                    Text(
                      stage,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: AppColors.primaryDark,
                        fontSize: 13,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      eta,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: AppColors.muted,
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const Spacer(),
                    Row(
                      children: [
                        Expanded(
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(99),
                            child: LinearProgressIndicator(
                              value: progress / 100,
                              minHeight: 6,
                              backgroundColor: AppColors.surfaceSoft,
                              color: AppColors.primary,
                            ),
                          ),
                        ),
                        const SizedBox(width: 8),
                        Text(
                          '$progress%',
                          style: const TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w800,
                            color: AppColors.primaryDark,
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
      ),
    );
  }
}

class _QuotesActionCarousel extends StatefulWidget {
  const _QuotesActionCarousel({
    required this.api,
    required this.quotes,
    required this.onAction,
  });

  final ApiClient api;
  final List<Quotation> quotes;
  final ActionRunner onAction;

  @override
  State<_QuotesActionCarousel> createState() => _QuotesActionCarouselState();
}

class _QuotesActionCarouselState extends State<_QuotesActionCarousel> {
  final _controller = PageController(viewportFraction: 0.92);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 178,
      child: PageView.builder(
        controller: _controller,
        itemCount: widget.quotes.length,
        itemBuilder: (context, index) {
          final quote = widget.quotes[index];
          return Padding(
            padding: const EdgeInsets.only(right: 10),
            child: _QuoteActionCard(
              api: widget.api,
              quote: quote,
              onOpen: () => showModalBottomSheet<void>(
                context: context,
                isScrollControlled: true,
                useSafeArea: true,
                builder: (_) => _DashboardQuoteReview(
                  api: widget.api,
                  quote: quote,
                  onAction: widget.onAction,
                ),
              ),
              onProceed: () => _respond(quote, true),
              onReject: () => _respond(quote, false),
            ),
          );
        },
      ),
    );
  }

  Future<void> _respond(Quotation quote, bool accept) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(accept ? 'Place this order?' : 'Reject this quote?'),
        content: Text(
          accept
              ? '${quote.quantity} pieces for ${money(quote.totalAmount)}.'
              : quote.productName,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Back'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: Text(accept ? 'Proceed' : 'Reject'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    await widget.onAction(() async {
      if (accept) {
        await widget.api.acceptQuote(quote.id);
      } else {
        await widget.api.rejectQuote(quote.id);
      }
    }, accept ? 'Order created from quote.' : 'Quote rejected.');
  }
}

class _QuoteActionCard extends StatelessWidget {
  const _QuoteActionCard({
    required this.api,
    required this.quote,
    required this.onOpen,
    required this.onProceed,
    required this.onReject,
  });

  final ApiClient api;
  final Quotation quote;
  final VoidCallback onOpen;
  final VoidCallback onProceed;
  final VoidCallback onReject;

  @override
  Widget build(BuildContext context) {
    final photo = quote.resolvedImageFileIds.isEmpty
        ? ''
        : quote.resolvedImageFileIds.first;
    final unit = quote.quantity > 0
        ? (quote.totalAmount / quote.quantity).round()
        : 0;

    return Material(
      color: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: const BorderSide(color: AppColors.line),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onOpen,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            children: [
              Expanded(
                child: Row(
                  children: [
                    _DashboardPhoto(
                      api: api,
                      fileId: photo,
                      size: 72,
                      backgroundColor: AppColors.surfaceSoft,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text(
                            'Price ready',
                            style: TextStyle(
                              color: AppColors.blue,
                              fontSize: 11,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          const SizedBox(height: 4),
                          Text(
                            quote.productName.isEmpty
                                ? 'Product quote'
                                : quote.productName,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                          const Spacer(),
                          Text(
                            '${quote.quantity} pcs',
                            style: const TextStyle(
                              fontSize: 12,
                              color: AppColors.muted,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          const SizedBox(height: 4),
                          Text(
                            'Unit ${money(unit)}',
                            style: const TextStyle(
                              fontSize: 13,
                              fontWeight: FontWeight.w700,
                              color: AppColors.ink,
                            ),
                          ),
                          Text(
                            'Total ${money(quote.totalAmount)}',
                            style: const TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w800,
                              color: AppColors.primaryDark,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 10),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton(
                      onPressed: onReject,
                      style: OutlinedButton.styleFrom(
                        foregroundColor: AppColors.red,
                        side: BorderSide(
                          color: AppColors.red.withValues(alpha: 0.35),
                        ),
                        minimumSize: const Size(0, 38),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(10),
                        ),
                      ),
                      child: const Text('Reject'),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: FilledButton(
                      onPressed: onProceed,
                      style: FilledButton.styleFrom(
                        backgroundColor: AppColors.primary,
                        minimumSize: const Size(0, 38),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(10),
                        ),
                      ),
                      child: const Text('Proceed'),
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

class _DashboardQuoteReview extends StatefulWidget {
  const _DashboardQuoteReview({
    required this.api,
    required this.quote,
    required this.onAction,
  });
  final ApiClient api;
  final Quotation quote;
  final ActionRunner onAction;

  @override
  State<_DashboardQuoteReview> createState() => _DashboardQuoteReviewState();
}

class _DashboardQuoteReviewState extends State<_DashboardQuoteReview> {
  bool _busy = false;

  Future<void> _respond(bool accept) async {
    if (_busy) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(accept ? 'Place this order?' : 'Reject this quote?'),
        content: Text(
          accept
              ? '${widget.quote.quantity} pieces for ${money(widget.quote.totalAmount)}.'
              : widget.quote.productName,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Back'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: Text(accept ? 'Proceed' : 'Reject'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    setState(() => _busy = true);
    var succeeded = false;
    try {
      await widget.onAction(() async {
        if (accept) {
          await widget.api.acceptQuote(widget.quote.id);
        } else {
          await widget.api.rejectQuote(widget.quote.id);
        }
        succeeded = true;
      }, accept ? 'Order created from quote.' : 'Quote rejected.');
      if (succeeded && mounted) Navigator.pop(context);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 24),
      child: Column(
        children: [
          Row(
            children: [
              const Expanded(
                child: Text(
                  'Quotation details',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
                ),
              ),
              IconButton(
                tooltip: 'Close',
                onPressed: () => Navigator.pop(context),
                icon: const Icon(Icons.close_rounded),
              ),
            ],
          ),
          if (_busy) const LinearProgressIndicator(minHeight: 2),
          AbsorbPointer(
            absorbing: _busy,
            child: QuoteTile(
              api: widget.api,
              quote: widget.quote,
              onAccept: widget.quote.status == 'Priced'
                  ? () => _respond(true)
                  : null,
              onReject: widget.quote.status == 'Priced'
                  ? () => _respond(false)
                  : null,
            ),
          ),
        ],
      ),
    );
  }
}

class _DashboardShipment extends StatelessWidget {
  const _DashboardShipment({required this.shipment, required this.onTap});
  final ShippingRequest shipment;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: const BorderSide(color: AppColors.line),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Row(
            children: [
              Container(
                width: 40,
                height: 40,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: AppColors.blue.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: const Icon(
                  Icons.local_shipping_outlined,
                  size: 20,
                  color: AppColors.blue,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${shipment.courier} ${shipment.service}'.trim(),
                      style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 3),
                    Text(
                      shipment.destination,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 12,
                        color: AppColors.muted,
                      ),
                    ),
                  ],
                ),
              ),
              Text(
                shipment.status,
                style: const TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.w700,
                  color: AppColors.blue,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DashboardPhoto extends StatefulWidget {
  const _DashboardPhoto({
    required this.api,
    required this.fileId,
    required this.size,
    this.backgroundColor = const Color(0xFFF0F3F4),
  });
  final ApiClient api;
  final String fileId;
  final double size;
  final Color backgroundColor;

  @override
  State<_DashboardPhoto> createState() => _DashboardPhotoState();
}

class _DashboardPhotoState extends State<_DashboardPhoto> {
  Uint8List? _bytes;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(covariant _DashboardPhoto oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.fileId != widget.fileId || oldWidget.api != widget.api) {
      _load();
    }
  }

  Future<void> _load() async {
    final generation = ++_generation;
    _bytes = null;
    if (widget.fileId.trim().isEmpty) return;
    try {
      final bytes = await widget.api.fetchFileBytes(widget.fileId);
      if (mounted && generation == _generation) setState(() => _bytes = bytes);
    } catch (_) {
      if (mounted && generation == _generation) setState(() => _bytes = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    final pixelSize = (widget.size * MediaQuery.devicePixelRatioOf(context))
        .round();
    final placeholder = Center(
      child: Icon(
        Icons.image_outlined,
        size: 24,
        color: AppColors.muted.withValues(alpha: 0.5),
      ),
    );
    return ClipRRect(
      borderRadius: BorderRadius.circular(12),
      child: ColoredBox(
        color: widget.backgroundColor,
        child: SizedBox(
          width: widget.size,
          height: widget.size,
          child: _bytes == null
              ? placeholder
              : Image.memory(
                  _bytes!,
                  key: ValueKey(widget.fileId),
                  width: widget.size,
                  height: widget.size,
                  fit: BoxFit.cover,
                  cacheWidth: pixelSize,
                  filterQuality: FilterQuality.low,
                  errorBuilder: (_, error, stack) => placeholder,
                ),
        ),
      ),
    );
  }
}
