import { defineStore } from 'pinia'
import { get } from '../utils/request'

export const useWalletStore = defineStore('wallet', {
  state: () => ({
    balance: 0
  }),
  actions: {
    async fetchBalance() {
      const res = await get('/wallet/balance')
      this.balance = res.balance || 0
      return this.balance
    },
    setBalance(v) {
      this.balance = v
    }
  }
})
