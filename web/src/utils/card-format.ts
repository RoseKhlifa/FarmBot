export interface CardDurationLike {
  days?: number
  value?: number
  durationValue?: number
  durationUnit?: 'hour' | 'day'
  durationMs?: number | null
  isPermanent?: boolean
}

function formatNumber(value: number) {
  if (Number.isInteger(value))
    return String(value)
  return Number(value.toFixed(2)).toString()
}

function formatDurationMs(durationMs: number) {
  const totalHours = Math.round(durationMs / 3600000)
  const days = Math.floor(totalHours / 24)
  const hours = totalHours % 24
  if (days > 0 && hours > 0)
    return `${days}天${hours}小时`
  if (days > 0)
    return `${days}天`
  if (hours > 0)
    return `${hours}小时`
  return '未激活'
}

export function getCardQuotaValue(card: CardDurationLike | null | undefined) {
  return Number(card?.value ?? card?.days ?? 0)
}

export function formatTimeDuration(card: CardDurationLike | null | undefined) {
  if (!card)
    return '无'
  if (card.isPermanent === true || card.days === -1 || card.durationValue === -1)
    return '永久'
  const durationMs = Number(card.durationMs)
  if (Number.isFinite(durationMs) && durationMs > 0)
    return formatDurationMs(durationMs)
  const durationValue = Number(card.durationValue)
  const durationUnit = card.durationUnit === 'hour' ? 'hour' : 'day'
  if (Number.isFinite(durationValue) && durationValue > 0)
    return `${formatNumber(durationValue)}${durationUnit === 'hour' ? '小时' : '天'}`
  const days = Number(card.days)
  if (Number.isFinite(days) && days > 0)
    return `${formatNumber(days)}天`
  return '未激活'
}

export function formatCardValue(card: CardDurationLike & { type?: 'time' | 'quota' }) {
  if (card.type === 'quota')
    return `+${formatNumber(getCardQuotaValue(card))}额度`
  return formatTimeDuration(card)
}
