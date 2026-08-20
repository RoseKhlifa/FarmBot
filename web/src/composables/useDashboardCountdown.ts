/* eslint-disable style/max-statements-per-line */
import { onScopeDispose, ref } from 'vue'

export function useDashboardCountdown(isDisconnected: () => boolean) {
  const nextFarmCheck = ref('--:--:--'); const nextHelpCheck = ref('--:--:--'); const nextStealCheck = ref('--:--:--'); const localUptime = ref(0)
  const farmPct = ref(0); const helpPct = ref(0); const stealPct = ref(0)
  let farm = 0; let help = 0; let steal = 0; let farmTotal = 120; let helpTotal = 180; let stealTotal = 120; let raf = 0
  const formatDuration = (seconds: number) => { const s = Math.max(0, Math.floor(seconds)); const pad = (n: number) => String(n).padStart(2, '0'); return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}` }
  const update = () => { if (isDisconnected()) { nextFarmCheck.value = '账号未登录'; nextHelpCheck.value = '账号未登录'; nextStealCheck.value = '账号未登录'; return }; localUptime.value++; farm = Math.max(0, farm - 1); help = Math.max(0, help - 1); steal = Math.max(0, steal - 1); nextFarmCheck.value = farm ? formatDuration(farm) : '检查中...'; nextHelpCheck.value = help ? formatDuration(help) : '检查中...'; nextStealCheck.value = steal ? formatDuration(steal) : '检查中...' }
  const setFromStatus = (status: any) => {
    const checks = status?.nextChecks; if (checks) {
      const f = Number(checks.farmRemainSec) || 0; const h = Number(checks.helpRemainSec) || 0; const s = Number(checks.stealRemainSec) || 0; if (f > farm)
        farmTotal = f; if (h > help)
        helpTotal = h; if (s > steal)
        stealTotal = s; farm = f; help = h; steal = s; update()
    }; if (status?.uptime !== undefined)
      localUptime.value = Number(status.uptime) || 0
  }
  const reset = () => { farm = 0; help = 0; steal = 0; localUptime.value = 0; nextFarmCheck.value = '--:--:--'; nextHelpCheck.value = '--:--:--'; nextStealCheck.value = '--:--:--' }
  const animate = () => { farmPct.value = farmTotal ? farm / farmTotal : 0; helpPct.value = helpTotal ? help / helpTotal : 0; stealPct.value = stealTotal ? steal / stealTotal : 0; raf = requestAnimationFrame(animate) }
  if (typeof window !== 'undefined')
    raf = requestAnimationFrame(animate)
  onScopeDispose(() => {
    if (raf)
      cancelAnimationFrame(raf)
  })
  return { nextFarmCheck, nextHelpCheck, nextStealCheck, localUptime, farmPct, helpPct, stealPct, formatDuration, update, setFromStatus, reset }
}
