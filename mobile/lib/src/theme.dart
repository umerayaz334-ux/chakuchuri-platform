import 'package:flutter/material.dart';

class AppColors {
  static const ink = Color(0xFF14201B);
  static const charcoal = Color(0xFF202A25);
  static const primary = Color(0xFF0B6B53);
  static const primaryDark = Color(0xFF07513F);
  static const blue = Color(0xFF216483);
  static const amber = Color(0xFFD18416);
  static const red = Color(0xFFC43D3D);
  static const paper = Color(0xFFF3F7F5);
  static const surface = Color(0xFFFFFFFF);
  static const surfaceSoft = Color(0xFFEDF3F0);
  static const line = Color(0xFFD8E3DE);
  static const muted = Color(0xFF5F6F68);

  // Compatibility aliases used throughout the existing feature screens.
  static const navy = blue;
  static const green = primary;
  static const orange = amber;
}

ThemeData buildTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: AppColors.primary,
    brightness: Brightness.light,
    primary: AppColors.primary,
    secondary: AppColors.blue,
    surface: AppColors.surface,
    error: AppColors.red,
  );
  final baseText = ThemeData.light().textTheme.apply(
    bodyColor: AppColors.ink,
    displayColor: AppColors.ink,
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: AppColors.paper,
    splashFactory: InkSparkle.splashFactory,
    textTheme: baseText.copyWith(
      displaySmall: baseText.displaySmall?.copyWith(
        fontSize: 32,
        height: 1.08,
        fontWeight: FontWeight.w800,
        letterSpacing: 0,
      ),
      headlineSmall: baseText.headlineSmall?.copyWith(
        fontSize: 24,
        height: 1.15,
        fontWeight: FontWeight.w800,
        letterSpacing: 0,
      ),
      titleLarge: baseText.titleLarge?.copyWith(
        fontSize: 20,
        height: 1.2,
        fontWeight: FontWeight.w800,
        letterSpacing: 0,
      ),
      titleMedium: baseText.titleMedium?.copyWith(
        fontSize: 16,
        height: 1.25,
        fontWeight: FontWeight.w700,
        letterSpacing: 0,
      ),
      bodyLarge: baseText.bodyLarge?.copyWith(
        fontSize: 16,
        height: 1.45,
        letterSpacing: 0,
      ),
      bodyMedium: baseText.bodyMedium?.copyWith(
        fontSize: 14,
        height: 1.4,
        letterSpacing: 0,
      ),
      labelLarge: baseText.labelLarge?.copyWith(
        fontSize: 14,
        fontWeight: FontWeight.w700,
        letterSpacing: 0,
      ),
    ),
    appBarTheme: const AppBarTheme(
      backgroundColor: Colors.white,
      foregroundColor: AppColors.ink,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: false,
      toolbarHeight: 64,
      titleSpacing: 16,
    ),
    cardTheme: CardThemeData(
      color: AppColors.surface,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(8),
        side: const BorderSide(color: AppColors.line),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: AppColors.surface,
      labelStyle: const TextStyle(color: AppColors.muted),
      hintStyle: const TextStyle(color: Color(0xFF8A9690)),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: AppColors.line),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: AppColors.line),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: AppColors.primary, width: 1.5),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: AppColors.red),
      ),
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: AppColors.primary,
        foregroundColor: Colors.white,
        minimumSize: const Size(48, 48),
        padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 13),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        textStyle: const TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.w800,
          letterSpacing: 0,
        ),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: AppColors.ink,
        minimumSize: const Size(48, 48),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 13),
        side: const BorderSide(color: AppColors.line),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        textStyle: const TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.w700,
          letterSpacing: 0,
        ),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        foregroundColor: AppColors.primary,
        textStyle: const TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.w700,
          letterSpacing: 0,
        ),
      ),
    ),
    iconButtonTheme: IconButtonThemeData(
      style: IconButton.styleFrom(
        foregroundColor: AppColors.charcoal,
        minimumSize: const Size.square(44),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
    ),
    navigationBarTheme: NavigationBarThemeData(
      height: 70,
      elevation: 0,
      backgroundColor: Colors.white,
      surfaceTintColor: Colors.transparent,
      indicatorColor: AppColors.surfaceSoft,
      indicatorShape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(8),
      ),
      labelTextStyle: WidgetStateProperty.resolveWith((states) {
        return TextStyle(
          color: states.contains(WidgetState.selected)
              ? AppColors.primaryDark
              : AppColors.muted,
          fontSize: 11,
          fontWeight: states.contains(WidgetState.selected)
              ? FontWeight.w800
              : FontWeight.w600,
          letterSpacing: 0,
        );
      }),
    ),
    segmentedButtonTheme: SegmentedButtonThemeData(
      style: ButtonStyle(
        minimumSize: const WidgetStatePropertyAll(Size(48, 44)),
        shape: WidgetStatePropertyAll(
          RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        ),
        side: const WidgetStatePropertyAll(BorderSide(color: AppColors.line)),
        textStyle: const WidgetStatePropertyAll(
          TextStyle(fontWeight: FontWeight.w700, letterSpacing: 0),
        ),
      ),
    ),
    dividerTheme: const DividerThemeData(
      color: AppColors.line,
      thickness: 1,
      space: 1,
    ),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: AppColors.charcoal,
      contentTextStyle: const TextStyle(color: Colors.white),
      behavior: SnackBarBehavior.floating,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
    ),
    bottomSheetTheme: const BottomSheetThemeData(
      backgroundColor: AppColors.surface,
      surfaceTintColor: Colors.transparent,
      showDragHandle: true,
    ),
  );
}
