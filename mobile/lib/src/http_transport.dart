import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

class HttpBytes {
  const HttpBytes(this.statusCode, this.bytes);
  final int statusCode;
  final Uint8List bytes;
}

Future<HttpBytes> requestBytes({
  required HttpClient client,
  required String method,
  required Uri uri,
  required Duration timeout,
  Map<String, String> headers = const {},
  void Function(HttpClientRequest)? writeBody,
  int maxBytes = 16 * 1024 * 1024,
}) async {
  HttpClientRequest? request;
  StreamIterator<List<int>>? chunks;
  var expired = false;

  Future<HttpBytes> run() async {
    final opened = await client.openUrl(method, uri);
    request = opened;
    if (expired) {
      opened.abort();
      throw TimeoutException('Request timed out');
    }
    headers.forEach((name, value) => opened.headers.set(name, value));
    writeBody?.call(opened);
    final response = await opened.close();
    final iterator = StreamIterator<List<int>>(response);
    chunks = iterator;
    final bytes = BytesBuilder(copy: false);
    try {
      if (expired) throw TimeoutException('Request timed out');
      while (await iterator.moveNext()) {
        if (expired) throw TimeoutException('Request timed out');
        if (bytes.length + iterator.current.length > maxBytes) {
          throw const HttpException('Response exceeds the download limit');
        }
        bytes.add(iterator.current);
      }
      return HttpBytes(response.statusCode, bytes.takeBytes());
    } finally {
      await iterator.cancel();
    }
  }

  try {
    return await run().timeout(
      timeout,
      onTimeout: () {
        expired = true;
        request?.abort();
        final iterator = chunks;
        if (iterator != null)
          unawaited(iterator.cancel().catchError((Object _) {}));
        throw TimeoutException('Request timed out', timeout);
      },
    );
  } catch (_) {
    request?.abort();
    rethrow;
  }
}
