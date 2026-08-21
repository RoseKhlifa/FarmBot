import type { Friend, InteractRecord } from '@/stores/friend'
import { formatGoldAmount } from '@/utils/number-format'

export function getFriendStatusText(friend: Friend): string {
  const plant = friend?.plant || {}
  const parts: string[] = []
  if (plant.stealNum)
    parts.push(`偷${plant.stealNum}`)
  if (plant.dryNum)
    parts.push(`水${plant.dryNum}`)
  if (plant.weedNum)
    parts.push(`草${plant.weedNum}`)
  if (plant.insectNum)
    parts.push(`虫${plant.insectNum}`)
  return parts.length ? parts.join(' ') : '无操作'
}

export function getFriendStatusHint(friend: Friend): string {
  const plant = friend?.plant || {}
  if (Number(plant.stealNum || 0) > 0)
    return `当前可偷 ${plant.stealNum} 块地，适合优先展开查看。`
  if (Number(plant.dryNum || 0) > 0 || Number(plant.weedNum || 0) > 0 || Number(plant.insectNum || 0) > 0)
    return '当前有可帮忙状态，可展开查看浇水、除草和除虫详情。'
  return '当前没有明显的手动互动提示，可先作为普通好友资料查看。'
}

export function getFriendLevel(friend: Friend): number {
  const level = Number.parseInt(String(friend?.level ?? ''), 10)
  return Number.isFinite(level) && level > 0 ? level : 0
}

export function getFriendGold(friend: Friend): number {
  const gold = Number.parseInt(String(friend?.gold ?? ''), 10)
  return Number.isFinite(gold) && gold >= 0 ? gold : 0
}

export function formatFriendGold(value: unknown): string {
  const gold = Number.parseInt(String(value ?? ''), 10)
  return Number.isFinite(gold) && gold >= 0 ? formatGoldAmount(gold) : '0'
}

export function getFriendAvatar(friend: Friend): string {
  const direct = String(friend?.avatarUrl || friend?.avatar_url || '').trim()
  if (direct)
    return direct
  const uin = String(friend?.uin || '').trim()
  return uin ? `https://q1.qlogo.cn/g?b=qq&nk=${uin}&s=100` : ''
}

export function getFriendAvatarKey(friend: Friend): string {
  return String(friend?.gid || friend?.uin || friend?.name || '').trim()
}

export function getInteractAvatar(record: InteractRecord): string {
  return String(record?.avatarUrl || '').trim()
}

export function getInteractAvatarKey(record: InteractRecord): string {
  const key = String(record?.visitorGid || record?.key || record?.nick || '').trim()
  return key ? `interact:${key}` : ''
}

export function getInteractBadgeClass(actionType: number): string {
  if (Number(actionType) === 1)
    return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  if (Number(actionType) === 2)
    return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (Number(actionType) === 3)
    return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
}

export function formatInteractTime(timestamp: number): string {
  const value = Number(timestamp)
  if (!Number.isFinite(value) || value <= 0)
    return '--'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
