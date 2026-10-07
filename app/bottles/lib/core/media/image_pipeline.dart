import 'dart:io';

import 'package:flutter_image_compress/flutter_image_compress.dart';

/// 压缩结果。原始大小与压缩后大小都留着——选图页要把这两个数字显示出来。
class CompressedImage {
  const CompressedImage({
    required this.path,
    required this.originalBytes,
    required this.compressedBytes,
  });

  final String path;
  final int originalBytes;
  final int compressedBytes;

  double get ratio => originalBytes == 0 ? 1 : compressedBytes / originalBytes;

  static String formatSize(int bytes) {
    if (bytes >= 1024 * 1024) {
      return '${(bytes / 1024 / 1024).toStringAsFixed(1)} MB';
    }
    return '${(bytes / 1024).round()} KB';
  }
}

/// 上传前的客户端压缩。
///
/// 后端 `upload` 目前落本地磁盘、`/static` 裸服务，**既不压缩也不出缩略图**，
/// 对印度用户首屏图片可能要等数秒。客户端先把 4MB 的原图压到几百 KB，
/// 是这条链路上现在就能做、且收益最大的一步。
///
/// 服务端压缩 + 对象存储 + 海外 CDN 仍然要做，这里只是把客户端该做的那半做完。
class ImagePipeline {
  const ImagePipeline._();

  /// 社交图片走 1080 长边 + JPEG 82，肉眼几乎无损，体积通常降到 1/8 以内。
  static const _minEdge = 1080;
  static const _quality = 82;

  static Future<CompressedImage> compress(String path) async {
    final source = File(path);
    final original = await source.length();

    try {
      final target = _targetPath(path);
      final out = await FlutterImageCompress.compressAndGetFile(
        path,
        target,
        minWidth: _minEdge,
        minHeight: _minEdge,
        quality: _quality,
        format: CompressFormat.jpeg,
      );
      if (out == null) return _asIs(path, original);
      final size = await File(out.path).length();
      // 压不动就用原图——有些图片（已经很小、或本身是 JPEG 低质量）压完反而更大。
      if (size >= original) return _asIs(path, original);
      return CompressedImage(
        path: out.path,
        originalBytes: original,
        compressedBytes: size,
      );
    } catch (_) {
      // 压缩失败不能挡住发布链路，退回原图继续。
      return _asIs(path, original);
    }
  }

  static Future<List<CompressedImage>> compressAll(List<String> paths) async {
    return Future.wait(paths.map(compress));
  }

  static CompressedImage _asIs(String path, int bytes) => CompressedImage(
        path: path,
        originalBytes: bytes,
        compressedBytes: bytes,
      );

  /// 输出到源文件同目录（image_picker 给的本来就是临时目录），不额外申请路径。
  static String _targetPath(String path) {
    final file = File(path);
    final stamp = DateTime.now().microsecondsSinceEpoch;
    return '${file.parent.path}${Platform.pathSeparator}drift_$stamp.jpg';
  }
}
