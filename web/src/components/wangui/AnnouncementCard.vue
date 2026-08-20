<script setup lang="ts">
import { AlertOctagon, AlertTriangle, CheckCircle2, Info } from 'lucide-vue-next'
import { computed } from 'vue'
// Renders one announcement using the fixed template — same look on the
// user Dashboard AND inside the admin editor's preview pane. The author
// only fills title + content + level; everything else is decoration.
//
// Content gets a tiny inline-markdown pass:
//   **bold**     → <strong>
//   *italic*     → <em>
//   [text](url)  → <a target="_blank" rel="noopener">
//   newline      → preserved (whitespace-pre-wrap)
// No raw HTML; angle brackets are escaped first so admin can't accidentally
// break the page layout by pasting <div>s.

const props = withDefaults(
  defineProps<{
    a: FarmAnnouncement
    /** Show updated-at and the expiry hint. Admin list wants extra detail; the
     *  Dashboard panel prefers a cleaner card. */
    detailed?: boolean
    /** Compact mode: smaller padding/typography, no footer hairline. Used in
     *  the sidebar where horizontal space is tight (~232px content width). */
    compact?: boolean
  }>(),
  { detailed: false, compact: false },
)

function formatDateTime(value: number | string | null | undefined): string {
  if (value === null || value === undefined || value === '')
    return '-'
  const numeric = typeof value === 'number' ? value : Number(value)
  const date = new Date(Number.isFinite(numeric) ? (numeric < 10_000_000_000 ? numeric * 1000 : numeric) : value)
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString('zh-CN', { hour12: false })
}

interface FarmAnnouncement {
  id: string | number
  title: string
  content: string
  level: 'info' | 'success' | 'warning' | 'critical'
  createdAt: number | string
  expiresAt?: number | string | null
}

type AnnouncementLevel = FarmAnnouncement['level']

interface LevelStyle {
  icon: any
  ringClass: string
  bgClass: string
  titleClass: string
  badgeClass: string
  badgeLabel: string
  iconClass: string
}

const LEVEL_STYLES: Record<AnnouncementLevel, LevelStyle> = {
  info: {
    icon: Info,
    ringClass: 'ring-blue-500/30',
    bgClass: 'bg-blue-500/[0.07]',
    titleClass: 'text-blue-700 dark:text-blue-300',
    badgeClass: 'bg-blue-500/15 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/30',
    badgeLabel: '通知',
    iconClass: 'text-blue-500 dark:text-blue-400',
  },
  success: {
    icon: CheckCircle2,
    ringClass: 'ring-emerald-500/30',
    bgClass: 'bg-emerald-500/[0.07]',
    titleClass: 'text-emerald-700 dark:text-emerald-300',
    badgeClass: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 ring-1 ring-emerald-500/30',
    badgeLabel: '喜报',
    iconClass: 'text-emerald-500 dark:text-emerald-400',
  },
  warning: {
    icon: AlertTriangle,
    ringClass: 'ring-amber-500/30',
    bgClass: 'bg-amber-500/[0.07]',
    titleClass: 'text-amber-700 dark:text-amber-300',
    badgeClass: 'bg-amber-500/15 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30',
    badgeLabel: '注意',
    iconClass: 'text-amber-500 dark:text-amber-400',
  },
  critical: {
    icon: AlertOctagon,
    ringClass: 'ring-red-500/40',
    bgClass: 'bg-red-500/[0.07]',
    titleClass: 'text-red-700 dark:text-red-300',
    badgeClass: 'bg-red-500/15 text-red-700 dark:text-red-300 ring-1 ring-red-500/30',
    badgeLabel: '紧急',
    iconClass: 'text-red-500 dark:text-red-400',
  },
}

const style = computed<LevelStyle>(() => LEVEL_STYLES[props.a.level] ?? LEVEL_STYLES.info)

// HTML escape for safety. Admin is trusted but typos shouldn't break the
// page either. Run this BEFORE the markdown inline pass so any "**" inside
// pre-existing tags is irrelevant (there shouldn't be any tags).
function escape(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

// Inline markdown subset. Order matters: links first (so their text isn't
// stripped of asterisks), then bold (** before *), then italic.
function renderInline(text: string): string {
  let s = escape(text)
  s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, (_m, label, url) => {
    return `<a href="${url}" target="_blank" rel="noopener noreferrer" class="text-emerald-500 dark:text-emerald-300 underline decoration-dotted underline-offset-2 hover:text-emerald-400">${label}</a>`
  })
  s = s.replace(/\*\*([^*]+)\*\*/g, '<strong class="font-semibold">$1</strong>')
  s = s.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>')
  return s
}

// Content is split on blank lines into paragraphs; single newlines are
// preserved as <br> within a paragraph. Matches how most chat apps render.
const contentHTML = computed(() => {
  const paragraphs = props.a.content.split(/\n\s*\n/)
  return paragraphs
    .map((p) => {
      const inner = p
        .split('\n')
        .map(line => renderInline(line))
        .join('<br>')
      return `<p>${inner}</p>`
    })
    .join('')
})

const isExpired = computed(() => {
  if (!props.a.expiresAt)
    return false
  const value = typeof props.a.expiresAt === 'number' ? props.a.expiresAt : Number(props.a.expiresAt)
  const timestamp = value < 10_000_000_000 ? value * 1000 : value
  return Number.isFinite(timestamp) && timestamp < Date.now()
})
</script>

<template>
  <article
    class="overflow-hidden ring-1 transition-colors"
    :class="[
      style.ringClass,
      style.bgClass,
      isExpired ? 'opacity-50' : '',
      compact ? 'rounded-lg' : 'rounded-2xl',
    ]"
  >
    <!-- Header strip: icon + level badge + title -->
    <header
      class="flex items-start gap-2"
      :class="compact ? 'px-3 pt-2.5 pb-1.5' : 'px-5 pt-4 pb-3 gap-3'"
    >
      <component
        :is="style.icon"
        class="shrink-0"
        :class="[style.iconClass, compact ? 'w-3.5 h-3.5 mt-0.5' : 'w-5 h-5 mt-0.5']"
      />
      <div class="min-w-0 flex-1">
        <div class="mb-1 flex flex-wrap items-center gap-1.5">
          <span
            class="inline-flex items-center rounded font-medium tracking-wide"
            :class="[style.badgeClass, compact ? 'px-1 py-0 text-[9px]' : 'px-1.5 py-0.5 text-[10px]']"
          >
            {{ style.badgeLabel }}
          </span>
          <span
            class="font-mono-token text-zinc-500 tabular-nums"
            :class="compact ? 'text-[9px]' : 'text-[10px]'"
          >
            {{ compact ? formatDateTime(a.createdAt).slice(5, 16) : formatDateTime(a.createdAt) }}
          </span>
          <span
            v-if="detailed && a.expiresAt"
            class="text-[10px] text-zinc-500 tabular-nums"
            :title="`截止: ${formatDateTime(a.expiresAt)}`"
          >
            · 截止 {{ formatDateTime(a.expiresAt).slice(0, 10) }}
          </span>
          <span
            v-if="isExpired"
            class="inline-flex items-center rounded bg-zinc-500/15 px-1.5 py-0.5 text-[10px] text-zinc-500 font-medium ring-1 ring-zinc-500/25"
          >
            已过期
          </span>
        </div>
        <h3
          class="break-words font-bold tracking-tight"
          :class="[style.titleClass, compact ? 'text-xs leading-snug' : 'text-base sm:text-lg']"
        >
          {{ a.title }}
        </h3>
      </div>
    </header>

    <!-- Body: content with the inline-markdown subset rendered -->
    <div :class="compact ? 'px-3 pb-3' : 'px-5 pb-4 sm:pb-5'">
      <div
        class="announcement-prose text-zinc-700 leading-relaxed dark:text-zinc-300"
        :class="compact ? 'text-[11px] leading-relaxed' : 'text-sm'"
        v-html="contentHTML"
      />
    </div>

    <!-- Footer hairline accent in the level's color so cards stack
         distinctively. Skipped in compact mode to save vertical space. -->
    <div
      v-if="!compact"
      class="h-1"
      :class="a.level === 'info'
        ? 'bg-gradient-to-r from-transparent via-blue-500/40 to-transparent'
        : a.level === 'success'
          ? 'bg-gradient-to-r from-transparent via-emerald-500/40 to-transparent'
          : a.level === 'warning'
            ? 'bg-gradient-to-r from-transparent via-amber-500/40 to-transparent'
            : 'bg-gradient-to-r from-transparent via-red-500/40 to-transparent'"
    />
  </article>
</template>

<style scoped>
/* Prose-like spacing inside the rendered HTML content. Scoped to the card
   so the styles can't leak into surrounding text. */
.announcement-prose :deep(p) {
  margin: 0;
}
.announcement-prose :deep(p + p) {
  margin-top: 0.65rem;
}
.announcement-prose :deep(strong) {
  color: inherit;
}
.announcement-prose :deep(a) {
  word-break: break-all;
}
</style>
