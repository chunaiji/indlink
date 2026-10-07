import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../core/design/tokens.dart';
import '../../core/media/image_pipeline.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers.dart';
import '../../core/utils/birthday.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/avatar.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/cards.dart';
import '../../ui/widgets/chips.dart';
import '../../ui/widgets/headers.dart';
import '../../ui/widgets/overlays.dart';
import '../auth/auth_controller.dart';

/// 编辑资料：头像、昵称、出生日期。
///
/// 性别不在这里——完善资料时定一次，服务端之后不放行修改（匹配 / 筛选都靠它）。
/// 年龄由出生日期推出，页面上不单独填。
/// 类别：表单页（规范 §6）
/// 固定区：NavBar、底部「保存」
/// 弹性区：无
/// 可滚动区：头像 + 昵称 + 出生日期
/// 键盘：昵称输入时整体可滚，底部按钮随 resizeToAvoidBottomInset 上移
class ProfileEditPage extends ConsumerStatefulWidget {
  const ProfileEditPage({super.key});

  @override
  ConsumerState<ProfileEditPage> createState() => _ProfileEditPageState();
}

class _ProfileEditPageState extends ConsumerState<ProfileEditPage> {
  late final TextEditingController _nickname;

  /// 新选的本地头像；null = 没动。
  String? _avatarPath;
  String? _birthday;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    final me = ref.read(authProvider).profile;
    _nickname = TextEditingController(text: me?.nickname ?? '');
    _birthday = me?.birthday;
  }

  @override
  void dispose() {
    _nickname.dispose();
    super.dispose();
  }

  Future<void> _pickAvatar() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery);
    if (picked == null) return;
    final compressed = await ImagePipeline.compress(picked.path);
    if (mounted) setState(() => _avatarPath = compressed.path);
  }

  Future<void> _pickBirthday() async {
    final now = DateTime.now();
    // 年龄下限来自后台 app_profile_min_age（与完善资料同一门槛），没拉到按 18。
    final minAge =
        ref.read(profileOptionsProvider).value?.minAge ??
        OnboardingDraft.minAge;
    final last = DateTime(now.year - minAge, now.month, now.day);
    final current = parseBirthday(_birthday) ?? DateTime(2000, 1, 1);
    final picked = await showDatePicker(
      context: context,
      initialDate: current.isAfter(last) ? last : current,
      firstDate: DateTime(1900),
      lastDate: last,
    );
    if (picked != null && mounted) {
      setState(() => _birthday = formatBirthday(picked));
    }
  }

  Future<void> _save() async {
    final l = L.of(context);
    final name = _nickname.text.trim();
    if (name.isEmpty) {
      showToast(context, l.profileEditNicknameRequired, error: true);
      return;
    }
    setState(() => _busy = true);
    try {
      // 头像先传成 URL 再提交；本地路径存进库换台手机就是破图。
      final avatarUrl = _avatarPath == null
          ? null
          : await ref.read(momentRepoProvider).uploadImage(_avatarPath!);
      final profile = await ref
          .read(authRepoProvider)
          .updateProfile(
            nickname: name,
            avatar: avatarUrl,
            birthday: _birthday,
          );
      ref.read(authProvider.notifier).updateProfile(profile);
      if (!mounted) return;
      showToast(context, l.profileEditSaved);
      Navigator.of(context).maybePop();
    } on ApiException catch (e) {
      if (mounted) showToast(context, e.message, error: true);
    } catch (_) {
      if (mounted) showToast(context, l.stateErrorTitle, error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final l = L.of(context);
    final me = ref.watch(authProvider).profile;

    return Scaffold(
      appBar: NavBar(title: l.profileEditTitle),
      body: Column(
        children: [
          Expanded(
            child: ListView(
              keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
              padding: const EdgeInsets.all(Dim.gutter),
              children: [
                const SizedBox(height: Dim.s2),
                Center(
                  child: GestureDetector(
                    key: const ValueKey('profile-avatar'),
                    onTap: _pickAvatar,
                    child: Stack(
                      clipBehavior: Clip.none,
                      children: [
                        AvatarRing(
                          avatar: _avatarPath ?? me?.brief.avatar,
                          seed: me?.id ?? '',
                          size: 96,
                        ),
                        // 右下角相机角标：告诉人这块能点。
                        Positioned(
                          right: -2,
                          bottom: -2,
                          child: Container(
                            width: 30,
                            height: 30,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: c.aqua,
                              border: Border.all(color: c.surface, width: 2),
                            ),
                            child: const Icon(
                              Icons.photo_camera_rounded,
                              size: 15,
                              color: Colors.white,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: Dim.s2),
                Text(
                  l.profileEditAvatarHint,
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: Dim.t1, color: c.ink3),
                ),
                const SizedBox(height: Dim.s5),
                SectionLabel(l.profileEditNickname),
                const SizedBox(height: Dim.s2),
                TextField(
                  key: const ValueKey('profile-nickname'),
                  controller: _nickname,
                  maxLength: 20,
                  textInputAction: TextInputAction.done,
                  decoration: InputDecoration(
                    hintText: l.profileNicknameHint,
                    counterText: '',
                  ),
                ),
                const SizedBox(height: Dim.s4),
                SectionLabel(l.profileEditBirthday),
                const SizedBox(height: Dim.s2),
                ValueField(
                  value: _birthday ?? l.profileEditBirthdayHint,
                  leading: Icons.cake_outlined,
                  onTap: _pickBirthday,
                ),
                const SizedBox(height: Dim.s2),
                Text(
                  l.profileEditBirthdayNote,
                  style: TextStyle(
                    fontSize: Dim.t1,
                    color: c.ink3,
                    height: 1.5,
                  ),
                ),
              ],
            ),
          ),
          BottomActionBar(
            child: AppButton(
              label: l.profileEditSave,
              loading: _busy,
              onTap: _busy ? null : _save,
            ),
          ),
        ],
      ),
    );
  }
}
