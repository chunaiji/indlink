<template>
  <view class="pe">
    <!-- 注:编辑资料是工具形态 mine 页的常驻入口,不接整页覆盖 -->
    <view class="tip" v-if="firstTime">完善资料后,更容易在同城/扩列被有缘人看到 🌊</view>

    <!-- 头像 -->
    <view class="avrow">
      <!-- #ifdef MP-WEIXIN -->
      <button class="ava-btn" open-type="chooseAvatar" @chooseavatar="onChooseAvatar">
        <user-avatar :name="form.nickname || '我'" :src="form.avatar" :size="140" shape="circle" />
        <text class="avtip">{{ uploading ? '上传中…' : '点击更换头像' }}</text>
      </button>
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <view @tap="pickAvatar($event)">
        <user-avatar :name="form.nickname || '我'" :src="form.avatar" :size="140" shape="circle" />
        <text class="avtip">{{ uploading ? '上传中…' : '点击更换头像' }}</text>
      </view>
      <!-- #endif -->
    </view>

    <view class="card">
      <view class="row">
        <text class="lb">昵称</text>
        <!-- #ifdef MP-WEIXIN -->
        <input v-model="form.nickname" type="nickname" class="ipt" :maxlength="16" placeholder="点击填写昵称" placeholder-class="ph" @blur="e => { if (e.detail.value) form.nickname = e.detail.value }" />
        <!-- #endif -->
        <!-- #ifndef MP-WEIXIN -->
        <input v-model="form.nickname" class="ipt" :maxlength="16" placeholder="给自己取个名字" placeholder-class="ph" />
        <!-- #endif -->
      </view>

      <view class="row">
        <text class="lb">性别</text>
        <view class="seg" :class="{ locked: genderLocked }">
          <text :class="{ on: form.gender === 1 }" @tap="setGender(1)">♂ 男生</text>
          <text :class="{ on: form.gender === 2 }" @tap="setGender(2)">♀ 女生</text>
        </view>
        <text v-if="genderLocked" class="lock-tip">不可修改</text>
      </view>

      <view class="row">
        <text class="lb">年龄</text>
        <picker mode="selector" :range="ages" :value="ageIndex" @change="onAge($event)" style="flex: 1">
          <view class="pick">{{ form.age ? form.age + ' 岁' : '请选择' }} ›</view>
        </picker>
      </view>

      <view class="row last">
        <text class="lb">城市</text>
        <picker mode="region" :value="region" @change="onRegion($event)" style="flex: 1">
          <view class="pick">{{ form.city || '请选择所在城市' }} ›</view>
        </picker>
      </view>
    </view>

    <button class="save" :loading="saving" @tap="save($event)">保存资料</button>
  </view>
</template>

<script>
import { useUserStore } from '../../store/user'
import { chooseAndUploadImage } from '../../utils/upload'
import { BASE_URL } from '../../utils/config'

export default {
  data() {
    return {
      form: { nickname: '', avatar: '', gender: 0, age: 0, city: '' },
      region: ['', '', ''],
      ages: Array.from({ length: 53 }, (_, i) => i + 18), // 18~70
      uploading: false,
      saving: false,
      firstTime: false,
      genderLocked: false // 已设置过性别则锁定
    }
  },
  computed: {
    ageIndex() {
      const i = this.ages.indexOf(this.form.age)
      return i < 0 ? 0 : i
    }
  },
  onLoad(q) {
    this.firstTime = q && q.first === '1'
    const p = useUserStore().profile || {}
    this.form = {
      nickname: p.nickname || '',
      avatar: p.avatar || '',
      gender: p.gender || 0,
      age: p.age || 0,
      city: p.city || ''
    }
    this.genderLocked = !!p.gender // 已有性别 → 锁定不可改
  },
  methods: {
    async onChooseAvatar(e) {
      const filePath = e.detail.avatarUrl
      console.log('[avatar] chooseAvatar fired, avatarUrl=', filePath)
      if (!filePath) return

      // https/http CDN URL（微信头像直接返回网络地址）→ 直接存，无需上传
      if (filePath.startsWith('https://') || filePath.startsWith('http://')) {
        this.form.avatar = filePath
        console.log('[avatar] cdn url, set directly')
        return
      }

      // 本地路径（相册选图 / wxfile:// 临时文件）→ 上传到服务器
      this.uploading = true
      uni.showLoading({ title: '上传中', mask: true })
      try {
        await new Promise((resolve, reject) => {
          uni.uploadFile({
            url: BASE_URL + '/upload',
            filePath,
            name: 'file',
            header: { Authorization: 'Bearer ' + (uni.getStorageSync('token') || '') },
            success: (r) => {
              console.log('[avatar] upload response:', r.statusCode, r.data)
              let body = {}
              try { body = JSON.parse(r.data) } catch (_) {}
              if (body.code === 0 && body.data && body.data.url) {
                this.form.avatar = body.data.url
                resolve()
              } else {
                reject(new Error('server error: ' + r.data))
              }
            },
            fail: (err) => {
              console.log('[avatar] upload fail:', JSON.stringify(err))
              reject(err)
            }
          })
        })
      } catch (err) {
        console.log('[avatar] caught:', err && err.message)
        uni.showToast({ title: '头像上传失败', icon: 'none' })
      } finally {
        uni.hideLoading()
        this.uploading = false
      }
    },
    async pickAvatar() {
      if (this.uploading) return
      this.uploading = true
      try { this.form.avatar = await chooseAndUploadImage() } catch (e) {} finally { this.uploading = false }
    },
    setGender(g) {
      if (this.genderLocked) { uni.showToast({ title: '性别设置后不可修改', icon: 'none' }); return }
      this.form.gender = g
    },
    onAge(e) { this.form.age = this.ages[e.detail.value] },
    onRegion(e) {
      this.region = e.detail.value
      // 取「市」级;直辖市取省级
      this.form.city = (e.detail.value[1] || e.detail.value[0] || '').replace(/市$/, '')
    },
    async save() {
      if (!this.form.nickname.trim()) { uni.showToast({ title: '取个昵称吧', icon: 'none' }); return }
      if (!this.form.gender) { uni.showToast({ title: '选择一下性别', icon: 'none' }); return }
      this.saving = true
      try {
        await useUserStore().updateProfile({
          nickname: this.form.nickname.trim(),
          avatar: this.form.avatar,
          gender: this.form.gender,
          age: this.form.age,
          city: this.form.city
        })
        uni.setStorageSync('needProfile', '')
        uni.showToast({ title: '已保存' })
        setTimeout(() => uni.navigateBack(), 700)
      } catch (e) {
        uni.showToast({ title: '保存失败,稍后再试', icon: 'none' })
      } finally {
        this.saving = false
      }
    }
  }
}
</script>

<style lang="scss">
.pe { min-height: 100vh; background: linear-gradient(180deg, #EAF7FB, #DDEFF3); padding: 28rpx; }
.tip {
  background: rgba(255,107,91,0.1); color: $coral-deep; font-size: 24rpx; font-weight: 600;
  padding: 18rpx 24rpx; border-radius: $r-md; margin-bottom: 24rpx;
}
.avrow { display: flex; flex-direction: column; align-items: center; gap: 14rpx; padding: 20rpx 0 32rpx; }
.avtip { font-size: 24rpx; color: $ink-soft; }
.ava-btn {
  display: flex; flex-direction: column; align-items: center; gap: 14rpx;
  background: transparent; border: none; padding: 0; margin: 0; line-height: normal;
}
.ava-btn::after { border: none; }
.card { background: $card; border: 1rpx solid $line; border-radius: $r-lg; box-shadow: $shadow-card; overflow: hidden; }
.row {
  display: flex; align-items: center; min-height: 96rpx; padding: 0 28rpx; border-bottom: 1rpx solid $line;
  &.last { border-bottom: none; }
  .lb { width: 120rpx; font-size: 28rpx; font-weight: 600; color: $ink; flex: none; }
  .ipt { flex: 1; font-size: 28rpx; color: $ink; text-align: right; }
  .ph { color: $ink-faint; }
}
.seg {
  flex: 1; display: flex; justify-content: flex-end; gap: 16rpx;
  text {
    font-size: 26rpx; font-weight: 600; padding: 10rpx 26rpx; border-radius: 999rpx;
    background: #EEF4F6; color: $ink-soft;
    &.on { background: linear-gradient(135deg, $sea-2, $sea-3); color: #fff; }
  }
  &.locked { opacity: 0.55; }
  &.locked text:not(.on) { opacity: 0.6; }
}
.lock-tip { flex: none; margin-left: 12rpx; font-size: 20rpx; color: $ink-faint; }
.pick { flex: 1; text-align: right; font-size: 28rpx; color: $ink; }
.save {
  margin-top: 40rpx; height: 96rpx; line-height: 96rpx; border-radius: $r-lg;
  background: linear-gradient(135deg, $coral, $coral-deep); color: #fff; font-weight: 700; font-size: 32rpx;
  box-shadow: $shadow-pop;
}
</style>
