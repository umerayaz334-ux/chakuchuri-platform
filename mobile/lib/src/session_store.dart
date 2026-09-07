import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'models.dart';

class SessionStore {
  const SessionStore({FlutterSecureStorage storage = const FlutterSecureStorage()})
      : _storage = storage;

  final FlutterSecureStorage _storage;
  static const _key = 'chakuchuri.session.v1';

  Future<SessionStoreData?> read() async {
    final raw = await _storage.read(key: _key);
    if (raw == null || raw.isEmpty) return null;
    try {
      return SessionStoreData.fromJson(asJsonMap(jsonDecode(raw)));
    } on FormatException {
      await _storage.delete(key: _key);
      return null;
    }
  }

  Future<void> write(SessionStoreData data) =>
      _storage.write(key: _key, value: jsonEncode(data.toJson()));
}

class SessionStoreData {
  const SessionStoreData({required this.baseUrl, this.session});

  factory SessionStoreData.fromJson(JsonMap json) {
    final sessionJson = asJsonMap(json['session']);
    return SessionStoreData(
      baseUrl: readString(json, 'baseUrl'),
      session: readString(sessionJson, 'token').isEmpty
          ? null : Session.fromJson(sessionJson),
    );
  }

  final String baseUrl;
  final Session? session;

  JsonMap toJson() => {'baseUrl': baseUrl, 'session': session?.toJson()};
}
