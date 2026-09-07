import 'package:flutter/widgets.dart';

import 'src/app.dart';
import 'src/app_notifications.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await AppNotifications.init();
  runApp(const ChakuChuriMobileApp());
}
