import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/payment_status.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
      'status ring is 77pt and hides the spinner when animations are disabled',
      (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: const MediaQuery(
        data: MediaQueryData(disableAnimations: true),
        child: Scaffold(
          body: PaymentStatusView(
            kind: PaymentStatusKind.pending,
            title: 't',
            body: 'b',
          ),
        ),
      ),
    ));
    expect(t.getSize(find.byKey(const ValueKey('pst-ring'))).width, 77);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });

  testWidgets('ok state shows the amount in gold and the order id chip',
      (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: const Scaffold(
        body: PaymentStatusView(
          kind: PaymentStatusKind.ok,
          title: '到账',
          body: 'b',
          amount: '+600',
          orderNo: 'PO20261001',
        ),
      ),
    ));
    expect(find.text('+600'), findsOneWidget);
    expect(find.text('PO20261001'), findsOneWidget);
    expect(t.widget<Text>(find.text('+600')).style!.fontSize, 39);
  });
}
