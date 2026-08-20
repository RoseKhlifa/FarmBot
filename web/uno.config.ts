import { defineConfig, presetAttributify, presetIcons, presetUno } from 'unocss'

export default defineConfig({
  content: {
    pipeline: {
      include: [
        /\.(vue|svelte|[jt]sx|mdx?|astro|elm|php|phtml|html)($|\?)/,
        'src/**/*.{js,ts}',
      ],
    },
  },
  theme: {
    fontFamily: {
      sans: '"HarmonyOS Sans SC", "鸿蒙黑体", "HarmonyOS Sans", "PingFang SC", "Microsoft YaHei", ui-sans-serif, system-ui, sans-serif',
      mono: '"JetBrains Mono", "JetBrains Mono NL", ui-monospace, SFMono-Regular, Consolas, "Liberation Mono", monospace',
    },
  },
  shortcuts: {
    'i-carbon-list-blocked': 'i-fas-ban',
    'i-carbon-user-offline': 'i-fas-user-slash',
    'i-carbon-leaf': 'i-fas-leaf',
    'i-carbon-crown': 'i-fas-crown',
    'i-carbon-circle': 'i-fas-circle',
    'i-fas-circle-notch-function': 'i-fas-circle-notch',
    'i-fas-circle-notch-string': 'i-fas-circle-notch',
    'i-fas-circle-notch-return': 'i-fas-circle-notch',
  },
  presets: [
    presetUno(),
    presetAttributify(),
    presetIcons({
      scale: 1.2,
      warn: true,
      safelist: [
        'i-fas-ban',
        'i-fas-user-slash',
        'i-fas-leaf',
        'i-fas-crown',
        'i-fas-circle',
        'i-fas-circle-notch',
      ],
      collections: {
        'carbon': () => import('@iconify-json/carbon/icons.json').then(i => i.default),
        'fas': () => import('@iconify-json/fa-solid/icons.json').then(i => i.default),
        'svg-spinners': () => import('@iconify-json/svg-spinners/icons.json').then(i => i.default),
      },
    }),
  ],
})
