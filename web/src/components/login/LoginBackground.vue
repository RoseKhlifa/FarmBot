<template>
  <div class="bg-decoration" aria-hidden="true">
    <div class="sky-gradient" />

    <div class="sun-decoration">
      <div class="sun-core" />
      <div class="sun-glow-1" />
      <div class="sun-glow-2" />
    </div>

    <div class="cloud cloud-1" />
    <div class="cloud cloud-2" />
    <div class="cloud cloud-3" />

    <div class="particle particle-1" />
    <div class="particle particle-2" />
    <div class="particle particle-3" />
    <div class="particle particle-4" />
    <div class="particle particle-5" />
    <div class="particle particle-6" />
  </div>
</template>

<style scoped>
@keyframes bg-shift {
  0% {
    background: linear-gradient(180deg, #070b1f 0%, #141a40 36%, #2a2350 60%, #4b3a6b 80%, #7d5a7e 100%);
  }
  100% {
    background: linear-gradient(180deg, #0a0f2c 0%, #1a2150 36%, #322a60 60%, #574a82 80%, #8a6589 100%);
  }
}

.bg-decoration {
  position: fixed;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
  z-index: 0;
  background: linear-gradient(180deg, #0b1020 0%, #141a40 36%, #2a2350 60%, #4b3a6b 80%, #7d5a7e 100%);
  animation: bg-shift 20s ease-in-out infinite alternate;
}

.sky-gradient {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 72%;
  background:
    radial-gradient(1.5px 1.5px at 18% 28%, rgba(255, 255, 255, 0.85) 0, transparent 100%),
    radial-gradient(1px 1px at 42% 18%, rgba(255, 255, 255, 0.55) 0, transparent 100%),
    radial-gradient(1.5px 1.5px at 68% 34%, rgba(255, 255, 255, 0.7) 0, transparent 100%),
    radial-gradient(1px 1px at 84% 22%, rgba(255, 255, 255, 0.5) 0, transparent 100%),
    radial-gradient(1px 1px at 56% 46%, rgba(255, 255, 255, 0.45) 0, transparent 100%),
    radial-gradient(1.5px 1.5px at 30% 52%, rgba(255, 255, 255, 0.6) 0, transparent 100%),
    linear-gradient(180deg, rgba(7, 11, 31, 0) 0%, rgba(20, 26, 64, 0) 100%);
}

.sun-decoration {
  position: absolute;
  top: 7%;
  right: 12%;
  width: 90px;
  height: 90px;
}

.sun-core {
  position: absolute;
  inset: 22px;
  border-radius: 50%;
  background: radial-gradient(circle, #fdfdfb 0%, #e8eef7 55%, rgba(232, 238, 247, 0) 100%);
  box-shadow: 0 0 26px 6px rgba(210, 225, 255, 0.35);
  animation: sun-pulse 6s ease-in-out infinite;
}

.sun-glow-1 {
  position: absolute;
  inset: 4px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(200, 220, 255, 0.25) 0%, transparent 70%);
  animation: sun-glow-soft 8s ease-in-out infinite alternate;
}

.sun-glow-2 {
  position: absolute;
  inset: -6px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(200, 220, 255, 0.1) 0%, transparent 60%);
  animation: sun-glow-soft 10s ease-in-out infinite alternate-reverse;
}

@keyframes sun-pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.9;
  }
  50% {
    transform: scale(1.12);
    opacity: 1;
  }
}

@keyframes sun-glow-soft {
  0% {
    opacity: 0.7;
    transform: scale(1);
  }
  100% {
    opacity: 1;
    transform: scale(1.3);
  }
}

.cloud {
  position: absolute;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.85);
  box-shadow: 0 0 6px 1px rgba(255, 255, 255, 0.5);
  filter: blur(0.3px);
}

.cloud::before,
.cloud::after {
  content: '';
  position: absolute;
  border-radius: 50%;
  background: inherit;
  box-shadow: inherit;
}

.cloud-1 {
  width: 3px;
  height: 3px;
  top: 18%;
  left: 30%;
  animation: star-twinkle 3.5s ease-in-out infinite;
}
.cloud-1::before {
  width: 2px;
  height: 2px;
  top: 10px;
  left: 14px;
}
.cloud-1::after {
  width: 2px;
  height: 2px;
  top: 24px;
  left: -8px;
}

.cloud-2 {
  width: 2px;
  height: 2px;
  top: 28%;
  left: 56%;
  animation: star-twinkle 4.5s ease-in-out infinite;
  animation-delay: -1.5s;
}
.cloud-2::before {
  width: 2px;
  height: 2px;
  top: 14px;
  left: 10px;
}
.cloud-2::after {
  width: 3px;
  height: 3px;
  top: -10px;
  left: 18px;
}

.cloud-3 {
  width: 2px;
  height: 2px;
  top: 12%;
  left: 72%;
  animation: star-twinkle 5s ease-in-out infinite;
  animation-delay: -3s;
  opacity: 0.8;
}
.cloud-3::before {
  width: 3px;
  height: 3px;
  top: 18px;
  left: -10px;
}
.cloud-3::after {
  width: 2px;
  height: 2px;
  top: -8px;
  left: 12px;
}

@keyframes star-twinkle {
  0%,
  100% {
    opacity: 0.3;
    transform: scale(0.8);
  }
  50% {
    opacity: 1;
    transform: scale(1.25);
  }
}

.particle {
  position: absolute;
  border-radius: 50%;
  background: rgba(255, 240, 150, 0.85);
  box-shadow: 0 0 6px 1px rgba(255, 230, 130, 0.6);
  filter: blur(0.5px);
}

.particle-1 {
  width: 6px;
  height: 6px;
  top: 20%;
  left: 20%;
  animation: float-up 12s ease-in-out infinite;
}
.particle-2 {
  width: 4px;
  height: 4px;
  top: 30%;
  left: 70%;
  animation: float-up 15s ease-in-out infinite reverse;
  animation-delay: -3s;
}
.particle-3 {
  width: 5px;
  height: 5px;
  top: 15%;
  left: 45%;
  animation: float-up 10s ease-in-out infinite;
  animation-delay: -6s;
}
.particle-4 {
  width: 3px;
  height: 3px;
  top: 45%;
  left: 25%;
  animation: float-up 18s ease-in-out infinite reverse;
  animation-delay: -8s;
}
.particle-5 {
  width: 7px;
  height: 7px;
  top: 25%;
  left: 85%;
  animation: float-up 14s ease-in-out infinite;
  animation-delay: -4s;
}
.particle-6 {
  width: 4px;
  height: 4px;
  top: 35%;
  left: 10%;
  animation: float-up 11s ease-in-out infinite reverse;
  animation-delay: -10s;
}

@keyframes float-up {
  0%,
  100% {
    transform: translateY(0) translateX(0) scale(1);
    opacity: 0.3;
  }
  25% {
    transform: translateY(-30px) translateX(10px) scale(1.2);
    opacity: 0.7;
  }
  50% {
    transform: translateY(-50px) translateX(-5px) scale(0.9);
    opacity: 0.5;
  }
  75% {
    transform: translateY(-20px) translateX(15px) scale(1.1);
    opacity: 0.8;
  }
}

@media (max-width: 480px) {
  .cloud-3 {
    display: none;
  }

  .particle {
    opacity: 0.15;
  }
}

@media (prefers-reduced-motion: reduce) {
  .cloud,
  .particle,
  .sun-core,
  .sun-glow-1,
  .sun-glow-2 {
    animation: none !important;
  }
}
</style>
