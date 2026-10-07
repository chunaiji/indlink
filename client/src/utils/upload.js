// 图片上传封装:选图 → 上传到后端本地图片服务 → 返回可访问 URL。
import { BASE_URL } from './config'

// 上传单个本地文件,resolve(url)。
export function uploadFilePath(filePath) {
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: BASE_URL + '/upload',
      filePath,
      name: 'file',
      header: { Authorization: 'Bearer ' + (uni.getStorageSync('token') || '') },
      success: (r) => {
        let body = {}
        try { body = JSON.parse(r.data) } catch (e) {}
        if (body.code === 0 && body.data && body.data.url) resolve(body.data.url)
        else reject(new Error((body && body.msg) || '上传失败'))
      },
      fail: () => reject(new Error('上传失败'))
    })
  })
}

// 选多张图并逐张上传,resolve(url[])。count 为剩余可选数。
export function chooseAndUploadImages(count = 9) {
  return new Promise((resolve, reject) => {
    uni.chooseImage({
      count,
      sizeType: ['compressed'],
      success: async (res) => {
        const paths = res.tempFilePaths || []
        if (!paths.length) { resolve([]); return }
        uni.showLoading({ title: '上传中', mask: true })
        try {
          const urls = []
          for (const p of paths) urls.push(await uploadFilePath(p)) // 顺序传,保持选图顺序
          uni.hideLoading()
          resolve(urls)
        } catch (e) {
          uni.hideLoading()
          uni.showToast({ title: e.message || '上传失败', icon: 'none' })
          reject(e)
        }
      },
      fail: reject
    })
  })
}

// 选一张图并上传,resolve(url)。
export function chooseAndUploadImage() {
  return new Promise((resolve, reject) => {
    uni.chooseImage({
      count: 1,
      sizeType: ['compressed'],
      success: (res) => {
        const filePath = res.tempFilePaths && res.tempFilePaths[0]
        if (!filePath) { reject(new Error('未选择图片')); return }
        uni.showLoading({ title: '上传中', mask: true })
        uni.uploadFile({
          url: BASE_URL + '/upload',
          filePath,
          name: 'file',
          header: { Authorization: 'Bearer ' + (uni.getStorageSync('token') || '') },
          success: (r) => {
            uni.hideLoading()
            let body = {}
            try { body = JSON.parse(r.data) } catch (e) {}
            if (body.code === 0 && body.data && body.data.url) {
              resolve(body.data.url)
            } else {
              uni.showToast({ title: (body && body.msg) || '上传失败', icon: 'none' })
              reject(body)
            }
          },
          fail: (e) => {
            uni.hideLoading()
            uni.showToast({ title: '上传失败', icon: 'none' })
            reject(e)
          }
        })
      },
      fail: reject
    })
  })
}
