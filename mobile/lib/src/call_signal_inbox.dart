/// Serializes native peer operations; failed signals remain eligible for catch-up.
class CallSignalInbox {
  Future<void> _tail = Future<void>.value();
  final Set<String> _handled = {};
  bool _closed = false;

  Future<void> process(String id, Future<void> Function() apply) {
    final next = _tail.then((_) async {
      if (_closed || _handled.contains(id)) return;
      await apply();
      if (!_closed) _handled.add(id);
    });
    _tail = next.catchError((Object _) {});
    return next;
  }

  void close() {
    _closed = true;
    _handled.clear();
  }
}
