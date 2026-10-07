package com.ambertu.bottles

import android.content.Context
import android.os.Build
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

/**
 * 捞瓶仪式要「拉扯感」，靠 Flutter 的 HapticFeedback 做不到：
 * 它走的是 View.performHapticFeedback，在系统「触感反馈」关掉时**完全静默**，
 * 而且 CLOCK_TICK / KEYBOARD_TAP 这两档在多数真机上根本感觉不到。
 *
 * 这里直接驱动 Vibrator（需要 VIBRATE 权限），时长与强度都可控，
 * 不受触摸反馈开关影响。设备没有振动器时静默返回，不报错。
 */
class MainActivity : FlutterActivity() {

    private val channelName = "drift/haptics"

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "hasVibrator" -> result.success(vibrator()?.hasVibrator() ?: false)
                    "vibrate" -> {
                        val ms = (call.argument<Int>("ms") ?: 20).toLong()
                        // 1..255，越大越重；设备不支持强度控制时忽略。
                        val amplitude = (call.argument<Int>("amplitude") ?: -1)
                        vibrate(ms, amplitude)
                        result.success(null)
                    }
                    else -> result.notImplemented()
                }
            }
    }

    private fun vibrator(): Vibrator? =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            val manager =
                getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as? VibratorManager
            manager?.defaultVibrator
        } else {
            @Suppress("DEPRECATION")
            getSystemService(Context.VIBRATOR_SERVICE) as? Vibrator
        }

    private fun vibrate(ms: Long, amplitude: Int) {
        val v = vibrator() ?: return
        if (!v.hasVibrator()) return
        val amp = when {
            amplitude in 1..255 && v.hasAmplitudeControl() -> amplitude
            else -> VibrationEffect.DEFAULT_AMPLITUDE
        }
        v.vibrate(VibrationEffect.createOneShot(ms, amp))
    }
}
