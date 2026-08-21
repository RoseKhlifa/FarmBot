import{o as i,b as c,N as ee,k as se,w as U,E as H,O as ae,c as $,y as oe,D as re,P as te,a as V,n as z,i as a,Q as G,t as m,F as ne,I as M,e as f,J as P,f as _,h as s,K as k,B as g,L as S,G as le,S as ie,A as y,U as x,V as de}from"./vendor-vue-lKpSKBc8.js";import{_ as E,b as ce,e as W,p as ue,v as me,f as ge,h as pe}from"./index-tqNlRxW1.js";import{B as h}from"./BaseInput-DVi9Xijp.js";import{c as fe,f as we,g as ve}from"./card-format-CCzYXzCP.js";import{g as be}from"./vendor-CoHGKvmS.js";import{_ as ye}from"./BaseButton.vue_vue_type_script_setup_true_lang-J0yLu1Jv.js";import"./vendor-axios-BFz1NM_0.js";const Ce={},ke={class:"bg-decoration","aria-hidden":"true"};function he(u,o){return i(),c("div",ke,[...o[0]||(o[0]=[ee('<div class="sky-gradient" data-v-e81aa645></div><div class="sun-decoration" data-v-e81aa645><div class="sun-core" data-v-e81aa645></div><div class="sun-glow-1" data-v-e81aa645></div><div class="sun-glow-2" data-v-e81aa645></div></div><div class="cloud cloud-1" data-v-e81aa645></div><div class="cloud cloud-2" data-v-e81aa645></div><div class="cloud cloud-3" data-v-e81aa645></div><div class="particle particle-1" data-v-e81aa645></div><div class="particle particle-2" data-v-e81aa645></div><div class="particle particle-3" data-v-e81aa645></div><div class="particle particle-4" data-v-e81aa645></div><div class="particle particle-5" data-v-e81aa645></div><div class="particle particle-6" data-v-e81aa645></div>',11)])])}const _e=E(Ce,[["render",he],["__scopeId","data-v-e81aa645"]]),Me=/[a-z]/,Pe=/[A-Z]/,Re=/\d/,Le=/[!@#$%^&*(),.?":{}|<>_\-+=[\]\\;'/`~]/,$e=["password","123456","qwerty","abc123","111111"];function O(u){if(!u)return{score:0,level:"",valid:!1};let o=0;u.length>=6&&o++,u.length>=10&&o++;let p=0;Me.test(u)&&p++,Pe.test(u)&&p++,Re.test(u)&&p++,Le.test(u)&&p++,p>=2&&(o+=2),p>=3&&o++,p>=4&&o++,$e.some(b=>u.toLowerCase().includes(b))&&(o=Math.max(0,o-2));const r=o<=2?"弱":o<=4?"中":o<=6?"强":"非常强",l=o<=2?"#ef5350":o<=4?"#ffa726":o<=6?"#66bb6a":"#43a047",e=u.length>=6&&p>=2;return{score:o,level:r,color:l,valid:e}}const Se=/^\w+$/,Y=Symbol("farmbot-auth-flow");function L(u,o){const p=u;return p.response?.data||{error:p.message||o}}function Ee(){const u=ce(),o=W(),p=re(),r=oe();let l;se(()=>{l!==void 0&&(clearTimeout(l),l=void 0)});const e=ae({gameVersion:"",loginLinks:$(()=>o.loginPageConfig),showUpdateLog:!1,logoLoadFailed:!1,isLogin:!0,username:"",password:"",cardCode:"",error:"",success:"",loading:!1,showPasswordStrength:!1,lockoutRemaining:0,rateLimitRemaining:0,cardClaimEnabled:!1,cardClaimLoading:!1,showClaimModal:!1,claimModalContent:{success:!0,title:"",message:"",cardCode:"",days:0},showResetVerifyModal:!1,showResetPasswordModal:!1,resetUsername:"",resetCardCode:"",resetNewPassword:"",resetConfirmPassword:"",resetError:"",resetLoading:!1,resetPasswordTouched:!1,showRenewalModal:!1,renewalUsername:"",renewalCardCode:"",renewalError:"",renewalSuccess:"",renewalLoading:!1,passwordStrength:$(()=>O(e.password)),resetPasswordStrength:$(()=>O(e.resetNewPassword)),usernameValid:$(()=>{const t=e.username;return t?t.length<3?{valid:!1,message:"用户名至少3位"}:t.length>32?{valid:!1,message:"用户名最多32位"}:Se.test(t)?{valid:!0,message:""}:{valid:!1,message:"只能包含字母、数字、下划线"}:{valid:!1,message:""}}),handleSubmit:async()=>{},toggleMode:()=>{},openRenewal:()=>{},closeRenewalModal:()=>{},submitRenewal:async()=>{},openResetVerifyModal:()=>{},closeResetVerifyModal:()=>{},closeResetPasswordModal:()=>{},verifyResetPassword:async()=>{},submitResetPassword:async()=>{},claimFreeCard:async()=>{},closeClaimModal:()=>{}});function b(){if(!e.username)return e.error="请输入用户名",!1;if(!e.usernameValid.valid)return e.error=e.usernameValid.message,!1;if(!e.password)return e.error="请输入密码",!1;if(!e.isLogin){if(e.password.length<6)return e.error="密码长度至少6位",!1;if(!e.passwordStrength.valid)return e.error="密码强度不足：需包含大写字母、小写字母、数字、特殊符号中的至少两种",!1;if(!e.cardCode)return e.error="请输入卡密",!1}return!0}e.handleSubmit=async()=>{if(b()){e.loading=!0,e.error="",e.success="";try{if(e.isLogin){const t=await u.login(e.username,e.password);t.ok?(t.data?.mustChangePassword&&(e.success="登录成功！请修改默认密码以确保账户安全"),l!==void 0&&clearTimeout(l),l=setTimeout(()=>{l=void 0,r.push({name:"dashboard"})},500)):t.errorType==="rate_limit"?(e.error=t.error||"请求过于频繁，请稍后重试",t.remainingMs&&(e.rateLimitRemaining=Math.ceil(t.remainingMs/1e3))):t.errorType==="locked"?(e.error=t.error||"账户已被锁定",t.remainingMs&&(e.lockoutRemaining=Math.ceil(t.remainingMs/1e3/60))):e.error=t.error||"登录失败"}else{const t=await u.register(e.username,e.password,e.cardCode);t.ok?(e.success="注册成功，请登录",e.isLogin=!0,e.cardCode="",e.password=""):e.error=t.error||"注册失败"}}catch(t){const d=L(t,"操作异常");d.errorType==="rate_limit"?(e.error=d.error||"请求过于频繁",d.remainingMs&&(e.rateLimitRemaining=Math.ceil(d.remainingMs/1e3))):d.errorType==="locked"?(e.error=d.error||"账户已被锁定",d.remainingMs&&(e.lockoutRemaining=Math.ceil(d.remainingMs/1e3/60))):e.error=d.error||"操作异常"}finally{e.loading=!1}}},e.toggleMode=()=>{e.isLogin=!e.isLogin,e.error="",e.success="",e.showPasswordStrength=!1,e.lockoutRemaining=0,e.rateLimitRemaining=0},e.openRenewal=()=>{e.renewalUsername=e.username.trim(),e.renewalCardCode="",e.renewalError="",e.renewalSuccess="",e.showRenewalModal=!0},e.closeRenewalModal=()=>{e.renewalLoading||(e.showRenewalModal=!1,e.renewalError="",e.renewalSuccess="")},e.submitRenewal=async()=>{if(!e.renewalUsername.trim()){e.renewalError="请输入用户名";return}if(!e.renewalCardCode.trim()){e.renewalError="请输入卡密";return}e.renewalLoading=!0,e.renewalError="",e.renewalSuccess="";try{const{data:t}=await ue({username:e.renewalUsername.trim(),cardCode:e.renewalCardCode.trim()});if(!t.ok){e.renewalError=t.error||"续费失败";return}const d=t.data?.cardType,R=t.data?.card;e.renewalSuccess=d==="quota"?"续费成功，账号额度已更新":`续费成功，有效期已更新${R?.expiresAt?`至 ${new Date(R.expiresAt).toLocaleString("zh-CN")}`:""}`,e.username=e.renewalUsername.trim()}catch(t){const d=L(t,"续费失败");e.renewalError=d.error||"续费失败"}finally{e.renewalLoading=!1}},e.openResetVerifyModal=()=>{e.resetUsername=e.username.trim(),e.resetCardCode="",e.resetNewPassword="",e.resetConfirmPassword="",e.resetError="",e.resetPasswordTouched=!1,e.showResetVerifyModal=!0},e.closeResetVerifyModal=()=>{e.resetLoading||(e.showResetVerifyModal=!1,e.resetError="")},e.closeResetPasswordModal=()=>{e.resetLoading||(e.showResetPasswordModal=!1,e.resetNewPassword="",e.resetConfirmPassword="",e.resetError="",e.resetPasswordTouched=!1)},e.verifyResetPassword=async()=>{if(!e.resetUsername.trim()){e.resetError="请输入用户名";return}if(!e.resetCardCode.trim()){e.resetError="请输入注册时使用的卡密";return}e.resetLoading=!0,e.resetError="";try{const{data:t}=await me({username:e.resetUsername.trim(),cardCode:e.resetCardCode.trim()});if(!t.ok){e.resetError=t.error||"验证失败";return}e.showResetVerifyModal=!1,e.showResetPasswordModal=!0}catch(t){const d=L(t,"验证失败");e.resetError=d.error||"验证失败"}finally{e.resetLoading=!1}},e.submitResetPassword=async()=>{if(e.resetPasswordTouched=!0,!e.resetNewPassword){e.resetError="请输入新密码";return}if(e.resetNewPassword.length<6){e.resetError="密码长度至少6位";return}if(!e.resetPasswordStrength.valid){e.resetError="密码强度不足：需包含大写字母、小写字母、数字、特殊符号中的至少两种";return}if(e.resetNewPassword!==e.resetConfirmPassword){e.resetError="两次输入的密码不一致";return}e.resetLoading=!0,e.resetError="";try{const{data:t}=await ge({username:e.resetUsername.trim(),cardCode:e.resetCardCode.trim(),newPassword:e.resetNewPassword});if(!t.ok){e.resetError=t.error||"重置失败";return}e.showResetPasswordModal=!1,e.username=e.resetUsername.trim(),e.password="",e.isLogin=!0,e.success="密码重置成功，请使用新密码登录",e.resetNewPassword="",e.resetConfirmPassword=""}catch(t){const d=L(t,"重置失败");e.resetError=d.error||"重置失败"}finally{e.resetLoading=!1}},e.claimFreeCard=async()=>{if(!e.cardClaimLoading){e.cardClaimLoading=!0,e.error="";try{const{data:t}=await fe(),d=t;d.ok?(e.cardCode=d.cardCode||"",e.claimModalContent={success:!0,title:"领取成功",message:`成功领取 ${we(d)}卡密！`,cardCode:d.cardCode||"",days:d.days||0}):e.claimModalContent={success:!1,title:"领取失败",message:d.error||"领取失败，请稍后重试",cardCode:"",days:0},e.showClaimModal=!0}catch(t){const d=L(t,"领取失败");e.claimModalContent={success:!1,title:"领取失败",message:d.error||"领取失败",cardCode:"",days:0},e.showClaimModal=!0}finally{e.cardClaimLoading=!1}}},e.closeClaimModal=()=>{e.showClaimModal=!1};async function v(){try{const{data:t}=await ve();e.cardClaimEnabled=t.ok&&t.enabled===!0}catch(t){console.error("检查卡密领取状态失败:",t)}}async function C(){try{const{data:t}=await pe();t.ok&&(e.gameVersion=String(t.clientVersion||""))}catch(t){console.error("获取游戏版本失败:",t)}}return U(()=>e.password,()=>{!e.isLogin&&e.password&&(e.showPasswordStrength=!0)}),U(()=>String(p.query.username||"").trim(),t=>{t&&!e.username.trim()&&(e.username=t)},{immediate:!0}),U(()=>e.loginLinks.logoUrl,()=>{e.logoLoadFailed=!1}),H(()=>{v(),C(),o.fetchLoginPageConfig()}),e}function Ve(){const u=te(Y);if(!u)throw new Error("useAuthFlowContext must be used inside Login.vue");return u}const Ue={class:"strength-bar"},xe=V({__name:"PasswordStrengthMeter",props:{strength:{},compact:{type:Boolean,default:!1}},setup(u){return(o,p)=>(i(),c("div",{class:z(["password-strength",{compact:u.compact}])},[a("div",Ue,[a("div",{class:"strength-fill",style:G({width:`${Math.min(u.strength.score*12.5,100)}%`,backgroundColor:u.strength.color})},null,4)]),a("span",{class:"strength-text",style:G({color:u.strength.color})},m(u.strength.level),5)],2))}}),K=E(xe,[["__scopeId","data-v-49ae948e"]]),Ie={class:"claim-modal"},Te={class:"claim-modal-header"},Ne={class:"claim-modal-icon"},Ae={class:"claim-modal-title"},Be={class:"claim-modal-body"},qe={class:"claim-modal-message"},Fe={key:0,class:"claim-modal-card-info"},Qe={class:"card-code-value"},De={class:"claim-modal-footer"},je={class:"claim-modal reset-modal"},Ge={key:0,class:"reset-modal-error"},Oe={key:1,class:"reset-modal-success"},He={class:"reset-modal-actions"},ze=["disabled"],We=["disabled"],Ye={class:"claim-modal reset-modal"},Ke={key:0,class:"reset-modal-error"},Je={class:"reset-modal-actions"},Ze=["disabled"],Xe=["disabled"],es={class:"claim-modal reset-modal"},ss={key:1,class:"reset-modal-error"},as={class:"reset-modal-actions"},os=["disabled"],rs=["disabled"],ts=V({__name:"LoginModals",setup(u){const o=Ve();return(p,r)=>(i(),c(ne,null,[(i(),M(S,{to:"body"},[f(P,{name:"modal"},{default:_(()=>[s(o).showClaimModal?(i(),c("div",{key:0,class:"claim-modal-overlay",onClick:r[1]||(r[1]=k(l=>s(o).closeClaimModal(),["self"]))},[a("div",Ie,[a("div",Te,[a("span",Ne,m(s(o).claimModalContent.success?"🎉":"⚠️"),1),a("h3",Ae,m(s(o).claimModalContent.title),1)]),a("div",Be,[a("p",qe,m(s(o).claimModalContent.message),1),s(o).claimModalContent.success&&s(o).claimModalContent.cardCode?(i(),c("div",Fe,[r[18]||(r[18]=a("div",{class:"card-code-label"}," 卡密已自动填入 ",-1)),a("div",Qe,m(s(o).claimModalContent.cardCode),1)])):g("",!0)]),a("div",De,[a("button",{class:"claim-modal-btn",onClick:r[0]||(r[0]=l=>s(o).closeClaimModal())},m(s(o).claimModalContent.success?"开始注册":"我知道了"),1)])])])):g("",!0)]),_:1})])),(i(),M(S,{to:"body"},[f(P,{name:"modal"},{default:_(()=>[s(o).showRenewalModal?(i(),c("div",{key:0,class:"claim-modal-overlay",onClick:r[6]||(r[6]=k(l=>s(o).closeRenewalModal(),["self"]))},[a("div",je,[r[20]||(r[20]=a("div",{class:"claim-modal-header"},[a("span",{class:"claim-modal-icon"},[a("span",{class:"i-carbon-ticket"})]),a("h3",{class:"claim-modal-title"}," 账号续费 ")],-1)),a("form",{class:"reset-modal-body",onSubmit:r[5]||(r[5]=k(l=>s(o).submitRenewal(),["prevent"]))},[r[19]||(r[19]=a("p",{class:"reset-modal-tip"}," 输入用户名和续费卡密，确认后会直接为该账号续费。 ",-1)),f(h,{modelValue:s(o).renewalUsername,"onUpdate:modelValue":r[2]||(r[2]=l=>s(o).renewalUsername=l),label:"用户名",placeholder:"请输入用户名"},null,8,["modelValue"]),f(h,{modelValue:s(o).renewalCardCode,"onUpdate:modelValue":r[3]||(r[3]=l=>s(o).renewalCardCode=l),label:"续费卡密",placeholder:"请输入续费卡密"},null,8,["modelValue"]),s(o).renewalError?(i(),c("div",Ge,m(s(o).renewalError),1)):g("",!0),s(o).renewalSuccess?(i(),c("div",Oe,m(s(o).renewalSuccess),1)):g("",!0),a("div",He,[a("button",{type:"button",class:"claim-modal-btn secondary",disabled:s(o).renewalLoading,onClick:r[4]||(r[4]=l=>s(o).closeRenewalModal())},m(s(o).renewalSuccess?"关闭":"取消"),9,ze),a("button",{type:"submit",class:"claim-modal-btn",disabled:s(o).renewalLoading},m(s(o).renewalLoading?"续费中...":"确认续费"),9,We)])],32)])])):g("",!0)]),_:1})])),(i(),M(S,{to:"body"},[f(P,{name:"modal"},{default:_(()=>[s(o).showResetVerifyModal?(i(),c("div",{key:0,class:"claim-modal-overlay",onClick:r[11]||(r[11]=k(l=>s(o).closeResetVerifyModal(),["self"]))},[a("div",Ye,[r[22]||(r[22]=a("div",{class:"claim-modal-header"},[a("span",{class:"claim-modal-icon"},[a("span",{class:"i-carbon-password"})]),a("h3",{class:"claim-modal-title"}," 找回密码 ")],-1)),a("form",{class:"reset-modal-body",onSubmit:r[10]||(r[10]=k(l=>s(o).verifyResetPassword(),["prevent"]))},[r[21]||(r[21]=a("p",{class:"reset-modal-tip"}," 输入用户名和注册时使用的卡密，通过验证后即可设置新密码。 ",-1)),f(h,{modelValue:s(o).resetUsername,"onUpdate:modelValue":r[7]||(r[7]=l=>s(o).resetUsername=l),label:"用户名",placeholder:"请输入用户名"},null,8,["modelValue"]),f(h,{modelValue:s(o).resetCardCode,"onUpdate:modelValue":r[8]||(r[8]=l=>s(o).resetCardCode=l),label:"卡密",placeholder:"请输入注册时使用的卡密"},null,8,["modelValue"]),s(o).resetError?(i(),c("div",Ke,m(s(o).resetError),1)):g("",!0),a("div",Je,[a("button",{type:"button",class:"claim-modal-btn secondary",disabled:s(o).resetLoading,onClick:r[9]||(r[9]=l=>s(o).closeResetVerifyModal())}," 取消 ",8,Ze),a("button",{type:"submit",class:"claim-modal-btn",disabled:s(o).resetLoading},m(s(o).resetLoading?"验证中...":"验证"),9,Xe)])],32)])])):g("",!0)]),_:1})])),(i(),M(S,{to:"body"},[f(P,{name:"modal"},{default:_(()=>[s(o).showResetPasswordModal?(i(),c("div",{key:0,class:"claim-modal-overlay",onClick:r[17]||(r[17]=k(l=>s(o).closeResetPasswordModal(),["self"]))},[a("div",es,[r[23]||(r[23]=a("div",{class:"claim-modal-header"},[a("span",{class:"claim-modal-icon"},[a("span",{class:"i-carbon-security"})]),a("h3",{class:"claim-modal-title"}," 设置新密码 ")],-1)),a("form",{class:"reset-modal-body",onSubmit:r[16]||(r[16]=k(l=>s(o).submitResetPassword(),["prevent"]))},[f(h,{modelValue:s(o).resetNewPassword,"onUpdate:modelValue":r[12]||(r[12]=l=>s(o).resetNewPassword=l),label:"新密码",type:"password",placeholder:"请输入新密码",onInput:r[13]||(r[13]=l=>s(o).resetPasswordTouched=!0)},null,8,["modelValue"]),s(o).resetPasswordTouched&&s(o).resetNewPassword?(i(),M(K,{key:0,strength:s(o).resetPasswordStrength},null,8,["strength"])):g("",!0),f(h,{modelValue:s(o).resetConfirmPassword,"onUpdate:modelValue":r[14]||(r[14]=l=>s(o).resetConfirmPassword=l),label:"确认密码",type:"password",placeholder:"请再次输入新密码"},null,8,["modelValue"]),s(o).resetError?(i(),c("div",ss,m(s(o).resetError),1)):g("",!0),a("div",as,[a("button",{type:"button",class:"claim-modal-btn secondary",disabled:s(o).resetLoading,onClick:r[15]||(r[15]=l=>s(o).closeResetPasswordModal())}," 取消 ",8,os),a("button",{type:"submit",class:"claim-modal-btn",disabled:s(o).resetLoading},m(s(o).resetLoading?"提交中...":"确认修改"),9,rs)])],32)])])):g("",!0)]),_:1})]))],64))}}),ns=E(ts,[["__scopeId","data-v-8464d8c8"]]),ls=`# QQ 农场更新日志

这里记录最近更新了什么。

## 2026-07-31（Aoluis1005 维护分支）

### 新增功能

- **观星礼录（二十八星宿·每日馈赠）**：活动中心新增「观星」页签，展示 28 星宿每日进度、已解锁/已领取/可领取状态与奖励明细；支持一键领取全部已解锁星宿奖励，并带「自动领取」开关（localStorage 持久化，进入页签自动领一次）。后端新增 \`GET /api/activity/guanxing\`、\`POST /api/activity/guanxing/claim\`，解析活动 GetGroup/Operate 回包（星宿数据字段 110、扩展字段 119 贴合官方客户端）。
- **星纱（SAIJI）商店 16 件商品图标**：14 件装扮（萤火/月光小屋、街道、狗屋、木牌、仓库、栅栏、围栏）与 2 个头像框（萤火星房 2156 / 月光营地 2157）接入 \`skinDetail\` 干净图标（\`icon_skin_*\` / \`img_avatar_S2_*\`），\`getItemImageById\` 按 itemId 前缀自动解析，\`/api/activity/helu\` 的 \`exchangeShop.items[].image\` 全部有图。
- **植物数据表扩充**：\`ItemInfo.json\` 700 条（+101：星纱货币 1019/1021/1022/1023、头像框 2152-2157、新种子等）、\`Plant.json\` 255 条（+50 新植物）；新增 13 张 \`Crop_*_Seed.png\` 种子图标与 \`scripts/extract_seed_icons.py\` 提取脚本。
- **种子/果实名称 Plant.json 兜底**：\`getPlantNameOrNull\` / \`getPlantBySeedId\` / \`getPlantByFruitId\` 加入 \`gameConfig.js\`；仓库、背包、图鉴、土地分析等在 \`ItemInfo\` 缺失时统一回退到 \`Plant.json\`（含变异果实 \`1040xxx→1120xxx\` 换算）。

### Bug 修复

- **活动图标名称优先级**：\`HeluDrawPanel\` 等面板的兜底首字优先取 \`item.name\`（中文真名），\`itemName\` 兜底。
- **仓库种子判定统一**：\`warehouse.js\` 改为以 \`Plant.json\` 为唯一种子/果实判定依据，删除旧的启发式兜底路径，记录疑似漏配种子便于补表。

### 优化

- **月光营地滤镜**：萤火/月光两套皮肤共用同一张 \`skinDetail\` 图，\`ActivityItemImage.vue\` 对名称含「月光」的物品叠加冷色调滤镜（\`hue-rotate-180 brightness-90 saturate-125\`），视觉上区分两套装扮。

## 2026-07-27（Aoluis1005 维护分支）

### 新增功能

- **个人生涯统计弹窗**：点击概览页微信头像即可弹出，展示玩家头像 / 昵称 / 等级 / 经验 / 角色编号，以及「历史累计收获」与「累计摘取好友作物」两项统计；下方为收获明细网格（含前三名金银铜标牌）。新增后端 \`gamepb.careerpb.CareerService/CareerInfoGet\` 接口与 \`GET /api/career\`（需 \`x-admin-token\` 与 \`x-account-id\`）。

### Bug 修复

- **修复生涯弹窗收获列表为空**：后端 \`proto.js\` 全局开启 \`keepCase:true\`，而 \`career-api.js\` 误用 \`fruitId\` / \`statsTotal\` 等 camelCase 字段，导致解码取不到数值、列表被过滤为空；已统一改为 \`fruit_id\` / \`stats_total\` / \`level_stats\` / \`achieved_levels\` 等 snake_case，实测返回 174 条收获数据。
- **修复好友页面导致服务崩溃 / 反复重启**：\`admin.js\` 注册好友路由时漏传 \`getAccountIdFromRequest\` 等依赖，打开「好友」页面调用 \`/api/friends\` 会抛出 \`TypeError\` 使整个 Node 进程退出，触发容器重启、登录掉线、账号自动停止；已补全路由依赖，并新增 \`process.on('unhandledRejection')\` 全局守卫，单个坏请求不再拖垮全服。
- **修复生涯弹窗 API 超时体验**：后端 \`sendMsgAsync\` 超时由 20s 降到 10s，路由出错时返回 \`{ok:false, error}\`（不再误报 \`ok:true\` 造成弹窗空白）；前端超时提示文案改为「加载超时，请确认该账号已在游戏中上线后重试」并保留重试按钮。

### 优化

- **生涯弹窗移动端体验**：改为居中毛玻璃卡片（\`backdrop-blur-2xl\` + 半透明底色），四周留白、不再贴边/顶屏；层级抬到 \`z-[1100]\` 避免被底部悬浮导航栏遮挡；并隐藏滚动条（保留滚动能力），排版更美观。

## 2026-07-22（Aoluis1005 维护分支）

### Bug 修复

- 修复应用宝离线重连「无限重连」：重连失败分支（\`ws_reconnect_failed\`）在停 Worker 时无条件清空了重连计数，导致「重连 N 次后停止」永远不触发、日志恒显示 \`(1/3)\`。现改为 \`stopWorker\` 仅在手动停止 / 踢下线 / 删除账号时清零计数，自动重连路径保留计数跨周期累积，达到上限后真正停止重连。
- 修复重连日志在面板「常驻最下方」：账号日志时间戳为 \`YYYY-MM-DD HH:mm:ss\` 空格格式，浏览器 \`Date.parse\` 解析为 \`NaN\` 后回退成 \`Date.now()\`，使旧日志被错误顶到列表底部、不随新日志上移。现统一转成 ISO（\`T\` 分隔）再解析，并给账号日志补充数字 \`ts\` 字段，排序归位正常。

## 2026-07-19（Aoluis1005 维护分支）

### Bug 修复

- 修复好友「经验满只帮护主犬」模式失效、无差别帮助所有人的问题（经验额度数据缺失时不再误重置）。
- 修复自动重连失控：移除 Worker 内自动重连（指数退避重试），仅保留主进程的应用宝离线重连；并重连计数不再被 worker 启动清零，达到上限后真正停止。
- 修复悬浮导航栏「自动避让弹窗检测」引发的 dock 永久消失 bug：移除基于几何判定的自动检测（误判常驻 fixed 元素为弹窗），dock 回归稳定长驻；保留手动收起（把手/小药丸）与「更多」面板打开时自动收起。

### 优化

- 应用宝接口配置：API Token 不再写死，改为用户自行输入；接口地址保留可更改的默认值；appId 不再写死。
- 好友交互效率提升：移除预检查、并行帮助、批量偷菜、巡查期好友列表缓存、巡逻时收集狗信息等。
- 悬浮导航栏：稳定长驻底部；支持手动收起/展开（把手 + 小药丸），打开「更多」面板时自动收起。

## 2026-07-12

### 新增功能

- 新增设置方案上传云端，新账号可直接导入设置方案。
- 新增多账号一键同步设置方案。
- 新增抓包登录。
- 新增黄金虫图标、投放与清除。

### 优化

- 活动抽奖完成通知不再挡住右上角账号切换。

## 2026-07-11

### 新增功能

- 活动中心支持一次兑换多个化肥或有机化肥。
- 登录页支持自定义图标、标题和提示语。
- 新增购买卡密、加入 QQ 群和更新日志入口。

### Bug 修复

- 修复 QQ 会员每日礼包领取。

### 优化

- 运行日志只保留最近三天，减少日志占用。

## 2026-07-06

### 新增功能

- 新增四格作物优先种植，会自动安排合适的土地。
- 自动捣蛋连续失败后会暂停到第二天，避免一直报错。
- 新账号默认优先使用背包种子，也可以调整种子顺序。

### Bug 修复

- 修复好友农场相关操作。

### 优化

- 优化商城移动端布局、账号切换和网络稳定性。
- 精简运行日志，果实出售结果显示更清楚。

## 2026-07-04

### 新增功能

- 新增最终阶段施肥策略。
- 购买种子和化肥时可以选择数量，也可以按余额自动计算最多可买多少。
- 放虫放草连续失败后会自动停止。

### 优化

- 金币、点券、金豆豆数量显示更简洁。

## 2026-07-03

### Bug 修复

- 修复管理员弹窗在小屏幕上显示不完整的问题。

### 优化

- 优化手机端页面滚动和顶部栏显示。

## 2026-07-02

### 新增功能

- 新增“青梅酿万金”活动及青梅酿酒功能。
- 账号切换支持显示账号名称和头像。

### Bug 修复

- 修复满级后好友帮助异常的问题。

### 优化

- 优化界面和头像显示。
- 优化活动页移动端显示。
`,is={class:"update-log-panel"},ds={class:"update-log-header"},cs={class:"update-log-body"},us=["innerHTML"],ms={class:"update-log-footer"},gs=V({__name:"UpdateLogModal",props:{show:{type:Boolean}},emits:["close"],setup(u,{emit:o}){const p=o,r=W(),l=$(()=>be.parse(ls,{gfm:!0,breaks:!1}));function e(b){b.key==="Escape"&&p("close")}return H(()=>window.addEventListener("keydown",e)),le(()=>window.removeEventListener("keydown",e)),(b,v)=>(i(),M(S,{to:"body"},[f(P,{name:"update-log-modal"},{default:_(()=>[u.show?(i(),c("div",{key:0,class:"update-log-overlay",role:"dialog","aria-modal":"true","aria-labelledby":"update-log-title",onClick:v[2]||(v[2]=k(C=>p("close"),["self"]))},[a("section",is,[a("header",ds,[a("div",null,[v[3]||(v[3]=a("h2",{id:"update-log-title"}," 更新日志 ",-1)),a("p",null,m(s(r).loginPageConfig.title||"QQ农场智能助手")+"版本记录",1)]),a("button",{type:"button",class:"update-log-close","aria-label":"关闭更新日志",title:"关闭",onClick:v[0]||(v[0]=C=>p("close"))},[...v[4]||(v[4]=[a("span",{class:"i-carbon-close"},null,-1)])])]),a("div",cs,[a("article",{class:"markdown-content",innerHTML:l.value},null,8,us)]),a("footer",ms,[a("button",{type:"button",onClick:v[1]||(v[1]=C=>p("close"))}," 关闭 ")])])])):g("",!0)]),_:1})]))}}),ps=E(gs,[["__scopeId","data-v-a9810b1d"]]),fs={class:"login-container"},ws={class:"login-card"},vs={class:"logo-area"},bs={class:"logo-icon-wrapper"},ys={class:"logo-icon"},Cs=["src","alt"],ks={key:1,class:"i-carbon-sprout text-3xl"},hs={class:"logo-title"},_s={class:"logo-subtitle"},Ms={class:"form-group"},Ps={class:"input-wrapper"},Rs={key:0,class:"form-hint error"},Ls={class:"form-group"},$s={class:"input-wrapper"},Ss={class:"message-content"},Es={key:0,class:"lockout-timer"},Vs={key:1,class:"lockout-timer"},Us={key:0,class:"form-group"},xs={key:0,class:"mb-2"},Is=["disabled"],Ts={key:0,class:"i-svg-spinners-90-ring-with-bg"},Ns={key:1},As={class:"input-wrapper"},Bs={key:0,class:"submit-text"},qs={class:"switch-btn-icon"},Fs={key:0,class:"login-quick-actions"},Qs={class:"card-footer"},Ds={class:"footer-info"},js=["href"],Gs=["href"],Os={key:0,class:"game-version"},Hs=V({__name:"Login",setup(u){const o=Ee();de(Y,o);const{gameVersion:p,loginLinks:r,showUpdateLog:l,logoLoadFailed:e,isLogin:b,username:v,password:C,cardCode:t,error:d,success:R,loading:I,showPasswordStrength:J,lockoutRemaining:T,rateLimitRemaining:N,cardClaimEnabled:Z,cardClaimLoading:A,passwordStrength:X,usernameValid:B}=ie(o),{handleSubmit:q,toggleMode:F,openResetVerifyModal:Q,openRenewal:D,claimFreeCard:j}=o;return(zs,n)=>(i(),c("div",fs,[f(_e),a("main",ws,[n[28]||(n[28]=a("div",{class:"card-glow"},null,-1)),a("div",vs,[a("div",bs,[n[11]||(n[11]=a("div",{class:"logo-ring-1"},null,-1)),n[12]||(n[12]=a("div",{class:"logo-ring-2"},null,-1)),a("div",ys,[s(r).logoUrl&&!s(e)?(i(),c("img",{key:0,src:s(r).logoUrl,alt:`${s(r).title||"QQ农场智能助手"}图标`,class:"logo-image",onError:n[0]||(n[0]=w=>e.value=!0)},null,40,Cs)):(i(),c("div",ks))])]),a("h1",hs,m(s(r).title||"QQ农场智能助手"),1),a("p",_s,m(s(b)?s(r).loginSubtitle||"欢迎回来，开启智慧农耕之旅":s(r).registerSubtitle||"创建账号，开启智慧农耕之旅"),1)]),a("form",{class:"form-area",onSubmit:n[5]||(n[5]=k((...w)=>s(q)&&s(q)(...w),["prevent"]))},[a("div",Ms,[n[14]||(n[14]=a("label",{class:"form-label"},[a("span",{class:"label-icon i-carbon-user"}),y(" 用户名 ")],-1)),a("div",Ps,[n[13]||(n[13]=a("span",{class:"input-icon i-carbon-user"},null,-1)),f(h,{id:"username",modelValue:s(v),"onUpdate:modelValue":n[1]||(n[1]=w=>x(v)?v.value=w:null),type:"text",placeholder:"请输入用户名",required:""},null,8,["modelValue"])]),s(v)&&!s(B).valid?(i(),c("p",Rs,m(s(B).message),1)):g("",!0)]),a("div",Ls,[n[18]||(n[18]=a("label",{class:"form-label"},[a("span",{class:"label-icon i-carbon-password"}),y(" 密码 ")],-1)),a("div",$s,[n[15]||(n[15]=a("span",{class:"input-icon i-carbon-password"},null,-1)),f(h,{id:"password",modelValue:s(C),"onUpdate:modelValue":n[2]||(n[2]=w=>x(C)?C.value=w:null),type:"password",placeholder:"请输入密码",required:""},null,8,["modelValue"])]),s(J)&&s(C)?(i(),M(K,{key:0,strength:s(X),compact:""},null,8,["strength"])):g("",!0),f(P,{name:"msg-slide"},{default:_(()=>[s(d)?(i(),c("div",{key:`error-${s(d)}`,class:"message error-message"},[n[16]||(n[16]=a("span",{class:"message-icon i-carbon-warning"},null,-1)),a("div",Ss,[y(m(s(d))+" ",1),s(T)>0?(i(),c("span",Es," ("+m(s(T))+" 分钟后解锁) ",1)):g("",!0),s(N)>0?(i(),c("span",Vs," ("+m(s(N))+" 秒后可重试) ",1)):g("",!0)])])):g("",!0)]),_:1}),f(P,{name:"msg-slide"},{default:_(()=>[s(R)?(i(),c("div",{key:`success-${s(R)}`,class:"message success-message"},[n[17]||(n[17]=a("span",{class:"message-icon i-carbon-checkmark-filled"},null,-1)),y(" "+m(s(R)),1)])):g("",!0)]),_:1})]),s(b)?g("",!0):(i(),c("div",Us,[n[21]||(n[21]=a("label",{class:"form-label"},[a("span",{class:"label-icon i-carbon-ticket"}),y(" 卡密 ")],-1)),s(Z)?(i(),c("div",xs,[a("button",{type:"button",class:"claim-card-btn",disabled:s(A),onClick:n[3]||(n[3]=(...w)=>s(j)&&s(j)(...w))},[s(A)?(i(),c("span",Ts)):(i(),c("span",Ns,[...n[19]||(n[19]=[a("span",{class:"sparkle-icon"},"✦",-1),y(" 免费领取卡密 ",-1)])]))],8,Is)])):g("",!0),a("div",As,[n[20]||(n[20]=a("span",{class:"input-icon i-carbon-ticket"},null,-1)),f(h,{id:"cardCode",modelValue:s(t),"onUpdate:modelValue":n[4]||(n[4]=w=>x(t)?t.value=w:null),type:"text",placeholder:"请输入卡密",required:!s(b)},null,8,["modelValue","required"])])])),f(ye,{type:"submit",variant:"primary",block:"",loading:s(I),class:"submit-btn"},{default:_(()=>[s(I)?g("",!0):(i(),c("span",Bs,[n[22]||(n[22]=a("span",{class:"submit-icon i-carbon-login"},null,-1)),y(" "+m(s(b)?"登 录":"注 册"),1)]))]),_:1},8,["loading"])],32),a("div",{class:z(["switch-area",{"single-action":!s(b)}])},[a("button",{type:"button",class:"switch-btn",onClick:n[6]||(n[6]=(...w)=>s(F)&&s(F)(...w))},[a("span",qs,m(s(b)?"→":"←"),1),y(" "+m(s(b)?"没有账号？立即注册":"已有账号？立即登录"),1)]),s(b)?(i(),c("div",Fs,[a("button",{type:"button",class:"quick-action-btn",onClick:n[7]||(n[7]=(...w)=>s(Q)&&s(Q)(...w))},[...n[23]||(n[23]=[a("span",{class:"i-carbon-reset"},null,-1),y(" 忘记密码 ",-1)])]),a("button",{type:"button",class:"quick-action-btn",onClick:n[8]||(n[8]=(...w)=>s(D)&&s(D)(...w))},[...n[24]||(n[24]=[a("span",{class:"i-carbon-renew"},null,-1),y(" 账号续费 ",-1)])])])):g("",!0)],2),a("div",Qs,[a("div",Ds,[s(r).purchaseUrl?(i(),c("a",{key:0,href:s(r).purchaseUrl,class:"footer-link purchase-link"},[...n[25]||(n[25]=[a("span",{class:"i-carbon-shopping-cart"},null,-1),y(" 购买卡密 ",-1)])],8,js)):g("",!0),s(r).qqGroupUrl?(i(),c("a",{key:1,href:s(r).qqGroupUrl,target:"_blank",rel:"noopener noreferrer",class:"footer-link qq-group-link"},[...n[26]||(n[26]=[a("span",{class:"i-carbon-logo-qq"},null,-1),y(" 加入QQ群 ",-1)])],8,Gs)):g("",!0),a("button",{type:"button",class:"footer-link update-log-link",onClick:n[9]||(n[9]=w=>l.value=!0)},[...n[27]||(n[27]=[a("span",{class:"i-carbon-document"},null,-1),y(" 更新日志 ",-1)])])]),s(p)?(i(),c("div",Os," 当前游戏版本："+m(s(p)),1)):g("",!0)])]),f(ns),f(ps,{show:s(l),onClose:n[10]||(n[10]=w=>l.value=!1)},null,8,["show"])]))}}),sa=E(Hs,[["__scopeId","data-v-66db68b6"]]);export{sa as default};
