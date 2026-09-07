import 'package:flutter/material.dart';

import '../api_client.dart';
import '../countries.dart';
import '../models.dart';
import '../shipping_rates.dart';
import '../theme.dart';
import '../widgets.dart';

typedef ShippingActionRunner = Future<void> Function(
  Future<void> Function() action,
  String success,
);

bool _shipClosed(String value) {
  final normalized = value.toLowerCase();
  return normalized.contains('complete') ||
      normalized.contains('cancel') ||
      normalized.contains('reject') ||
      normalized.contains('delivered');
}

class ShippingPage extends StatefulWidget {
  const ShippingPage({
    super.key,
    required this.api,
    required this.workspace,
    required this.onAction,
    this.footer,
  });

  final ApiClient api;
  final Workspace workspace;
  final ShippingActionRunner onAction;
  final Widget? footer;

  @override
  State<ShippingPage> createState() => _ShippingPageState();
}

class _ShippingPageState extends State<ShippingPage> {
  final _zip = TextEditingController();
  final _weight = TextEditingController(text: '1');
  final _packages = TextEditingController(text: '1');
  final _length = TextEditingController();
  final _width = TextEditingController();
  final _height = TextEditingController();

  String _tab = 'rates';
  String _countryCode = 'US';
  String _dutyMode = 'duty_paid';
  bool _includePsw = true;
  bool _showDimensions = false;
  bool _loadingSnapshot = true;
  bool _lookupBusy = false;
  String _error = '';
  ShippingRateSnapshot? _snapshot;
  ShippingLookupRequest? _lastRequest;
  ShippingLookupResponse? _result;

  @override
  void initState() {
    super.initState();
    _loadSnapshot();
  }

  @override
  void dispose() {
    _zip.dispose();
    _weight.dispose();
    _packages.dispose();
    _length.dispose();
    _width.dispose();
    _height.dispose();
    super.dispose();
  }

  Future<void> _loadSnapshot() async {
    setState(() {
      _loadingSnapshot = true;
      _error = '';
    });
    try {
      final snapshot = await widget.api.shippingRateSnapshot();
      if (!mounted) return;
      setState(() {
        _snapshot = snapshot;
        _loadingSnapshot = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loadingSnapshot = false;
        _error = error.toString();
      });
    }
  }

  Future<void> _showRates() async {
    final zip = _zip.text.trim();
    final weight = double.tryParse(_weight.text.trim()) ?? 0;
    final packages = int.tryParse(_packages.text.trim()) ?? 0;
    if (zip.isEmpty ||
        (_countryCode == 'US' &&
            !RegExp(r'^\d{5}(?:-\d{4})?$').hasMatch(zip))) {
      setState(() {
        _error = _countryCode == 'US'
            ? 'Enter a valid five-digit US ZIP code.'
            : 'Enter a valid postal code.';
      });
      return;
    }
    if (weight <= 0 || packages < 1) {
      setState(
        () => _error = 'Enter a valid package weight and package count.',
      );
      return;
    }
    final lengthText = _length.text.trim();
    final widthText = _width.text.trim();
    final heightText = _height.text.trim();
    final anyDimension =
        lengthText.isNotEmpty || widthText.isNotEmpty || heightText.isNotEmpty;
    final length = double.tryParse(lengthText) ?? 0;
    final width = double.tryParse(widthText) ?? 0;
    final height = double.tryParse(heightText) ?? 0;
    if (anyDimension && (length <= 0 || width <= 0 || height <= 0)) {
      setState(
        () => _error = 'Dimensions are optional. Fill length, width and height together, or leave all blank.',
      );
      return;
    }
    final request = ShippingLookupRequest(
      country: _countryCode,
      postalCode: zip,
      dutyMode: _dutyMode,
      weightKg: weight,
      lengthCm: length,
      widthCm: width,
      heightCm: height,
      packages: packages,
      includePsw: _includePsw,
    );
    FocusManager.instance.primaryFocus?.unfocus();
    setState(() {
      _lookupBusy = true;
      _error = '';
      _result = null;
    });
    try {
      final response = await widget.api.lookupShippingRates(request);
      if (!mounted) return;
      setState(() {
        _lastRequest = request;
        _result = response;
        _lookupBusy = false;
      });
      await _openRateResults(response);
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _lookupBusy = false;
        _error = error.toString();
      });
    }
  }

  Future<void> _openRateResults(ShippingLookupResponse response) async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: false,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(8)),
      ),
      builder: (sheetContext) => FractionallySizedBox(
        heightFactor: 0.88,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 10, 8, 10),
              child: Column(
                children: [
                  Container(
                    width: 42,
                    height: 4,
                    decoration: BoxDecoration(
                      color: AppColors.line,
                      borderRadius: BorderRadius.circular(99),
                    ),
                  ),
                  const SizedBox(height: 12),
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
                          Icons.local_shipping_outlined,
                          color: AppColors.primary,
                        ),
                      ),
                      const SizedBox(width: 10),
                      const Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'Available services',
                              style: TextStyle(
                                fontSize: 20,
                                fontWeight: FontWeight.w900,
                              ),
                            ),
                            Text(
                              'Compare price and delivery time.',
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
                        onPressed: () => Navigator.pop(sheetContext),
                        icon: const Icon(Icons.close_rounded),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            const Divider(height: 1),
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(16, 14, 16, 24),
                child: _RateResults(
                  result: response,
                  onBook: (option) async {
                    Navigator.pop(sheetContext);
                    await Future<void>.delayed(
                      const Duration(milliseconds: 180),
                    );
                    if (mounted) await _book(option);
                  },
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _book(ShippingRateOption option) async {
    final lookup = _lastRequest;
    if (lookup == null) return;
    final booked = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(8)),
      ),
      builder: (context) => _BookingSheet(
        api: widget.api,
        option: option,
        lookup: lookup,
        onAction: widget.onAction,
      ),
    );
    if (booked == true && mounted) {
      setState(() => _tab = 'shipments');
    }
  }

  @override
  Widget build(BuildContext context) {
    final shipments = [...widget.workspace.shipping]
      ..sort((a, b) {
        final recent = b.createdAt.compareTo(a.createdAt);
        return recent != 0 ? recent : b.id.compareTo(a.id);
      });
    final active = shipments.where((s) => !_shipClosed(s.status)).length;

    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 16, 14, 24),
      children: [
        Container(
          padding: const EdgeInsets.all(4),
          decoration: BoxDecoration(
            color: const Color(0xFFEEF3F0),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            children: [
              Expanded(
                child: _ShipTab(
                  label: 'Get rates',
                  selected: _tab == 'rates',
                  onTap: () => setState(() => _tab = 'rates'),
                ),
              ),
              Expanded(
                child: _ShipTab(
                  label: 'Shipments ($active)',
                  selected: _tab == 'shipments',
                  onTap: () => setState(() => _tab = 'shipments'),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 14),
        if (_tab == 'rates') ...[
          if (_error.isNotEmpty && _snapshot == null && !_loadingSnapshot)
            _InlineError(message: _error, onRetry: _loadSnapshot),
          if (_loadingSnapshot)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 48),
              child: Center(child: CircularProgressIndicator()),
            )
          else if ((_snapshot?.activeBooks ?? 0) < 1)
            const EmptyState(
              'Rates are being updated',
              detail: 'The shipping team will publish the next active rate book shortly.',
              icon: Icons.table_chart_outlined,
            )
          else ...[
            _CalculatorForm(
              zip: _zip,
              weight: _weight,
              packages: _packages,
              length: _length,
              width: _width,
              height: _height,
              countryCode: _countryCode,
              dutyMode: _dutyMode,
              includePsw: _includePsw,
              showDimensions: _showDimensions,
              busy: _lookupBusy,
              error: _error,
              onDutyMode: (value) => setState(() => _dutyMode = value),
              onCountry: (value) => setState(() {
                _countryCode = value;
                _result = null;
                _error = '';
              }),
              onPsw: (value) => setState(() => _includePsw = value),
              onToggleDimensions: () =>
                  setState(() => _showDimensions = !_showDimensions),
              onShowRates: _showRates,
            ),
            if (_result != null) ...[
              const SizedBox(height: 10),
              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: () => _openRateResults(_result!),
                  icon: const Icon(Icons.receipt_long_outlined),
                  label: const Text('View last offers'),
                ),
              ),
            ],
          ],
        ] else ...[
          if (shipments.isEmpty)
            const EmptyState(
              'No shipments yet',
              detail: 'Calculate a rate and book a service to create one.',
              icon: Icons.local_shipping_outlined,
            )
          else ...[
            _ShipmentOverview(shipments: shipments),
            const SizedBox(height: 12),
            ...shipments.map(_ShipmentCard.new),
          ],
          if (widget.footer != null) widget.footer!,
        ],
      ],
    );
  }
}

class _ShipTab extends StatelessWidget {
  const _ShipTab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: selected ? Colors.white : Colors.transparent,
      borderRadius: BorderRadius.circular(8),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: SizedBox(
          height: 40,
          child: Center(
            child: Text(
              label,
              style: TextStyle(
                fontWeight: FontWeight.w800,
                fontSize: 13,
                color: selected ? AppColors.ink : AppColors.muted,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _InlineError extends StatelessWidget {
  const _InlineError({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFFFEF2F2),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFFFECACA)),
      ),
      child: Row(
        children: [
          const Icon(Icons.error_outline, color: AppColors.red, size: 18),
          const SizedBox(width: 8),
          Expanded(child: Text(message, style: const TextStyle(fontSize: 12))),
          TextButton(onPressed: onRetry, child: const Text('Retry')),
        ],
      ),
    );
  }
}

class _CalculatorForm extends StatelessWidget {
  const _CalculatorForm({
    required this.zip,
    required this.weight,
    required this.packages,
    required this.length,
    required this.width,
    required this.height,
    required this.countryCode,
    required this.dutyMode,
    required this.includePsw,
    required this.showDimensions,
    required this.busy,
    required this.error,
    required this.onDutyMode,
    required this.onCountry,
    required this.onPsw,
    required this.onToggleDimensions,
    required this.onShowRates,
  });

  final TextEditingController zip;
  final TextEditingController weight;
  final TextEditingController packages;
  final TextEditingController length;
  final TextEditingController width;
  final TextEditingController height;
  final String countryCode;
  final String dutyMode;
  final bool includePsw;
  final bool showDimensions;
  final bool busy;
  final String error;
  final ValueChanged<String> onDutyMode;
  final ValueChanged<String> onCountry;
  final ValueChanged<bool> onPsw;
  final VoidCallback onToggleDimensions;
  final VoidCallback onShowRates;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: _cardDecoration(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'RATE CALCULATOR',
            style: TextStyle(
              color: AppColors.primary,
              fontSize: 10,
              fontWeight: FontWeight.w900,
            ),
          ),
          const SizedBox(height: 4),
          const Text(
            'Package details',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: _DutyChip(
                  label: 'Duty paid',
                  selected: dutyMode == 'duty_paid',
                  onTap: () => onDutyMode('duty_paid'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: _DutyChip(
                  label: 'Non-duty paid',
                  selected: dutyMode == 'non_duty_paid',
                  onTap: () => onDutyMode('non_duty_paid'),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          _CountryField(countryCode: countryCode, onChanged: onCountry),
          const SizedBox(height: 10),
          TextField(
            controller: zip,
            keyboardType: countryCode == 'US'
                ? TextInputType.number
                : TextInputType.streetAddress,
            textCapitalization: TextCapitalization.characters,
            decoration: InputDecoration(
              labelText: countryCode == 'US' ? 'ZIP code' : 'Postal code',
              hintText: countryCode == 'US' ? 'e.g. 10001' : null,
              prefixIcon: const Icon(Icons.pin_drop_outlined),
            ),
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: weight,
                  keyboardType: const TextInputType.numberWithOptions(
                    decimal: true,
                  ),
                  decoration: const InputDecoration(
                    labelText: 'Weight / box',
                    suffixText: 'kg',
                    prefixIcon: Icon(Icons.scale_outlined),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: TextField(
                  controller: packages,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(
                    labelText: 'Packages',
                    prefixIcon: Icon(Icons.inventory_2_outlined),
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton.icon(
              onPressed: onToggleDimensions,
              icon: Icon(
                showDimensions
                    ? Icons.expand_less_rounded
                    : Icons.straighten_rounded,
                size: 18,
              ),
              label: Text(
                showDimensions
                    ? 'Hide box dimensions'
                    : 'Add box dimensions (optional)',
              ),
            ),
          ),
          if (showDimensions) ...[
            const SizedBox(height: 4),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: length,
                    keyboardType: const TextInputType.numberWithOptions(
                      decimal: true,
                    ),
                    decoration: const InputDecoration(
                      labelText: 'Length',
                      suffixText: 'cm',
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: TextField(
                    controller: width,
                    keyboardType: const TextInputType.numberWithOptions(
                      decimal: true,
                    ),
                    decoration: const InputDecoration(
                      labelText: 'Width',
                      suffixText: 'cm',
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: TextField(
                    controller: height,
                    keyboardType: const TextInputType.numberWithOptions(
                      decimal: true,
                    ),
                    decoration: const InputDecoration(
                      labelText: 'Height',
                      suffixText: 'cm',
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
          ],
          Material(
            color: Colors.transparent,
            child: SwitchListTile(
              contentPadding: EdgeInsets.zero,
              value: includePsw,
              onChanged: onPsw,
              title: const Text(
                'Include PSW handling',
                style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700),
              ),
              subtitle: const Text(
                'Pakistan Single Window service',
                style: TextStyle(fontSize: 11),
              ),
            ),
          ),
          if (error.isNotEmpty) ...[
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: const Color(0xFFFEF2F2),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                error,
                style: const TextStyle(color: AppColors.red, fontSize: 12),
              ),
            ),
            const SizedBox(height: 10),
          ],
          SizedBox(
            width: double.infinity,
            height: 48,
            child: FilledButton.icon(
              onPressed: busy ? null : onShowRates,
              icon: busy
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : const Icon(Icons.calculate_outlined),
              label: Text(busy ? 'Calculating...' : 'Show rates'),
            ),
          ),
        ],
      ),
    );
  }
}

class _DutyChip extends StatelessWidget {
  const _DutyChip({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: selected
          ? AppColors.primary.withValues(alpha: 0.12)
          : const Color(0xFFF7FAF8),
      borderRadius: BorderRadius.circular(8),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: Container(
          height: 42,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(8),
            border: Border.all(
              color: selected ? AppColors.primary : AppColors.line,
            ),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontWeight: FontWeight.w800,
              fontSize: 12,
              color: selected ? AppColors.primary : AppColors.muted,
            ),
          ),
        ),
      ),
    );
  }
}

class _CountryField extends StatelessWidget {
  const _CountryField({required this.countryCode, required this.onChanged});

  final String countryCode;
  final ValueChanged<String> onChanged;

  Future<void> _openPicker(BuildContext context) async {
    final search = TextEditingController();
    var query = '';
    final selected = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      builder: (context) => StatefulBuilder(
        builder: (context, setSheetState) {
          final countries = shippingCountries.where((country) {
            final needle = query.trim().toLowerCase();
            return needle.isEmpty ||
                country.name.toLowerCase().contains(needle) ||
                country.code.toLowerCase().contains(needle);
          }).toList();
          return SafeArea(
            child: SizedBox(
              height: MediaQuery.sizeOf(context).height * 0.78,
              child: Column(
                children: [
                  Padding(
                    padding: const EdgeInsets.fromLTRB(16, 4, 8, 10),
                    child: Row(
                      children: [
                        const Expanded(
                          child: Text(
                            'Select country',
                            style: TextStyle(
                              fontSize: 20,
                              fontWeight: FontWeight.w900,
                            ),
                          ),
                        ),
                        IconButton(
                          tooltip: 'Close',
                          onPressed: () => Navigator.pop(context),
                          icon: const Icon(Icons.close_rounded),
                        ),
                      ],
                    ),
                  ),
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    child: TextField(
                      controller: search,
                      autofocus: true,
                      decoration: const InputDecoration(
                        hintText: 'Search country',
                        prefixIcon: Icon(Icons.search_rounded),
                      ),
                      onChanged: (value) => setSheetState(() => query = value),
                    ),
                  ),
                  const SizedBox(height: 8),
                  Expanded(
                    child: ListView.builder(
                      itemCount: countries.length,
                      itemBuilder: (context, index) {
                        final country = countries[index];
                        final active = country.code == countryCode;
                        return ListTile(
                          selected: active,
                          title: Text(
                            country.name,
                            style: TextStyle(
                              fontWeight: active
                                  ? FontWeight.w900
                                  : FontWeight.w600,
                            ),
                          ),
                          trailing: active
                              ? const Icon(
                                  Icons.check_rounded,
                                  color: AppColors.primary,
                                )
                              : Text(
                                  country.code,
                                  style: const TextStyle(
                                    color: AppColors.muted,
                                    fontSize: 12,
                                  ),
                                ),
                          onTap: () => Navigator.pop(context, country.code),
                        );
                      },
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
    search.dispose();
    if (selected != null && selected != countryCode) onChanged(selected);
  }

  @override
  Widget build(BuildContext context) {
    final country = shippingCountry(countryCode);
    return Semantics(
      button: true,
      label: 'Destination country ${country.name}',
      child: InkWell(
        onTap: () => _openPicker(context),
        borderRadius: BorderRadius.circular(8),
        child: InputDecorator(
          decoration: const InputDecoration(
            labelText: 'Destination country',
            prefixIcon: Icon(Icons.public_rounded),
            suffixIcon: Icon(Icons.expand_more_rounded),
          ),
          child: Text(
            country.name,
            style: const TextStyle(fontWeight: FontWeight.w700),
          ),
        ),
      ),
    );
  }
}

class _RateResults extends StatelessWidget {
  const _RateResults({required this.result, required this.onBook});

  final ShippingLookupResponse result;
  final ValueChanged<ShippingRateOption> onBook;

  @override
  Widget build(BuildContext context) {
    final options = [...result.options]
      ..sort((a, b) {
        if (a.requiresReview != b.requiresReview) {
          return a.requiresReview ? 1 : -1;
        }
        return a.totalAmount.compareTo(b.totalAmount);
      });
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    result.region.isEmpty ? 'USA' : result.region,
                    style: const TextStyle(
                      color: AppColors.muted,
                      fontSize: 11,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  Text(
                    result.zone.isEmpty
                        ? 'ZIP ${result.postalPrefix}'
                        : 'Zone ${result.zone}',
                    style: const TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ],
              ),
            ),
            Text(
              '${options.length} services',
              style: const TextStyle(
                color: AppColors.muted,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
        for (final warning in result.warnings)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(
              warning,
              style: const TextStyle(color: Color(0xFF9A6700), fontSize: 12),
            ),
          ),
        const SizedBox(height: 10),
        if (options.isEmpty)
          const EmptyState(
            'No services for this ZIP/weight',
            icon: Icons.search_off_rounded,
          )
        else
          ...options.asMap().entries.map((entry) {
            return _OptionCard(
              option: entry.value,
              recommended: entry.key == 0,
              onBook: () => onBook(entry.value),
            );
          }),
      ],
    );
  }
}

class _OptionCard extends StatelessWidget {
  const _OptionCard({
    required this.option,
    required this.recommended,
    required this.onBook,
  });

  final ShippingRateOption option;
  final bool recommended;
  final VoidCallback onBook;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: recommended ? const Color(0xFFEDF7F3) : Colors.white,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: recommended
              ? AppColors.primary.withValues(alpha: 0.35)
              : AppColors.line,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      option.carrier,
                      style: const TextStyle(
                        color: AppColors.muted,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    Text(
                      option.service,
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                  ],
                ),
              ),
              if (recommended)
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 3,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.primary,
                    borderRadius: BorderRadius.circular(99),
                  ),
                  child: const Text(
                    'Best rate',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 10,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
            ],
          ),
          const SizedBox(height: 10),
          Text(
            shippingMoney(option.totalAmount, option.currency),
            style: const TextStyle(
              fontSize: 22,
              fontWeight: FontWeight.w900,
              color: AppColors.primary,
            ),
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              const Icon(
                Icons.schedule_rounded,
                size: 15,
                color: AppColors.muted,
              ),
              const SizedBox(width: 5),
              Text(
                '${option.etaMinDays}-${option.etaMaxDays} business days',
                style: const TextStyle(
                  color: AppColors.muted,
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(width: 12),
              const Icon(
                Icons.scale_outlined,
                size: 15,
                color: AppColors.muted,
              ),
              const SizedBox(width: 5),
              Expanded(
                child: Text(
                  '${option.chargeableWeightKg.toStringAsFixed(1)} kg',
                  style: const TextStyle(
                    color: AppColors.muted,
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
          if (option.charges.isNotEmpty || option.baseAmount > 0)
            Theme(
              data: Theme.of(context)
                  .copyWith(dividerColor: Colors.transparent),
              child: Material(
                color: Colors.transparent,
                child: ExpansionTile(
                  tilePadding: EdgeInsets.zero,
                  childrenPadding: const EdgeInsets.only(bottom: 8),
                  title: const Text(
                    'Price details',
                    style: TextStyle(fontSize: 12, fontWeight: FontWeight.w800),
                  ),
                  children: [
                    _RateChargeRow(
                      label: 'Base rate',
                      amount: option.baseAmount,
                      currency: option.currency,
                    ),
                    for (final charge in option.charges)
                      _RateChargeRow(
                        label: charge.label,
                        amount: charge.amount,
                        currency: charge.currency,
                      ),
                  ],
                ),
              ),
            ),
          if (option.requiresReview) ...[
            const SizedBox(height: 8),
            Text(
              option.reviewReasons.isNotEmpty
                  ? option.reviewReasons.first
                  : 'Shipping team review required.',
              style: const TextStyle(
                color: Color(0xFF9A6700),
                fontSize: 12,
                fontWeight: FontWeight.w700,
              ),
            ),
          ] else ...[
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: onBook,
                child: const Text('Choose service'),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _RateChargeRow extends StatelessWidget {
  const _RateChargeRow({
    required this.label,
    required this.amount,
    required this.currency,
  });

  final String label;
  final int amount;
  final String currency;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        children: [
          Expanded(
            child: Text(
              label,
              style: const TextStyle(color: AppColors.muted, fontSize: 12),
            ),
          ),
          Text(
            shippingMoney(amount, currency),
            style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w800),
          ),
        ],
      ),
    );
  }
}

class _BookingSheet extends StatefulWidget {
  const _BookingSheet({
    required this.api,
    required this.option,
    required this.lookup,
    required this.onAction,
  });

  final ApiClient api;
  final ShippingRateOption option;
  final ShippingLookupRequest lookup;
  final ShippingActionRunner onAction;

  @override
  State<_BookingSheet> createState() => _BookingSheetState();
}

class _BookingSheetState extends State<_BookingSheet> {
  final _name = TextEditingController();
  final _phone = TextEditingController();
  final _address1 = TextEditingController();
  final _address2 = TextEditingController();
  final _city = TextEditingController();
  final _state = TextEditingController();
  late final TextEditingController _postal;
  final _contents = TextEditingController();
  bool _busy = false;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _postal = TextEditingController(text: widget.lookup.postalCode);
  }

  @override
  void dispose() {
    _name.dispose();
    _phone.dispose();
    _address1.dispose();
    _address2.dispose();
    _city.dispose();
    _state.dispose();
    _postal.dispose();
    _contents.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_name.text.trim().isEmpty ||
        _phone.text.trim().isEmpty ||
        _address1.text.trim().isEmpty ||
        _city.text.trim().isEmpty ||
        _postal.text.trim().isEmpty ||
        _contents.text.trim().isEmpty) {
      setState(
        () => _error =
            'Complete the recipient, address, postal code and contents.',
      );
      return;
    }
    setState(() {
      _busy = true;
      _error = '';
    });
    try {
      await widget.onAction(
        () async {
          await widget.api.bookShippingRate(
            ShippingBookingRequest(
              lookup: widget.lookup.copyWith(
                carrier: widget.option.carrier,
                serviceId: widget.option.serviceId,
              ),
              recipientName: _name.text.trim(),
              phone: _phone.text.trim(),
              addressLine1: _address1.text.trim(),
              addressLine2: _address2.text.trim(),
              city: _city.text.trim(),
              state: _state.text.trim(),
              postalCode: _postal.text.trim(),
              contents: _contents.text.trim(),
            ),
          );
        },
        'Shipment created with ${widget.option.carrier} ${widget.option.service}.',
      );
      if (mounted) Navigator.pop(context, true);
    } catch (error) {
      if (mounted) {
        setState(() {
          _busy = false;
          _error = error.toString();
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final option = widget.option;
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
                  color: AppColors.line,
                  borderRadius: BorderRadius.circular(99),
                ),
              ),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text(
                        'Create shipment',
                        style: TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.w800,
                        ),
                      ),
                      Text(
                        '${option.carrier} ${option.service} · ${shippingMoney(option.totalAmount, option.currency)}',
                        style: const TextStyle(
                          color: AppColors.muted,
                          fontSize: 12,
                        ),
                      ),
                    ],
                  ),
                ),
                IconButton(
                  onPressed: () => Navigator.pop(context),
                  icon: const Icon(Icons.close_rounded),
                ),
              ],
            ),
            const SizedBox(height: 10),
            TextField(
              controller: _name,
              decoration: const InputDecoration(labelText: 'Recipient name'),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _phone,
              keyboardType: TextInputType.phone,
              decoration: const InputDecoration(labelText: 'Phone'),
            ),
            const SizedBox(height: 8),
            InputDecorator(
              decoration: const InputDecoration(
                labelText: 'Destination country',
                prefixIcon: Icon(Icons.public_rounded),
              ),
              child: Text(
                shippingCountry(widget.lookup.country).name,
                style: const TextStyle(fontWeight: FontWeight.w700),
              ),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _address1,
              decoration: const InputDecoration(labelText: 'Address'),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _address2,
              decoration: const InputDecoration(labelText: 'Apartment / suite'),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _city,
              decoration: const InputDecoration(labelText: 'City'),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _state,
              textCapitalization: TextCapitalization.words,
              decoration: const InputDecoration(
                labelText: 'State / province (optional)',
                prefixIcon: Icon(Icons.map_outlined),
              ),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _postal,
              decoration: const InputDecoration(labelText: 'ZIP code'),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _contents,
              decoration: const InputDecoration(labelText: 'Package contents'),
            ),
            if (_error.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(
                _error,
                style: const TextStyle(color: AppColors.red, fontSize: 12),
              ),
            ],
            const SizedBox(height: 14),
            SizedBox(
              width: double.infinity,
              height: 48,
              child: FilledButton(
                onPressed: _busy ? null : _submit,
                child: Text(_busy ? 'Creating…' : 'Create shipment'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ShipmentOverview extends StatelessWidget {
  const _ShipmentOverview({required this.shipments});

  final List<ShippingRequest> shipments;

  @override
  Widget build(BuildContext context) {
    final active = shipments.where((item) => !_shipClosed(item.status)).length;
    final transit = shipments
        .where((item) => item.status.toLowerCase().contains('transit'))
        .length;
    final delivered = shipments
        .where((item) => item.status.toLowerCase().contains('delivered'))
        .length;
    return Container(
      decoration: _cardDecoration(),
      child: Row(
        children: [
          Expanded(
            child: _ShippingMetric(
              label: 'Active',
              value: active,
              color: AppColors.primary,
            ),
          ),
          const SizedBox(height: 48, child: VerticalDivider()),
          Expanded(
            child: _ShippingMetric(
              label: 'In transit',
              value: transit,
              color: AppColors.blue,
            ),
          ),
          const SizedBox(height: 48, child: VerticalDivider()),
          Expanded(
            child: _ShippingMetric(
              label: 'Delivered',
              value: delivered,
              color: AppColors.muted,
            ),
          ),
        ],
      ),
    );
  }
}

class _ShippingMetric extends StatelessWidget {
  const _ShippingMetric({
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

class _ShipmentCard extends StatelessWidget {
  const _ShipmentCard(this.shipment);

  final ShippingRequest shipment;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(14),
      decoration: _cardDecoration(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  shipment.id,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: AppColors.muted,
                    fontSize: 10,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
              StatusPill(shipment.status),
            ],
          ),
          const SizedBox(height: 7),
          Text(
            '${shipment.courier} · ${shipment.service}',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(
              color: AppColors.ink,
              fontSize: 16,
              fontWeight: FontWeight.w900,
            ),
          ),
          const SizedBox(height: 8),
          _ShipmentInfoLine(
            icon: Icons.location_on_outlined,
            value: shipment.destination.isEmpty
                ? 'Destination pending'
                : shipment.destination,
          ),
          const SizedBox(height: 6),
          _ShipmentInfoLine(
            icon: Icons.scale_outlined,
            value:
                '${shipment.weight.isEmpty ? 'Weight pending' : shipment.weight}${shipment.zone.isEmpty ? '' : ' · Zone ${shipment.zone}'}',
          ),
          const SizedBox(height: 12),
          const Divider(),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'TRACKING',
                      style: TextStyle(
                        color: AppColors.muted,
                        fontSize: 9,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      shipment.tracking.isEmpty
                          ? 'Not assigned'
                          : shipment.tracking,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                  ],
                ),
              ),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(
                    shipment.createdAt.isEmpty
                        ? ''
                        : shortDate(shipment.createdAt),
                    style: const TextStyle(
                      color: AppColors.muted,
                      fontSize: 10,
                    ),
                  ),
                  if (shipment.quotedAmount > 0)
                    Text(
                      money(shipment.quotedAmount),
                      style: const TextStyle(
                        color: AppColors.primary,
                        fontSize: 14,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                ],
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ShipmentInfoLine extends StatelessWidget {
  const _ShipmentInfoLine({required this.icon, required this.value});

  final IconData icon;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, size: 16, color: AppColors.muted),
        const SizedBox(width: 7),
        Expanded(
          child: Text(
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(color: AppColors.muted, fontSize: 12),
          ),
        ),
      ],
    );
  }
}

BoxDecoration _cardDecoration() {
  return BoxDecoration(
    color: Colors.white,
    borderRadius: BorderRadius.circular(8),
    border: Border.all(color: AppColors.line),
  );
}
