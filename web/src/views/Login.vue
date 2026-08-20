<script setup lang="ts">
import { provide, toRefs } from 'vue'
import LoginModals from '@/components/login/LoginModals.vue'
import PasswordStrengthMeter from '@/components/login/PasswordStrengthMeter.vue'
import UpdateLogModal from '@/components/login/UpdateLogModal.vue'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'
import { authFlowKey, useAuthFlow } from '@/composables/useAuthFlow'

const authFlow = useAuthFlow()
provide(authFlowKey, authFlow)

const {
  gameVersion,
  loginLinks,
  showUpdateLog,
  logoLoadFailed,
  isLogin,
  username,
  password,
  cardCode,
  error,
  success,
  loading,
  showPasswordStrength,
  lockoutRemaining,
  rateLimitRemaining,
  cardClaimEnabled,
  cardClaimLoading,
  passwordStrength,
  usernameValid,
} = toRefs(authFlow)

const { handleSubmit, toggleMode, openResetVerifyModal, openRenewal, claimFreeCard } = authFlow
</script>

<template>
  <div class="auth-layout">
    <section class="auth-showcase" aria-label="FarmBot">
      <div class="showcase-topline">
        <span class="showcase-brand-mark"><span class="i-carbon-sprout" /></span>
        <span>FARMBOT / CONTROL CENTER</span>
      </div>

      <div class="showcase-copy">
        <span class="showcase-kicker">AUTOMATION WORKSPACE</span>
        <h1>把农场交给<br><em>可靠的流程。</em></h1>
        <p>从账号状态到每日任务，集中在一个清晰、安静的工作台里。</p>
      </div>

      <div class="showcase-board" aria-hidden="true">
        <div class="board-toolbar">
          <span /><span /><span /><b>LIVE SYSTEM MAP</b>
        </div>
        <div class="board-grid">
          <div class="board-column board-column--wide">
            <i /><i /><i /><i />
          </div>
          <div class="board-column">
            <i /><i /><i />
          </div>
          <div class="board-column board-column--accent">
            <i /><i /><i /><i />
          </div>
        </div>
        <div class="board-footer">
          <span class="status-dot status-dot--live" /> 运行环境已准备
        </div>
      </div>

      <div class="showcase-footer">
        <span>QQ FARM AUTOMATION</span>
        <span>v{{ gameVersion || '2.2' }}</span>
      </div>
    </section>

    <main class="auth-panel">
      <div class="auth-panel-top">
        <span class="auth-panel-label">账号访问</span>
        <span class="auth-panel-meta"><span class="status-dot status-dot--live" /> SECURE SESSION</span>
      </div>

      <div class="auth-content">
        <div class="auth-logo-row">
          <div class="auth-logo">
            <img
              v-if="loginLinks.logoUrl && !logoLoadFailed"
              :src="loginLinks.logoUrl"
              :alt="`${loginLinks.title || 'QQ农场智能助手'}图标`"
              @error="logoLoadFailed = true"
            >
            <span v-else class="i-carbon-sprout" />
          </div>
          <div>
            <span class="auth-logo-name">{{ loginLinks.title || 'QQ农场智能助手' }}</span>
            <span class="auth-logo-caption">FARMBOT WORKSPACE</span>
          </div>
        </div>

        <div class="auth-heading">
          <span class="showcase-kicker">{{ isLogin ? 'WELCOME BACK' : 'NEW WORKSPACE' }}</span>
          <h2>{{ isLogin ? '欢迎回来' : '创建账号' }}</h2>
          <p>{{ isLogin ? (loginLinks.loginSubtitle || '登录后继续管理你的农场') : (loginLinks.registerSubtitle || '创建账号，开始使用 FarmBot') }}</p>
        </div>

        <form class="auth-form" @submit.prevent="handleSubmit">
          <div class="field-group">
            <label for="username">用户名</label>
            <div class="field-control">
              <span class="field-icon i-carbon-user" />
              <BaseInput id="username" v-model="username" type="text" placeholder="请输入用户名" required />
            </div>
            <p v-if="username && !usernameValid.valid" class="field-hint field-hint--error">
              {{ usernameValid.message }}
            </p>
          </div>

          <div class="field-group">
            <label for="password">密码</label>
            <div class="field-control">
              <span class="field-icon i-carbon-password" />
              <BaseInput id="password" v-model="password" type="password" placeholder="请输入密码" required />
            </div>
            <PasswordStrengthMeter v-if="showPasswordStrength && password" :strength="passwordStrength" compact />
          </div>

          <div v-if="!isLogin" class="field-group">
            <div class="field-label-row">
              <label for="cardCode">卡密</label>
              <button v-if="cardClaimEnabled" type="button" class="free-card-button" :disabled="cardClaimLoading" @click="claimFreeCard">
                <span v-if="cardClaimLoading" class="i-svg-spinners-90-ring-with-bg" />
                <template v-else>
                  <span class="i-carbon-gift" /> 免费领取
                </template>
              </button>
            </div>
            <div class="field-control">
              <span class="field-icon i-carbon-ticket" />
              <BaseInput id="cardCode" v-model="cardCode" type="text" placeholder="请输入卡密" :required="!isLogin" />
            </div>
          </div>

          <Transition name="auth-message">
            <div v-if="error" :key="`error-${error}`" class="auth-message auth-message--error">
              <span class="i-carbon-warning-alt" />
              <span>{{ error }}<small v-if="lockoutRemaining > 0">{{ lockoutRemaining }} 分钟后解锁</small><small v-if="rateLimitRemaining > 0">{{ rateLimitRemaining }} 秒后可重试</small></span>
            </div>
          </Transition>
          <Transition name="auth-message">
            <div v-if="success" :key="`success-${success}`" class="auth-message auth-message--success">
              <span class="i-carbon-checkmark-filled" /> {{ success }}
            </div>
          </Transition>

          <BaseButton type="submit" variant="primary" block :loading="loading" class="auth-submit">
            <span v-if="!loading" class="submit-icon i-carbon-arrow-right" />
            {{ isLogin ? '登录工作台' : '创建账号' }}
          </BaseButton>
        </form>

        <div class="auth-actions">
          <button type="button" class="auth-switch" @click="toggleMode">
            <span>{{ isLogin ? '没有账号？' : '已有账号？' }}</span>
            {{ isLogin ? '立即注册' : '立即登录' }}
            <span class="i-carbon-arrow-right" />
          </button>
          <div v-if="isLogin" class="auth-secondary-actions">
            <button type="button" @click="openResetVerifyModal">
              <span class="i-carbon-reset" /> 忘记密码
            </button>
            <button type="button" @click="openRenewal">
              <span class="i-carbon-renew" /> 账号续费
            </button>
          </div>
        </div>

        <footer class="auth-footer">
          <div class="auth-footer-links">
            <a v-if="loginLinks.purchaseUrl" :href="loginLinks.purchaseUrl"><span class="i-carbon-shopping-cart" />购买卡密</a>
            <a v-if="loginLinks.qqGroupUrl" :href="loginLinks.qqGroupUrl" target="_blank" rel="noopener noreferrer"><span class="i-carbon-logo-qq" />加入QQ群</a>
            <button type="button" @click="showUpdateLog = true">
              <span class="i-carbon-document" />更新日志
            </button>
          </div>
          <span v-if="gameVersion">游戏版本 {{ gameVersion }}</span>
        </footer>
      </div>
    </main>

    <LoginModals />
    <UpdateLogModal :show="showUpdateLog" @close="showUpdateLog = false" />
  </div>
</template>

<style scoped>
.auth-layout {
  min-height: 100dvh;
  display: grid;
  grid-template-columns: minmax(360px, 0.88fr) minmax(440px, 1.12fr);
  background: var(--app-bg);
  color: var(--theme-text);
}
.auth-showcase {
  position: relative;
  display: flex;
  min-height: 100dvh;
  flex-direction: column;
  overflow: hidden;
  padding: 36px clamp(28px, 5vw, 84px);
  background: #173c2c;
  color: #f4fbf6;
}
.auth-showcase::before {
  content: '';
  position: absolute;
  inset: 0;
  opacity: 0.34;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.07) 1px, transparent 1px);
  background-size: 32px 32px;
  pointer-events: none;
}
.auth-showcase::after {
  content: '';
  position: absolute;
  right: -100px;
  bottom: -160px;
  width: 420px;
  height: 420px;
  border: 1px solid rgba(203, 242, 218, 0.18);
  border-radius: 50%;
  box-shadow:
    0 0 0 52px rgba(203, 242, 218, 0.04),
    0 0 0 104px rgba(203, 242, 218, 0.03);
  pointer-events: none;
}
.showcase-topline,
.showcase-copy,
.showcase-board,
.showcase-footer {
  position: relative;
  z-index: 1;
}
.showcase-topline {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #b6d8c3;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.16em;
}
.showcase-brand-mark {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border: 1px solid rgba(220, 255, 231, 0.3);
  border-radius: 8px;
  color: #baf0cb;
  font-size: 16px;
}
.showcase-copy {
  margin-top: auto;
  margin-bottom: 38px;
  max-width: 470px;
}
.showcase-kicker {
  color: var(--theme-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.16em;
}
.auth-showcase .showcase-kicker {
  color: #a5e0b8;
}
.showcase-copy h1 {
  margin: 12px 0 16px;
  font-size: clamp(32px, 4vw, 58px);
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 1.04;
}
.showcase-copy h1 em {
  color: #9ee0b1;
  font-style: normal;
}
.showcase-copy p {
  max-width: 360px;
  margin: 0;
  color: #c0d9c8;
  font-size: 14px;
  line-height: 1.8;
}
.showcase-board {
  max-width: 440px;
  padding: 16px;
  border: 1px solid rgba(220, 255, 231, 0.18);
  border-radius: 10px;
  background: rgba(8, 37, 24, 0.32);
}
.board-toolbar {
  display: flex;
  align-items: center;
  gap: 5px;
  color: #9cc9aa;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.12em;
}
.board-toolbar span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #8bc39a;
  opacity: 0.7;
}
.board-toolbar b {
  margin-left: auto;
  font-size: 8px;
  font-weight: 700;
}
.board-grid {
  height: 118px;
  display: flex;
  align-items: flex-end;
  gap: 10px;
  margin-top: 18px;
  padding: 0 4px;
  border-bottom: 1px solid rgba(220, 255, 231, 0.18);
}
.board-column {
  width: 24%;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.board-column--wide {
  width: 40%;
}
.board-column i {
  height: 12px;
  display: block;
  border-radius: 3px;
  background: rgba(181, 233, 194, 0.18);
}
.board-column i:nth-child(2) {
  width: 80%;
  background: rgba(181, 233, 194, 0.32);
}
.board-column i:nth-child(3) {
  width: 64%;
}
.board-column i:nth-child(4) {
  width: 92%;
  background: rgba(181, 233, 194, 0.24);
}
.board-column--accent i:nth-child(2) {
  background: #b7e7c1;
}
.board-column--accent i:nth-child(4) {
  background: #78c591;
}
.board-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  color: #b5d9be;
  font-size: 10px;
  font-weight: 700;
}
.showcase-footer {
  display: flex;
  justify-content: space-between;
  margin-top: 28px;
  color: #8fb69b;
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.12em;
}
.auth-panel {
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface-1);
}
.auth-panel-top {
  min-height: 76px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 clamp(24px, 5vw, 84px);
  border-bottom: 1px solid var(--surface-border);
}
.auth-panel-label {
  color: var(--theme-text);
  font-size: 13px;
  font-weight: 750;
}
.auth-panel-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted-text);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.1em;
}
.auth-content {
  width: min(100%, 460px);
  margin: auto;
  padding: 42px 28px 36px;
}
.auth-logo-row {
  display: flex;
  align-items: center;
  gap: 11px;
}
.auth-logo {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--theme-primary) 28%, var(--surface-border));
  border-radius: 10px;
  background: color-mix(in srgb, var(--theme-primary) 10%, var(--surface-1));
  color: var(--theme-primary);
  font-size: 20px;
}
.auth-logo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.auth-logo-row > div:last-child {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.auth-logo-name {
  color: var(--theme-text);
  font-size: 13px;
  font-weight: 750;
}
.auth-logo-caption {
  color: var(--muted-text);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.1em;
}
.auth-heading {
  margin-top: 42px;
}
.auth-heading h2 {
  margin: 8px 0 7px;
  color: var(--theme-text);
  font-size: 32px;
  letter-spacing: -0.03em;
  line-height: 1.1;
}
.auth-heading p {
  margin: 0;
  color: var(--muted-text);
  font-size: 13px;
  line-height: 1.7;
}
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 18px;
  margin-top: 28px;
}
.field-group {
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.field-group label {
  color: var(--theme-text);
  font-size: 12px;
  font-weight: 700;
}
.field-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.field-control {
  position: relative;
}
.field-icon {
  position: absolute;
  z-index: 1;
  top: 50%;
  left: 13px;
  color: var(--muted-text);
  font-size: 16px;
  pointer-events: none;
  transform: translateY(-50%);
}
.field-control :deep(.base-input) {
  padding-left: 40px;
}
.field-control:focus-within .field-icon {
  color: var(--theme-primary);
}
.field-hint {
  margin: 0;
  padding-left: 2px;
  color: var(--muted-text);
  font-size: 11px;
}
.field-hint--error {
  color: #c45353;
}
.free-card-button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 0;
  background: transparent;
  color: var(--theme-primary);
  cursor: pointer;
  font-size: 11px;
  font-weight: 750;
}
.free-card-button:disabled {
  cursor: wait;
  opacity: 0.6;
}
.auth-message {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.5;
}
.auth-message small {
  display: inline-block;
  margin-left: 8px;
  opacity: 0.78;
}
.auth-message--error {
  border-color: #edcaca;
  background: #fff5f5;
  color: #a84242;
}
.auth-message--success {
  border-color: #bfe2cb;
  background: #f2fbf5;
  color: #247548;
}
.dark .auth-message--error {
  border-color: #5b3333;
  background: #2a1d1d;
  color: #f3a6a6;
}
.dark .auth-message--success {
  border-color: #315a40;
  background: #19291e;
  color: #91d4a4;
}
.auth-submit {
  min-height: 46px;
  margin-top: 4px;
  border-radius: 8px !important;
  font-size: 13px;
  font-weight: 750;
  letter-spacing: 0.01em;
}
.submit-icon {
  margin-right: 7px;
}
.auth-actions {
  margin-top: 18px;
}
.auth-switch {
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  border: 0;
  background: transparent;
  color: var(--theme-primary);
  cursor: pointer;
  font-size: 12px;
  font-weight: 750;
}
.auth-switch span:first-child {
  color: var(--muted-text);
  font-weight: 500;
}
.auth-switch > span:last-child {
  margin-left: 3px;
  transition: transform 0.18s ease;
}
.auth-switch:hover > span:last-child {
  transform: translateX(3px);
}
.auth-secondary-actions {
  display: flex;
  justify-content: center;
  gap: 18px;
  margin-top: 14px;
}
.auth-secondary-actions button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 0;
  background: transparent;
  color: var(--muted-text);
  cursor: pointer;
  font-size: 11px;
}
.auth-secondary-actions button:hover {
  color: var(--theme-text);
}
.auth-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 42px;
  padding-top: 16px;
  border-top: 1px solid var(--surface-border);
  color: var(--muted-text);
  font-size: 10px;
}
.auth-footer-links {
  display: flex;
  flex-wrap: wrap;
  gap: 13px;
}
.auth-footer a,
.auth-footer button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 0;
  background: transparent;
  color: var(--muted-text);
  cursor: pointer;
  font-size: 10px;
  text-decoration: none;
}
.auth-footer a:hover,
.auth-footer button:hover {
  color: var(--theme-primary);
}
.auth-message-enter-active,
.auth-message-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}
.auth-message-enter-from,
.auth-message-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media (max-width: 800px) {
  .auth-layout {
    display: block;
  }
  .auth-showcase {
    min-height: 190px;
    padding: 22px 24px;
  }
  .showcase-copy {
    margin: 44px 0 0;
  }
  .showcase-copy h1 {
    margin: 8px 0 0;
    font-size: 28px;
  }
  .showcase-copy p,
  .showcase-board,
  .showcase-footer {
    display: none;
  }
  .auth-panel-top {
    min-height: 62px;
    padding: 0 24px;
  }
  .auth-content {
    padding: 32px 24px 28px;
  }
  .auth-heading {
    margin-top: 34px;
  }
}

@media (max-width: 420px) {
  .auth-showcase {
    min-height: 158px;
    padding: 18px 18px;
  }
  .showcase-copy {
    margin-top: 32px;
  }
  .showcase-copy h1 {
    font-size: 25px;
  }
  .auth-panel-top {
    padding: 0 18px;
  }
  .auth-content {
    padding: 26px 18px 24px;
  }
  .auth-footer {
    align-items: flex-start;
    flex-direction: column;
    margin-top: 30px;
  }
}
</style>
