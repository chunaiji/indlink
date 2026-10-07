import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../core/providers.dart';
import '../features/auth/auth_controller.dart';
import '../features/auth/credential_flow_page.dart';
import '../features/auth/login_page.dart';
import '../features/auth/onboarding_page.dart';
import '../features/auth/otp_page.dart';
import '../features/auth/splash_page.dart';
import '../features/bottle/bottle_detail_page.dart';
import '../features/bottle/drift_map_page.dart';
import '../features/bottle/my_bottles_page.dart';
import '../features/bottle/ocean_page.dart';
import '../features/bottle/write_bottle_page.dart';
import '../features/chat/chat_list_page.dart';
import '../features/chat/chat_room_page.dart';
import '../features/discover/discover_page.dart';
import '../features/discover/filter_page.dart';
import '../features/discover/user_profile_page.dart';
import '../features/me/blocklist_page.dart';
import '../features/me/items_page.dart';
import '../features/me/legal_page.dart';
import '../features/me/log_viewer_page.dart';
import '../features/me/me_page.dart';
import '../features/me/notifications_page.dart';
import '../features/me/pay_records_page.dart';
import '../features/me/profile_edit_page.dart';
import '../features/me/payment_flow_page.dart';
import '../features/me/recharge_page.dart';
import '../features/me/voided_page.dart';
import '../domain/models/wallet.dart';
import '../features/me/relations_page.dart';
import '../features/me/rewards_page.dart';
import '../features/me/account_security_page.dart';
import '../features/me/settings_page.dart';
import '../features/me/wallet_page.dart';
import '../features/moment/moment_detail_page.dart';
import '../features/moment/moment_feed_page.dart';
import '../features/moment/post_moment_page.dart';
import '../core/logging/log_record.dart';
import '../core/logging/logger.dart';
import 'nav_logger.dart';
import 'routes.dart';
import 'shell.dart';

/// 根 Navigator 的 key。导出是为了全局弹窗(如火花)能拿到一个
/// 不依赖当前页面的 context。
final rootNavigatorKey = GlobalKey<NavigatorState>();
final _rootKey = rootNavigatorKey;

final routerProvider = Provider<GoRouter>((ref) {
  // go_router 只认 Listenable，把登录态变化桥接过去。
  final refresh = ValueNotifier<AuthStatus>(ref.read(authProvider).status);
  ref.listen<AuthState>(authProvider, (_, next) {
    refresh.value = next.status;
    Log.i(LogTag.auth, 'auth status', fields: {'status': next.status.name});
  });
  ref.onDispose(refresh.dispose);

  return GoRouter(
    navigatorKey: _rootKey,
    initialLocation: Routes.splash,
    refreshListenable: refresh,
    debugLogDiagnostics: false,
    observers: [NavLogger()],

    /// 登录态守卫。
    ///
    /// 启动 → A1 静默校验 JWT；未登录 → 登录页；新注册 → 完善资料两步引导；
    /// 已登录访问鉴权页 → 回海洋。
    redirect: (context, state) {
      final status = ref.read(authProvider).status;
      final loc = state.matchedLocation;

      switch (status) {
        case AuthStatus.checking:
          return loc == Routes.splash ? null : Routes.splash;

        case AuthStatus.unauthenticated:
          const authPages = [
            Routes.login,
            Routes.otp,
            Routes.register,
            Routes.forgotPassword,
          ];
          // 协议 / 隐私放行：勾选框旁边那两个链接必须在登录前就能点开。
          if (loc.startsWith(Routes.legalPrefix)) return null;
          return authPages.contains(loc) ? null : Routes.login;

        case AuthStatus.onboarding:
          return loc == Routes.onboarding ? null : Routes.onboarding;

        case AuthStatus.authenticated:
          // 找回密码不在这个名单里:「账号与安全 → 修改密码」走的就是它。
          // 以前在名单里,push 过去被 redirect 到 /ocean,等于把整个 Tab 壳当一页压在根导航器上——黑屏。
          const authPages = [
            Routes.splash,
            Routes.login,
            Routes.otp,
            Routes.register,
            Routes.onboarding,
          ];
          return authPages.contains(loc) ? Routes.ocean : null;
      }
    },

    routes: [
      GoRoute(path: Routes.splash, builder: (_, _) => const SplashPage()),
      GoRoute(
        path: Routes.login,
        builder: (_, _) => const LoginPage(),
        routes: [
          GoRoute(path: 'otp', builder: (_, _) => const OtpPage()),
          GoRoute(
            path: 'register',
            builder: (_, _) =>
                const CredentialFlowPage(flow: AuthFlow.register),
          ),
          GoRoute(
            path: 'forgot',
            builder: (_, _) => const CredentialFlowPage(flow: AuthFlow.reset),
          ),
        ],
      ),
      GoRoute(
        path: Routes.onboarding,
        builder: (_, _) => const OnboardingPage(),
      ),
      // 顶层：登录前后都要能打开，全屏盖住 Tab 栏。
      GoRoute(
        path: '/legal/:doc',
        builder: (_, state) =>
            LegalPage(doc: state.pathParameters['doc'] ?? 'terms'),
      ),

      // 5 个 Tab 固定常驻，各自保留滚动位置。
      //
      // Tab 根页之下的每一级都带 `parentNavigatorKey: _rootKey`：原型里只有
      // B1/C1/D1/F1/H1 五个根页带底部 Tab 栏，写瓶、详情、筛选、资料、发动态、
      // 选地点……全是盖住 Tab 栏的全屏页。少写一个，那一页就会在 Tab 栏上方
      // 少掉 65pt 可用高度，底部按钮也会挤在 Tab 栏上面（真机第二批截图全中）。
      // `test/app/router_test.dart` 守着这条。
      StatefulShellRoute.indexedStack(
        builder: (_, _, shell) => AppShell(navigationShell: shell),
        branches: [
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: Routes.ocean,
                builder: (_, _) => const OceanPage(),
                routes: [
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'write',
                    builder: (_, _) => const WriteBottlePage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'mine',
                    builder: (_, _) => const MyBottlesPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'bottle/:id',
                    builder: (_, state) => BottleDetailPage(
                      bottleId: state.pathParameters['id'] ?? '',
                    ),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'map/:id',
                    builder: (_, state) => DriftMapPage(
                      bottleId: state.pathParameters['id'] ?? '',
                    ),
                  ),
                ],
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: Routes.discover,
                builder: (_, _) => const DiscoverPage(),
                routes: [
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'filter',
                    builder: (_, _) => const FilterPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'user/:id',
                    builder: (_, state) => UserProfilePage(
                      userId: state.pathParameters['id'] ?? '',
                    ),
                  ),
                ],
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: Routes.chats,
                builder: (_, _) => const ChatListPage(),
                routes: [
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: ':id',
                    builder: (_, state) => ChatRoomPage(
                      conversationId: state.pathParameters['id'] ?? '',
                    ),
                  ),
                ],
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: Routes.moments,
                builder: (_, _) => const MomentFeedPage(),
                routes: [
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'post',
                    builder: (_, _) => const PostMomentPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: ':id',
                    builder: (_, state) => MomentDetailPage(
                      momentId: state.pathParameters['id'] ?? '',
                    ),
                  ),
                ],
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: Routes.me,
                builder: (_, _) => const MePage(),
                routes: [
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'wallet',
                    builder: (_, _) => const WalletPage(),
                    routes: [
                      GoRoute(
                        parentNavigatorKey: _rootKey,
                        path: 'voided',
                        builder: (_, _) => const VoidedPage(),
                      ),
                    ],
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'recharge',
                    builder: (_, _) => const RechargePage(),
                    routes: [
                      GoRoute(
                        parentNavigatorKey: _rootKey,
                        path: 'records',
                        builder: (_, _) => const PayRecordsPage(),
                      ),
                      GoRoute(
                        parentNavigatorKey: _rootKey,
                        path: 'pay/:orderNo',
                        builder: (_, s) => PaymentFlowPage(
                          orderNo: s.pathParameters['orderNo']!,
                          // 深链进来(比如从充值记录点一笔处理中的单)时没有 extra,
                          // 那笔单早就下过了,直接接管轮询即可。
                          pending: s.extra as PendingOrder?,
                          from: s.uri.queryParameters['from'],
                        ),
                      ),
                    ],
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'items',
                    builder: (_, _) => const ItemsPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'rewards',
                    builder: (_, _) => const RewardsPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'relations',
                    builder: (_, _) => const RelationsPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'notifications',
                    builder: (_, _) => const NotificationsPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'blocklist',
                    builder: (_, _) => const BlocklistPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'settings',
                    builder: (_, _) => const SettingsPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'profile',
                    builder: (_, _) => const ProfileEditPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'account-security',
                    builder: (_, _) => const AccountSecurityPage(),
                  ),
                  GoRoute(
                    parentNavigatorKey: _rootKey,
                    path: 'log-viewer',
                    builder: (_, _) => const LogViewerPage(),
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    ],
  );
});
