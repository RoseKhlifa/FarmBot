<script setup lang="ts">
import { provide, toRefs } from 'vue'
import LoginBackground from '@/components/login/LoginBackground.vue'
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

const {
  handleSubmit,
  toggleMode,
  openResetVerifyModal,
  openRenewal,
  claimFreeCard,
} = authFlow
</script>

<template>
  <div class="login-container">
    <LoginBackground />

    <!-- ===== 登录卡片（带入场动画） ===== -->
    <main class="login-card">
      <!-- 卡片顶部装饰光晕 -->
      <div class="card-glow" />

      <!-- Logo 区域（带呼吸光环） -->
      <div class="logo-area">
        <div class="logo-icon-wrapper">
          <div class="logo-ring-1" />
          <div class="logo-ring-2" />
          <div class="logo-icon">
            <img
              v-if="loginLinks.logoUrl && !logoLoadFailed"
              :src="loginLinks.logoUrl"
              :alt="`${loginLinks.title || 'QQ农场智能助手'}图标`"
              class="logo-image"
              @error="logoLoadFailed = true"
            >
            <div v-else class="i-carbon-sprout text-3xl" />
          </div>
        </div>
        <h1 class="logo-title">
          {{ loginLinks.title || 'QQ农场智能助手' }}
        </h1>
        <p class="logo-subtitle">
          {{ isLogin
            ? (loginLinks.loginSubtitle || '欢迎回来，开启智慧农耕之旅')
            : (loginLinks.registerSubtitle || '创建账号，开启智慧农耕之旅') }}
        </p>
      </div>

      <!-- 表单区域 -->
      <form class="form-area" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label class="form-label">
            <span class="label-icon i-carbon-user" />
            用户名
          </label>
          <div class="input-wrapper">
            <span class="input-icon i-carbon-user" />
            <BaseInput
              id="username"
              v-model="username"
              type="text"
              placeholder="请输入用户名"
              required
            />
          </div>
          <p v-if="username && !usernameValid.valid" class="form-hint error">
            {{ usernameValid.message }}
          </p>
        </div>

        <div class="form-group">
          <label class="form-label">
            <span class="label-icon i-carbon-password" />
            密码
          </label>
          <div class="input-wrapper">
            <span class="input-icon i-carbon-password" />
            <BaseInput
              id="password"
              v-model="password"
              type="password"
              placeholder="请输入密码"
              required
            />
          </div>
          <PasswordStrengthMeter
            v-if="showPasswordStrength && password"
            :strength="passwordStrength"
            compact
          />

          <!-- 消息动画容器 -->
          <Transition name="msg-slide">
            <div v-if="error" :key="`error-${error}`" class="message error-message">
              <span class="message-icon i-carbon-warning" />
              <div class="message-content">
                {{ error }}
                <span v-if="lockoutRemaining > 0" class="lockout-timer">
                  ({{ lockoutRemaining }} 分钟后解锁)
                </span>
                <span v-if="rateLimitRemaining > 0" class="lockout-timer">
                  ({{ rateLimitRemaining }} 秒后可重试)
                </span>
              </div>
            </div>
          </Transition>
          <Transition name="msg-slide">
            <div v-if="success" :key="`success-${success}`" class="message success-message">
              <span class="message-icon i-carbon-checkmark-filled" />
              {{ success }}
            </div>
          </Transition>
        </div>

        <div v-if="!isLogin" class="form-group">
          <label class="form-label">
            <span class="label-icon i-carbon-ticket" />
            卡密
          </label>

          <div v-if="cardClaimEnabled" class="mb-2">
            <button
              type="button"
              class="claim-card-btn"
              :disabled="cardClaimLoading"
              @click="claimFreeCard"
            >
              <span v-if="cardClaimLoading" class="i-svg-spinners-90-ring-with-bg" />
              <span v-else>
                <span class="sparkle-icon">✦</span>
                免费领取卡密
              </span>
            </button>
          </div>

          <div class="input-wrapper">
            <span class="input-icon i-carbon-ticket" />
            <BaseInput
              id="cardCode"
              v-model="cardCode"
              type="text"
              placeholder="请输入卡密"
              :required="!isLogin"
            />
          </div>
        </div>

        <BaseButton
          type="submit"
          variant="primary"
          block
          :loading="loading"
          class="submit-btn"
        >
          <span v-if="!loading" class="submit-text">
            <span class="submit-icon i-carbon-login" />
            {{ isLogin ? '登 录' : '注 册' }}
          </span>
        </BaseButton>
      </form>

      <!-- 模式切换 & 快捷操作 -->
      <div class="switch-area" :class="{ 'single-action': !isLogin }">
        <button
          type="button"
          class="switch-btn"
          @click="toggleMode"
        >
          <span class="switch-btn-icon">{{ isLogin ? '→' : '←' }}</span>
          {{ isLogin ? '没有账号？立即注册' : '已有账号？立即登录' }}
        </button>
        <div v-if="isLogin" class="login-quick-actions">
          <button type="button" class="quick-action-btn" @click="openResetVerifyModal">
            <span class="i-carbon-reset" />
            忘记密码
          </button>
          <button type="button" class="quick-action-btn" @click="openRenewal">
            <span class="i-carbon-renew" />
            账号续费
          </button>
        </div>
      </div>

      <!-- 底部链接 -->
      <div class="card-footer">
        <div class="footer-info">
          <a
            v-if="loginLinks.purchaseUrl"
            :href="loginLinks.purchaseUrl"
            class="footer-link purchase-link"
          >
            <span class="i-carbon-shopping-cart" />
            购买卡密
          </a>
          <a
            v-if="loginLinks.qqGroupUrl"
            :href="loginLinks.qqGroupUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link qq-group-link"
          >
            <span class="i-carbon-logo-qq" />
            加入QQ群
          </a>
          <button
            type="button"
            class="footer-link update-log-link"
            @click="showUpdateLog = true"
          >
            <span class="i-carbon-document" />
            更新日志
          </button>
        </div>
        <div v-if="gameVersion" class="game-version">
          当前游戏版本：{{ gameVersion }}
        </div>
      </div>
    </main>

    <LoginModals />

    <UpdateLogModal :show="showUpdateLog" @close="showUpdateLog = false" />
  </div>
</template>

<style scoped>
/* =============================================
   登录页 — 田园农场主题 · 增强版
   ============================================= */

/* --- 容器 --- */
.login-container {
  height: 100dvh;
  width: 100%;
  display: flex;
  flex-direction: column;
  background: transparent;
  font-family:
    'Noto Sans SC',
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    sans-serif;
  position: relative;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 24px;
}

/* =============================================
   登录卡片 — 增强毛玻璃 + 入场动画
   ============================================= */

.login-card {
  width: 100%;
  max-width: 420px;
  margin: 0 auto;
  padding: 34px 30px 22px;
  line-height: 1.35;
  background: transparent;
  border: none;
  border-radius: 24px;
  box-shadow: none;
  position: relative;
  z-index: 10;
  animation: card-enter 0.8s cubic-bezier(0.16, 1, 0.3, 1) both;
  overflow: visible;
}

@keyframes card-enter {
  0% {
    opacity: 0;
    transform: translateY(40px) scale(0.96);
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

/* 卡片顶部装饰光晕 */
.card-glow {
  position: absolute;
  top: -80px;
  left: 50%;
  transform: translateX(-50%);
  width: 280px;
  height: 160px;
  background: radial-gradient(ellipse, color-mix(in srgb, #818cf8 24%, transparent) 0%, transparent 70%);
  pointer-events: none;
  opacity: 0.6;
  animation: glow-breathe 5s ease-in-out infinite alternate;
}

@keyframes glow-breathe {
  0% {
    opacity: 0.4;
    transform: translateX(-50%) scaleY(0.9);
  }
  100% {
    opacity: 0.8;
    transform: translateX(-50%) scaleY(1.2);
  }
}

/* --- Logo 区域 --- */
.logo-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  margin-bottom: 18px;
  position: relative;
  z-index: 1;
}

.logo-icon-wrapper {
  position: relative;
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 呼吸光环 */
.logo-ring-1,
.logo-ring-2 {
  position: absolute;
  border-radius: 50%;
  border: 2px solid color-mix(in srgb, var(--theme-primary, #22c55e) 25%, transparent);
  animation: ring-pulse 3s ease-out infinite;
}
.logo-ring-1 {
  width: 94px;
  height: 94px;
}
.logo-ring-2 {
  width: 110px;
  height: 110px;
  animation-delay: 1s;
}

@keyframes ring-pulse {
  0% {
    transform: scale(0.8);
    opacity: 0;
  }
  50% {
    opacity: 0.6;
  }
  100% {
    transform: scale(1.3);
    opacity: 0;
  }
}

.logo-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  background: linear-gradient(135deg, #e8f5e9, #c8e6c9);
  border: 3px solid rgba(102, 187, 106, 0.5);
  border-radius: 50%;
  color: #43a047;
  box-shadow:
    0 8px 24px rgba(102, 187, 106, 0.2),
    0 0 0 1px rgba(255, 255, 255, 0.6) inset;
  position: relative;
  z-index: 1;
  transition:
    transform 0.3s ease,
    box-shadow 0.3s ease;
}

.logo-icon:hover {
  transform: scale(1.05);
  box-shadow:
    0 12px 32px rgba(102, 187, 106, 0.3),
    0 0 0 1px rgba(255, 255, 255, 0.7) inset;
}

.logo-title {
  font-size: 1.5rem;
  font-weight: 800;
  line-height: 1.2;
  margin: 0;
  max-width: 100%;
  overflow-wrap: anywhere;
  background: linear-gradient(135deg, #2e7d32, #43a047, #66bb6a);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  letter-spacing: 0.02em;
}

.logo-subtitle {
  color: #94a3b8;
  font-size: 0.82rem;
  line-height: 1.3;
  margin: 6px 0 0;
  max-width: 100%;
  overflow-wrap: anywhere;
  font-weight: 500;
  letter-spacing: 0.01em;
}

.logo-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 50%;
}

/* --- 表单区域 --- */
.form-area {
  display: flex;
  flex-direction: column;
  gap: 13px;
  position: relative;
  z-index: 1;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 0.8rem;
  line-height: 1.3;
  font-weight: 600;
  color: #86efac;
  letter-spacing: 0.02em;
}

.label-icon {
  font-size: 0.95rem;
  opacity: 0.7;
}

/* 输入框包裹（内置图标） */
.input-wrapper {
  position: relative;
}

.input-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 1rem;
  color: #a0aec0;
  z-index: 2;
  pointer-events: none;
  transition: color 0.25s ease;
}

.input-wrapper:focus-within .input-icon {
  color: var(--theme-primary, #22c55e);
}

.login-card :deep(.base-input) {
  min-height: 42px;
  border-color: rgba(255, 255, 255, 0.22);
  border-radius: 12px;
  background: var(--theme-glass);
  color: #e2e8f0;
  padding: 8px 12px 8px 36px;
  font-size: 0.85rem;
  transition: all 0.25s ease;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
}

.login-card :deep(.base-input:focus) {
  border-color: #818cf8;
  background: rgba(255, 255, 255, 0.14);
  box-shadow:
    0 0 0 3px rgba(129, 140, 248, 0.25),
    0 2px 8px rgba(0, 0, 0, 0.2);
}

.login-card :deep(.base-input:hover:not(:focus)) {
  border-color: rgba(255, 255, 255, 0.28);
  background: rgba(255, 255, 255, 0.12);
}

.form-hint {
  font-size: 0.75rem;
  color: #94a3b8;
  padding-left: 4px;
}

.form-hint.error {
  color: #ef5350;
  font-weight: 500;
}

.lockout-timer {
  display: block;
  font-size: 0.75rem;
  opacity: 0.8;
  margin-top: 2px;
}

/* --- 消息（带动画） --- */
.message {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 0.85rem;
  line-height: 1.5;
}

.message-icon {
  flex: 0 0 auto;
  font-size: 1rem;
  margin-top: 2px;
}

.error-message {
  background: rgba(254, 202, 202, 0.5);
  color: #b91c1c;
  border: 1px solid rgba(239, 68, 68, 0.2);
  backdrop-filter: blur(8px);
}

.success-message {
  background: rgba(187, 247, 208, 0.5);
  color: #166534;
  border: 1px solid rgba(34, 197, 94, 0.2);
  backdrop-filter: blur(8px);
}

/* 消息滑入动画 */
.msg-slide-enter-active {
  animation: msg-in 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}
.msg-slide-leave-active {
  animation: msg-out 0.3s ease-in;
}
@keyframes msg-in {
  0% {
    opacity: 0;
    transform: translateY(-8px) scale(0.96);
    max-height: 0;
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1);
    max-height: 80px;
  }
}
@keyframes msg-out {
  0% {
    opacity: 1;
    transform: translateY(0) scale(1);
    max-height: 80px;
  }
  100% {
    opacity: 0;
    transform: translateY(-6px) scale(0.96);
    max-height: 0;
    padding: 0;
    margin: 0;
  }
}

/* --- 提交按钮（带微光效） --- */
.submit-btn {
  margin-top: 4px;
  height: 46px;
  background: var(--theme-glass) !important;
  backdrop-filter: blur(14px) !important;
  -webkit-backdrop-filter: blur(14px) !important;
  border: 1px solid rgba(255, 255, 255, 0.28) !important;
  color: #e2e8f0 !important;
  font-size: 0.95rem;
  font-weight: 700;
  border-radius: 14px;
  box-shadow:
    0 8px 24px rgba(0, 0, 0, 0.22),
    0 2px 4px rgba(0, 0, 0, 0.1) !important;
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  letter-spacing: 0.06em;
  position: relative;
  overflow: hidden;
}

.submit-btn::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent 0%, rgba(255, 255, 255, 0.15) 50%, transparent 100%);
  transform: translateX(-100%);
  transition: transform 0.6s ease;
  pointer-events: none;
}

.submit-btn:hover:not(:disabled)::before {
  transform: translateX(100%);
}

.submit-btn:hover:not(:disabled) {
  transform: translateY(-3px);
  box-shadow:
    0 12px 32px rgba(0, 0, 0, 0.28) !important,
    0 4px 8px rgba(0, 0, 0, 0.12) !important;
}

.submit-btn:active:not(:disabled) {
  transform: translateY(-1px) scale(0.99);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.2) !important;
}

.submit-text {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.submit-icon {
  font-size: 1.1rem;
}

/* --- 切换 & 快捷操作 --- */
.switch-area {
  display: grid;
  gap: 8px;
  grid-template-columns: 1fr;
  margin-top: 14px;
  text-align: center;
  position: relative;
  z-index: 1;
}

.switch-btn {
  background: var(--theme-glass);
  border: 1px solid rgba(255, 255, 255, 0.22);
  color: #cbd5e1;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  padding: 11px 20px;
  border-radius: 12px;
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  backdrop-filter: blur(16px);
}

.switch-btn:hover {
  background: rgba(255, 255, 255, 0.14);
  border-color: rgba(129, 140, 248, 0.5);
  color: #a5b4fc;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.switch-btn:active {
  transform: translateY(0);
}

.switch-btn-icon {
  font-size: 1.1rem;
  transition: transform 0.25s ease;
}

.switch-btn:hover .switch-btn-icon {
  transform: translateX(3px);
}

.login-quick-actions {
  display: flex;
  gap: 10px;
}

.quick-action-btn {
  flex: 1;
  background: var(--theme-glass);
  border: 1px solid rgba(255, 255, 255, 0.22);
  border-radius: 12px;
  color: #cbd5e1;
  cursor: pointer;
  font-size: 0.82rem;
  font-weight: 600;
  padding: 11px 12px;
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  backdrop-filter: blur(16px);
}

.quick-action-btn:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(129, 140, 248, 0.4);
  color: #a5b4fc;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.04);
}

/* --- 底部链接 --- */
.card-footer {
  text-align: center;
  margin-top: 12px;
  color: #94a3b8;
  font-size: 0.8rem;
  position: relative;
  z-index: 1;
}

.footer-info {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
}

.footer-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 8px;
  padding: 4px 10px;
  font-weight: 600;
  font-size: 0.72rem;
  text-decoration: none;
  transition: all 0.2s ease;
  cursor: pointer;
  border: 1px solid transparent;
}

.footer-link:hover {
  text-decoration: none;
  transform: translateY(-1px);
}

.purchase-link {
  color: #bbf7d0;
  background: rgba(34, 197, 94, 0.18);
  border-color: rgba(34, 197, 94, 0.32);
}
.purchase-link:hover {
  background: rgba(34, 197, 94, 0.28);
}

.qq-group-link {
  color: #bae6fd;
  background: rgba(56, 189, 248, 0.16);
  border-color: rgba(56, 189, 248, 0.32);
}
.qq-group-link:hover {
  background: rgba(56, 189, 248, 0.26);
}

.update-log-link {
  color: #fde68a;
  background: rgba(250, 204, 21, 0.16);
  border-color: rgba(250, 204, 21, 0.32);
}
.update-log-link:hover {
  color: #fef08a;
  background: rgba(250, 204, 21, 0.26);
}

.game-version {
  display: flex;
  justify-content: center;
  width: fit-content;
  margin: 8px auto 0;
  font-size: 0.7rem;
  color: #e2e8f0;
  background: var(--theme-glass);
  border: 1px solid rgba(255, 255, 255, 0.22);
  border-radius: 999px;
  padding: 4px 14px;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  white-space: nowrap;
}

/* --- 免费领取按钮（带闪烁星星） --- */
.claim-card-btn {
  width: 100%;
  padding: 10px 16px;
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.22), rgba(16, 185, 129, 0.22));
  border: 1px solid rgba(34, 197, 94, 0.4);
  border-radius: 12px;
  color: #86efac;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.25s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.claim-card-btn:hover:not(:disabled) {
  border-color: #86efac;
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.34), rgba(16, 185, 129, 0.34));
  transform: translateY(-1px);
  box-shadow: 0 4px 16px rgba(34, 197, 94, 0.25);
}

.claim-card-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.sparkle-icon {
  display: inline-block;
  animation: sparkle 2s ease-in-out infinite;
}

@keyframes sparkle {
  0%,
  100% {
    transform: scale(0.8) rotate(0deg);
    opacity: 0.6;
  }
  50% {
    transform: scale(1.2) rotate(180deg);
    opacity: 1;
  }
}

/* --- 响应式 --- */
@media (max-width: 480px) {
  .login-container {
    align-items: flex-start;
    padding: 28px 12px 12px;
  }

  .login-card {
    padding: 24px 18px 18px;
    border-radius: 18px;
  }

  .logo-icon {
    width: 56px;
    height: 56px;
  }

  .logo-title {
    font-size: 1.2rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .login-card,
  .submit-btn,
  .switch-btn,
  .quick-action-btn,
  .card-glow,
  .logo-ring-1,
  .logo-ring-2,
  .sparkle-icon {
    animation: none !important;
    transition-duration: 0.01ms !important;
  }
}
</style>
