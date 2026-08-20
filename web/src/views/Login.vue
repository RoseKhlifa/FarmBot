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
  username,
  password,
  cardCode,
  isLogin,
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
  <div class="auth-page">
    <div class="auth-shell">
      <header class="auth-brand">
        <div class="auth-brand-mark">
          <span class="i-carbon-sprout" />
        </div>
        <div>
          <strong>{{ loginLinks.title || 'QQ农场智能助手' }}</strong>
          <span>FARMBOT / CONTROL CENTER</span>
        </div>
      </header>

      <section class="auth-card">
        <div class="auth-card-heading">
          <div class="auth-card-kicker">
            账号访问
          </div>
          <div class="auth-card-status">
            <span class="status-dot status-dot--live" /> SECURE SESSION
          </div>
        </div>

        <div class="auth-tabs" role="tablist" aria-label="账号操作">
          <button type="button" :class="{ 'auth-tab--active': isLogin }" @click="!isLogin && toggleMode()">
            登录
          </button>
          <button type="button" :class="{ 'auth-tab--active': !isLogin }" @click="isLogin && toggleMode()">
            注册
          </button>
        </div>

        <div class="auth-card-body">
          <div class="auth-intro">
            <span class="auth-intro-kicker">{{ isLogin ? 'WELCOME BACK' : 'NEW WORKSPACE' }}</span>
            <h1>{{ isLogin ? '欢迎回来' : '创建账号' }}</h1>
            <p>{{ isLogin ? (loginLinks.loginSubtitle || '登录后继续管理你的农场') : (loginLinks.registerSubtitle || '创建账号，开始使用 FarmBot') }}</p>
          </div>

          <form class="auth-form" @submit.prevent="handleSubmit">
            <div class="auth-field">
              <label for="username"><span class="i-carbon-user" />用户名</label>
              <BaseInput id="username" v-model="username" type="text" placeholder="请输入用户名" required />
              <p v-if="username && !usernameValid.valid" class="auth-field-error">
                {{ usernameValid.message }}
              </p>
            </div>

            <div class="auth-field">
              <label for="password"><span class="i-carbon-password" />密码</label>
              <BaseInput id="password" v-model="password" type="password" placeholder="请输入密码" required />
              <PasswordStrengthMeter v-if="showPasswordStrength && password" :strength="passwordStrength" compact />
            </div>

            <div v-if="!isLogin" class="auth-field">
              <div class="auth-label-row">
                <label for="cardCode"><span class="i-carbon-ticket" />卡密</label>
                <button v-if="cardClaimEnabled" type="button" class="auth-free-card" :disabled="cardClaimLoading" @click="claimFreeCard">
                  <span v-if="cardClaimLoading" class="i-svg-spinners-90-ring-with-bg" />
                  <template v-else>
                    <span class="i-carbon-gift" />免费领取
                  </template>
                </button>
              </div>
              <BaseInput id="cardCode" v-model="cardCode" type="text" placeholder="请输入卡密" :required="!isLogin" />
            </div>

            <Transition name="auth-message">
              <div v-if="error" :key="`error-${error}`" class="auth-message auth-message--error">
                <span class="i-carbon-warning-alt" />
                <span>{{ error }}<small v-if="lockoutRemaining > 0">{{ lockoutRemaining }} 分钟后解锁</small><small v-if="rateLimitRemaining > 0">{{ rateLimitRemaining }} 秒后可重试</small></span>
              </div>
            </Transition>
            <Transition name="auth-message">
              <div v-if="success" :key="`success-${success}`" class="auth-message auth-message--success">
                <span class="i-carbon-checkmark-filled" />{{ success }}
              </div>
            </Transition>

            <BaseButton type="submit" variant="primary" block :loading="loading" class="auth-submit">
              <span>{{ isLogin ? '登录工作台' : '创建账号' }}</span>
              <span v-if="!loading" class="i-carbon-arrow-right" />
            </BaseButton>
          </form>

          <div class="auth-actions">
            <button type="button" class="auth-switch" @click="toggleMode">
              <span>{{ isLogin ? '没有账号？' : '已有账号？' }}</span>{{ isLogin ? '立即注册' : '立即登录' }}<span class="i-carbon-arrow-right" />
            </button>
            <div v-if="isLogin" class="auth-secondary-actions">
              <button type="button" @click="openResetVerifyModal">
                <span class="i-carbon-reset" />忘记密码
              </button>
              <button type="button" @click="openRenewal">
                <span class="i-carbon-renew" />账号续费
              </button>
            </div>
          </div>
        </div>
      </section>

      <footer class="auth-footer">
        <div class="auth-footer-links">
          <a v-if="loginLinks.purchaseUrl" :href="loginLinks.purchaseUrl"><span class="i-carbon-shopping-cart" />购买卡密</a>
          <a v-if="loginLinks.qqGroupUrl" :href="loginLinks.qqGroupUrl" target="_blank" rel="noopener noreferrer"><span class="i-carbon-logo-qq" />加入QQ群</a>
          <button type="button" @click="showUpdateLog = true">
            <span class="i-carbon-document" />更新日志
          </button>
        </div>
        <span>{{ gameVersion ? `游戏版本 ${gameVersion}` : 'FarmBot operations' }}</span>
      </footer>
    </div>

    <LoginModals />
    <UpdateLogModal :show="showUpdateLog" @close="showUpdateLog = false" />
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow-y: auto;
  padding: 40px 16px;
  background: var(--app-bg);
}
.auth-shell {
  width: min(100%, 440px);
}
.auth-brand {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 11px;
  margin-bottom: 22px;
}
.auth-brand-mark {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--theme-primary) 35%, var(--surface-border));
  border-radius: 11px;
  background: color-mix(in srgb, var(--theme-primary) 12%, var(--surface-1));
  color: var(--theme-primary);
  font-size: 22px;
}
.auth-brand div:last-child {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.auth-brand strong {
  color: var(--theme-text);
  font-size: 17px;
  line-height: 1.1;
}
.auth-brand span:not(.i-carbon-sprout) {
  color: var(--muted-text);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.14em;
}
.auth-card {
  overflow: hidden;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface-1) 88%, transparent);
  box-shadow: var(--surface-shadow-soft);
}
.auth-card-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 48px;
  padding: 0 18px;
  border-bottom: 1px solid var(--surface-border);
}
.auth-card-kicker,
.auth-card-status {
  color: var(--muted-text);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.11em;
  text-transform: uppercase;
}
.auth-card-status {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 9px;
}
.auth-tabs {
  position: relative;
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  border-bottom: 1px solid var(--surface-border);
}
.auth-tabs button {
  position: relative;
  min-height: 44px;
  border: 0;
  background: transparent;
  color: var(--muted-text);
  cursor: pointer;
  font-size: 13px;
  font-weight: 700;
}
.auth-tabs button::after {
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  background: transparent;
  content: '';
  transition: background 0.18s ease;
}
.auth-tabs .auth-tab--active {
  color: var(--theme-text);
}
.auth-tabs .auth-tab--active::after {
  background: var(--theme-primary);
}
.auth-card-body {
  padding: 24px;
}
.auth-intro-kicker {
  color: var(--theme-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.14em;
}
.auth-intro h1 {
  margin: 8px 0 6px;
  color: var(--theme-text);
  font-size: 27px;
  letter-spacing: -0.03em;
  line-height: 1.15;
}
.auth-intro p {
  margin: 0;
  color: var(--muted-text);
  font-size: 12px;
  line-height: 1.7;
}
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 15px;
  margin-top: 22px;
}
.auth-field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.auth-field label,
.auth-label-row label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted-text);
  font-size: 11px;
  font-weight: 700;
}
.auth-field label span,
.auth-label-row label span {
  color: var(--theme-primary);
  font-size: 14px;
}
.auth-field :deep(.base-input) {
  min-height: 42px;
  border-radius: 9px;
  background: var(--input-bg);
}
.auth-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.auth-free-card,
.auth-secondary-actions button,
.auth-switch,
.auth-footer-links button,
.auth-footer-links a {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 0;
  background: transparent;
  color: var(--muted-text);
  cursor: pointer;
  font-size: 11px;
  text-decoration: none;
  transition: color 0.18s ease;
}
.auth-free-card:hover,
.auth-secondary-actions button:hover,
.auth-footer-links button:hover,
.auth-footer-links a:hover {
  color: var(--theme-primary);
}
.auth-free-card {
  color: var(--theme-primary);
  font-weight: 700;
}
.auth-field-error {
  margin: 0;
  color: #ef4444;
  font-size: 11px;
}
.auth-message {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 9px 11px;
  border-radius: 8px;
  font-size: 11px;
  line-height: 1.5;
}
.auth-message small {
  display: block;
  margin-top: 2px;
  opacity: 0.8;
}
.auth-message--error {
  background: color-mix(in srgb, #ef4444 10%, transparent);
  color: #ef4444;
}
.auth-message--success {
  background: color-mix(in srgb, var(--theme-primary) 10%, transparent);
  color: var(--theme-primary);
}
.auth-submit {
  min-height: 42px;
  border-radius: 9px !important;
  background: var(--theme-primary) !important;
  color: #052e1b !important;
  box-shadow: none !important;
  font-size: 13px;
  font-weight: 800;
}
.auth-submit:hover {
  filter: brightness(1.06);
  transform: none !important;
}
.auth-submit > span:last-child {
  margin-left: 8px;
}
.auth-actions {
  margin-top: 18px;
  text-align: center;
}
.auth-switch {
  color: var(--theme-primary);
  font-size: 12px;
  font-weight: 700;
}
.auth-switch span:first-child {
  margin-right: 4px;
  color: var(--muted-text);
  font-weight: 500;
}
.auth-switch span:last-child {
  margin-left: 5px;
}
.auth-secondary-actions {
  display: flex;
  justify-content: center;
  gap: 18px;
  margin-top: 14px;
}
.auth-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-top: 16px;
  color: var(--muted-text);
  font-size: 10px;
}
.auth-footer-links {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.status-dot {
  width: 6px;
  height: 6px;
  display: inline-block;
  flex: 0 0 auto;
  border-radius: 50%;
  background: currentColor;
}
.status-dot--live {
  color: var(--theme-primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--theme-primary) 15%, transparent);
}
.auth-message-enter-active,
.auth-message-leave-active {
  transition:
    opacity 0.15s ease,
    transform 0.15s ease;
}
.auth-message-enter-from,
.auth-message-leave-to {
  opacity: 0;
  transform: translateY(-3px);
}
@media (max-width: 480px) {
  .auth-page {
    align-items: flex-start;
    padding: 24px 12px;
  }
  .auth-brand {
    margin-bottom: 18px;
  }
  .auth-card-body {
    padding: 20px 16px;
  }
  .auth-footer {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
