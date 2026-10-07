import '../../core/utils/json_parse.dart';
import 'chat.dart';
import 'user.dart';

class Moment {
  const Moment({
    required this.id,
    required this.author,
    required this.text,
    required this.createdAt,
    this.images = const [],
    this.city,
    this.likeCount = 0,
    this.commentCount = 0,
    this.liked = false,
    this.following = false,
  });

  final String id;
  final UserBrief author;
  final String text;
  final DateTime createdAt;

  /// `Moment.Images` 是 JSON 数组，最多 9 张。
  final List<String> images;

  final String? city;
  final int likeCount;
  final int commentCount;
  final bool liked;
  final bool following;

  Moment copyWith({bool? liked, int? likeCount, int? commentCount, bool? following}) {
    return Moment(
      id: id,
      author: author,
      text: text,
      createdAt: createdAt,
      images: images,
      city: city,
      likeCount: likeCount ?? this.likeCount,
      commentCount: commentCount ?? this.commentCount,
      liked: liked ?? this.liked,
      following: following ?? this.following,
    );
  }

  factory Moment.fromJson(Map<String, dynamic> j) => Moment(
        id: idOf(j['id']),
        author: UserBrief.fromJson(
          (j['author'] ?? j['user'] ?? const <String, dynamic>{})
              as Map<String, dynamic>,
        ),
        text: (j['content'] ?? '') as String,
        createdAt: utcOrEpoch(j['created_at']),
        images: stringList(j['images']),
        city: j['city'] as String?,
        likeCount: intOf(j['like_count']),
        commentCount: intOf(j['comment_count']),
        liked: j['liked'] == true,
        following: j['following'] == true,
      );
}

class MomentComment {
  const MomentComment({
    required this.id,
    required this.author,
    required this.createdAt,
    this.text = '',
    this.gift,
  });

  final String id;
  final UserBrief author;
  final DateTime createdAt;
  final String text;

  /// `MomentComment.Type=gift` 时 content 存 JSON `{name,icon,coins}`。
  final GiftItem? gift;

  bool get isGift => gift != null;

  factory MomentComment.fromJson(Map<String, dynamic> j) {
    final giftJson = j['gift'] as Map<String, dynamic>?;
    return MomentComment(
      id: idOf(j['id']),
      author: UserBrief.fromJson(
        (j['author'] ?? const <String, dynamic>{}) as Map<String, dynamic>,
      ),
      createdAt: utcOrEpoch(j['created_at']),
      text: (j['content'] ?? '') as String,
      gift: giftJson == null ? null : GiftItem.fromJson(giftJson),
    );
  }
}
