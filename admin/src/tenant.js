import { reactive } from 'vue'

// 全局租户上下文。所有 admin 页面共享此状态，api.js 自动注入 X-Tenant-ID header。
// 注意：tenant_id 是雪花 int64，超过 2^53 用 Number 会丢精度，故全程按字符串处理。
export const tenantStore = reactive({
  currentTenantID: localStorage.getItem('admin_tenant_id') || '',
  tenants: [],

  setTenant(id) {
    this.currentTenantID = String(id)
    localStorage.setItem('admin_tenant_id', String(id))
  },

  // 从 API 加载租户列表，并在未选租户时自动选第一个
  async load(listTenantsFunc) {
    try {
      const list = await listTenantsFunc()
      this.tenants = list || []
      if (!this.currentTenantID && this.tenants.length > 0) {
        this.setTenant(this.tenants[0].tenant_id)
      }
    } catch (e) {}
  }
})
