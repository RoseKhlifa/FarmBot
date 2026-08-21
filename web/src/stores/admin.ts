import type { UserCard } from '@/stores/user'
import { defineStore } from 'pinia'
import { cardApi, loginLogApi, userApi } from '@/api'

export { formatCardValue, formatTimeDuration, getCardQuotaValue } from '@/utils/card-format'

export interface Card {
  code: string
  description: string
  days: number
  value?: number
  durationValue?: number
  durationUnit?: 'hour' | 'day'
  durationMs?: number | null
  isPermanent?: boolean
  type: 'time' | 'quota'
  enabled: boolean
  usedBy: string | null
  usedAt: number | null
  createdAt: number
}

export const useAdminStore = defineStore('admin', () => {
  async function getAllUsers() {
    const res = await userApi.getAdminUsers()
    return res.data
  }

  async function getLoginLogs(limit = 100, offset = 0) {
    const res = await loginLogApi.getLoginLogs(limit, offset)
    return res.data
  }

  async function clearLoginLogs(_payload?: { confirmed?: boolean, confirmText?: string }) {
    const res = await loginLogApi.clearLoginLogs()
    return res.data
  }

  async function clearExpiredUsers(payload?: { confirmed?: boolean, confirmText?: string }) {
    const res = await userApi.clearExpiredUsers(payload)
    return res.data
  }

  async function updateUser(username: string, updates: Partial<UserCard>, payload?: { confirmed?: boolean, confirmText?: string }) {
    const res = await userApi.updateAdminUser(username, { ...updates, ...payload })
    return res.data
  }

  async function editUser(username: string, payload: Record<string, unknown>) {
    const res = await userApi.editAdminUser(username, payload)
    return res.data
  }

  async function deleteUser(username: string, _payload?: { confirmed?: boolean, confirmText?: string }) {
    const res = await userApi.deleteAdminUser(username)
    return res.data
  }

  async function renewUser(username: string, cardCode: string, _payload?: { confirmed?: boolean, confirmText?: string }) {
    const res = await userApi.renewAdminUser(username, cardCode)
    return res.data
  }

  async function getAllCards() {
    const res = await cardApi.getCards()
    return res.data
  }

  async function createCard(
    description: string,
    days: number,
    count?: number,
    type?: 'time' | 'quota',
    payload?: {
      confirmed?: boolean
      confirmText?: string
      durationValue?: number
      durationUnit?: 'hour' | 'day'
      value?: number
    },
  ) {
    const res = await cardApi.createCard({ description, days, count, type, ...payload })
    return res.data
  }

  async function updateCard(code: string, updates: Partial<Card>, payload?: { confirmed?: boolean, confirmText?: string }) {
    const res = await cardApi.updateCard(code, { ...updates, ...payload })
    return res.data
  }

  async function deleteCard(code: string, _payload?: { confirmed?: boolean, confirmText?: string }) {
    const res = await cardApi.deleteCard(code)
    return res.data
  }

  async function deleteCardsBatch(codes: string[], payload?: { confirmed?: boolean, confirmText?: string }) {
    const res = await cardApi.batchDeleteCards({ codes, ...payload })
    return res.data
  }

  async function getCardClaimStatus() {
    const res = await cardApi.getClaimStatus()
    return res.data
  }

  async function updateCardClaimStatus(payload: { enabled: boolean, confirmed?: boolean, confirmText?: string }) {
    const res = await cardApi.updateClaimStatus(payload)
    return res.data
  }

  return {
    getAllUsers,
    getLoginLogs,
    clearLoginLogs,
    clearExpiredUsers,
    updateUser,
    editUser,
    deleteUser,
    renewUser,
    getAllCards,
    createCard,
    updateCard,
    deleteCard,
    deleteCardsBatch,
    getCardClaimStatus,
    updateCardClaimStatus,
  }
})
