import 'models.dart';

class ShippingRateSnapshot {
  const ShippingRateSnapshot({
    required this.books,
    required this.activeBooks,
    required this.activeServices,
    this.latestEffective = '',
  });

  factory ShippingRateSnapshot.fromJson(JsonMap json) {
    return ShippingRateSnapshot(
      books: const [],
      activeBooks: readInt(json, 'activeBooks'),
      activeServices: readInt(json, 'activeServices'),
      latestEffective: readString(json, 'latestEffective'),
    );
  }

  final List<Object> books;
  final int activeBooks;
  final int activeServices;
  final String latestEffective;
}

class ShippingLookupRequest {
  const ShippingLookupRequest({
    required this.country,
    required this.postalCode,
    required this.dutyMode,
    required this.weightKg,
    required this.lengthCm,
    required this.widthCm,
    required this.heightCm,
    required this.packages,
    required this.includePsw,
    this.carrier = '',
    this.serviceId = '',
  });

  final String country;
  final String postalCode;
  final String dutyMode;
  final double weightKg;
  final double lengthCm;
  final double widthCm;
  final double heightCm;
  final int packages;
  final bool includePsw;
  final String carrier;
  final String serviceId;

  Map<String, Object?> toJson() => {
        'country': country,
        'postalCode': postalCode,
        'dutyMode': dutyMode,
        'weightKg': weightKg,
        'lengthCm': lengthCm,
        'widthCm': widthCm,
        'heightCm': heightCm,
        'packages': packages,
        'includePsw': includePsw,
        if (carrier.isNotEmpty) 'carrier': carrier,
        if (serviceId.isNotEmpty) 'serviceId': serviceId,
      };

  ShippingLookupRequest copyWith({String? carrier, String? serviceId}) {
    return ShippingLookupRequest(
      country: country,
      postalCode: postalCode,
      dutyMode: dutyMode,
      weightKg: weightKg,
      lengthCm: lengthCm,
      widthCm: widthCm,
      heightCm: heightCm,
      packages: packages,
      includePsw: includePsw,
      carrier: carrier ?? this.carrier,
      serviceId: serviceId ?? this.serviceId,
    );
  }
}

class ShippingChargeLine {
  const ShippingChargeLine({
    required this.label,
    required this.amount,
    required this.currency,
  });

  factory ShippingChargeLine.fromJson(JsonMap json) {
    return ShippingChargeLine(
      label: readString(json, 'label'),
      amount: readInt(json, 'amount'),
      currency: readString(json, 'currency'),
    );
  }

  final String label;
  final int amount;
  final String currency;
}

class ShippingRateOption {
  const ShippingRateOption({
    required this.rateBookId,
    required this.serviceId,
    required this.carrier,
    required this.service,
    required this.currency,
    required this.effectiveDate,
    required this.etaMinDays,
    required this.etaMaxDays,
    required this.zone,
    required this.region,
    required this.dutyMode,
    required this.packages,
    required this.actualWeightKg,
    required this.volumetricWeightKg,
    required this.chargeableWeightKg,
    required this.rateMode,
    required this.rateWeightKg,
    required this.unitRate,
    required this.baseAmount,
    required this.charges,
    required this.totalAmount,
    required this.requiresReview,
    this.reviewReasons = const [],
  });

  factory ShippingRateOption.fromJson(JsonMap json) {
    return ShippingRateOption(
      rateBookId: readString(json, 'rateBookId'),
      serviceId: readString(json, 'serviceId'),
      carrier: readString(json, 'carrier'),
      service: readString(json, 'service'),
      currency: readString(json, 'currency'),
      effectiveDate: readString(json, 'effectiveDate'),
      etaMinDays: readInt(json, 'etaMinDays'),
      etaMaxDays: readInt(json, 'etaMaxDays'),
      zone: readString(json, 'zone'),
      region: readString(json, 'region'),
      dutyMode: readString(json, 'dutyMode'),
      packages: readInt(json, 'packages'),
      actualWeightKg: readDouble(json, 'actualWeightKg'),
      volumetricWeightKg: readDouble(json, 'volumetricWeightKg'),
      chargeableWeightKg: readDouble(json, 'chargeableWeightKg'),
      rateMode: readString(json, 'rateMode'),
      rateWeightKg: readDouble(json, 'rateWeightKg'),
      unitRate: readInt(json, 'unitRate'),
      baseAmount: readInt(json, 'baseAmount'),
      charges: readList(json, 'charges', ShippingChargeLine.fromJson),
      totalAmount: readInt(json, 'totalAmount'),
      requiresReview: json['requiresReview'] == true,
      reviewReasons: readStringList(json, 'reviewReasons'),
    );
  }

  final String rateBookId;
  final String serviceId;
  final String carrier;
  final String service;
  final String currency;
  final String effectiveDate;
  final int etaMinDays;
  final int etaMaxDays;
  final String zone;
  final String region;
  final String dutyMode;
  final int packages;
  final double actualWeightKg;
  final double volumetricWeightKg;
  final double chargeableWeightKg;
  final String rateMode;
  final double rateWeightKg;
  final int unitRate;
  final int baseAmount;
  final List<ShippingChargeLine> charges;
  final int totalAmount;
  final bool requiresReview;
  final List<String> reviewReasons;
}

class ShippingLookupResponse {
  const ShippingLookupResponse({
    required this.postalPrefix,
    required this.zone,
    required this.region,
    required this.options,
    this.warnings = const [],
  });

  factory ShippingLookupResponse.fromJson(JsonMap json) {
    return ShippingLookupResponse(
      postalPrefix: readString(json, 'postalPrefix'),
      zone: readString(json, 'zone'),
      region: readString(json, 'region'),
      options: readList(json, 'options', ShippingRateOption.fromJson),
      warnings: readStringList(json, 'warnings'),
    );
  }

  final String postalPrefix;
  final String zone;
  final String region;
  final List<ShippingRateOption> options;
  final List<String> warnings;
}

class ShippingBookingRequest {
  const ShippingBookingRequest({
    required this.lookup,
    required this.recipientName,
    required this.phone,
    required this.addressLine1,
    required this.addressLine2,
    required this.city,
    required this.state,
    required this.postalCode,
    required this.contents,
  });

  final ShippingLookupRequest lookup;
  final String recipientName;
  final String phone;
  final String addressLine1;
  final String addressLine2;
  final String city;
  final String state;
  final String postalCode;
  final String contents;

  Map<String, Object?> toJson() => {
        'lookup': lookup.toJson(),
        'recipientName': recipientName,
        'phone': phone,
        'addressLine1': addressLine1,
        'addressLine2': addressLine2,
        'city': city,
        'state': state,
        'postalCode': postalCode,
        'contents': contents,
      };
}

double readDouble(JsonMap json, String key) {
  final value = json[key];
  if (value is num) return value.toDouble();
  return double.tryParse('$value') ?? 0;
}

String shippingMoney(int amount, String currency) {
  final code = currency.trim().isEmpty ? 'USD' : currency.trim().toUpperCase();
  final formatted = amount.toString().replaceAllMapped(
        RegExp(r'(\d)(?=(\d{3})+$)'),
        (match) => '${match[1]},',
      );
  if (code == 'USD') return '\$$formatted';
  if (code == 'PKR' || code == 'RS') return 'Rs $formatted';
  return '$code $formatted';
}
