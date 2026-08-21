import{k as ee,w as T,N as W,a5 as se,c as E,E as ae,a6 as re,a7 as oe,a as V,b as c,n as N,i as a,D as Q,t as u,o as i,F as H,y as R,e as p,Z as P,f as M,h as s,_ as h,A as g,a8 as L,a9 as te,aa as ne,W as v,ab as I,ac as le}from"./vendor-vue-DEEziBfA.js";import{B as k}from"./BaseInput-BJkLdgeB.js";import{u as ie,c as z,p as de,v as ue,d as me,e as ce,_ as U}from"./index-B2xrYB5g.js";import{c as fe,f as ge,g as we}from"./card-format-BRsGwZhv.js";import{g as pe}from"./vendor-CoHGKvmS.js";import{_ as ye}from"./BaseButton.vue_vue_type_script_setup_true_lang-CBZj_31f.js";import"./vendor-axios-BFz1NM_0.js";const be=/[a-z]/,ve=/[A-Z]/,Ce=/\d/,he=/[!@#$%^&*(),.?":{}|<>_\-+=[\]\\;'/`~]/,ke=["password","123456","qwerty","abc123","111111"];function G(m){if(!m)return{score:0,level:"",valid:!1};let r=0;m.length>=6&&r++,m.length>=10&&r++;let f=0;be.test(m)&&f++,ve.test(m)&&f++,Ce.test(m)&&f++,he.test(m)&&f++,f>=2&&(r+=2),f>=3&&r++,f>=4&&r++,ke.some(C=>m.toLowerCase().includes(C))&&(r=Math.max(0,r-2));const o=r<=2?"弱":r<=4?"中":r<=6?"强":"非常强",l=r<=2?"#ef5350":r<=4?"#ffa726":r<=6?"#66bb6a":"#43a047",e=m.length>=6&&f>=2;return{score:r,level:o,color:l,valid:e}}const Me=/^\w+$/,K=Symbol("farmbot-auth-flow");function S(m,r){const f=m;return f.response?.data||{error:f.message||r}}function Re(){const m=ie(),r=z(),f=re(),o=ae();let l;ee(()=>{l!==void 0&&(clearTimeout(l),l=void 0)});const e=se({gameVersion:"",loginLinks:E(()=>r.loginPageConfig),showUpdateLog:!1,logoLoadFailed:!1,isLogin:!0,username:"",password:"",cardCode:"",error:"",success:"",loading:!1,showPasswordStrength:!1,lockoutRemaining:0,rateLimitRemaining:0,cardClaimEnabled:!1,cardClaimLoading:!1,showClaimModal:!1,claimModalContent:{success:!0,title:"",message:"",cardCode:"",days:0},showResetVerifyModal:!1,showResetPasswordModal:!1,resetUsername:"",resetCardCode:"",resetNewPassword:"",resetConfirmPassword:"",resetError:"",resetLoading:!1,resetPasswordTouched:!1,showRenewalModal:!1,renewalUsername:"",renewalCardCode:"",renewalError:"",renewalSuccess:"",renewalLoading:!1,passwordStrength:E(()=>G(e.password)),resetPasswordStrength:E(()=>G(e.resetNewPassword)),usernameValid:E(()=>{const t=e.username;return t?t.length<3?{valid:!1,message:"用户名至少3位"}:t.length>32?{valid:!1,message:"用户名最多32位"}:Me.test(t)?{valid:!0,message:""}:{valid:!1,message:"只能包含字母、数字、下划线"}:{valid:!1,message:""}}),handleSubmit:async()=>{},toggleMode:()=>{},openRenewal:()=>{},closeRenewalModal:()=>{},submitRenewal:async()=>{},openResetVerifyModal:()=>{},closeResetVerifyModal:()=>{},closeResetPasswordModal:()=>{},verifyResetPassword:async()=>{},submitResetPassword:async()=>{},claimFreeCard:async()=>{},closeClaimModal:()=>{}});function C(){if(!e.username)return e.error="请输入用户名",!1;if(!e.usernameValid.valid)return e.error=e.usernameValid.message,!1;if(!e.password)return e.error="请输入密码",!1;if(!e.isLogin){if(e.password.length<6)return e.error="密码长度至少6位",!1;if(!e.passwordStrength.valid)return e.error="密码强度不足：需包含大写字母、小写字母、数字、特殊符号中的至少两种",!1;if(!e.cardCode)return e.error="请输入卡密",!1}return!0}e.handleSubmit=async()=>{if(C()){e.loading=!0,e.error="",e.success="";try{if(e.isLogin){const t=await m.login(e.username,e.password);t.ok?(t.data?.mustChangePassword&&(e.success="登录成功！请修改默认密码以确保账户安全"),l!==void 0&&clearTimeout(l),l=setTimeout(()=>{l=void 0,o.push({name:"dashboard"})},500)):t.errorType==="rate_limit"?(e.error=t.error||"请求过于频繁，请稍后重试",t.remainingMs&&(e.rateLimitRemaining=Math.ceil(t.remainingMs/1e3))):t.errorType==="locked"?(e.error=t.error||"账户已被锁定",t.remainingMs&&(e.lockoutRemaining=Math.ceil(t.remainingMs/1e3/60))):e.error=t.error||"登录失败"}else{const t=await m.register(e.username,e.password,e.cardCode);t.ok?(e.success="注册成功，请登录",e.isLogin=!0,e.cardCode="",e.password=""):e.error=t.error||"注册失败"}}catch(t){const d=S(t,"操作异常");d.errorType==="rate_limit"?(e.error=d.error||"请求过于频繁",d.remainingMs&&(e.rateLimitRemaining=Math.ceil(d.remainingMs/1e3))):d.errorType==="locked"?(e.error=d.error||"账户已被锁定",d.remainingMs&&(e.lockoutRemaining=Math.ceil(d.remainingMs/1e3/60))):e.error=d.error||"操作异常"}finally{e.loading=!1}}},e.toggleMode=()=>{e.isLogin=!e.isLogin,e.error="",e.success="",e.showPasswordStrength=!1,e.lockoutRemaining=0,e.rateLimitRemaining=0},e.openRenewal=()=>{e.renewalUsername=e.username.trim(),e.renewalCardCode="",e.renewalError="",e.renewalSuccess="",e.showRenewalModal=!0},e.closeRenewalModal=()=>{e.renewalLoading||(e.showRenewalModal=!1,e.renewalError="",e.renewalSuccess="")},e.submitRenewal=async()=>{if(!e.renewalUsername.trim()){e.renewalError="请输入用户名";return}if(!e.renewalCardCode.trim()){e.renewalError="请输入卡密";return}e.renewalLoading=!0,e.renewalError="",e.renewalSuccess="";try{const{data:t}=await de({username:e.renewalUsername.trim(),cardCode:e.renewalCardCode.trim()});if(!t.ok){e.renewalError=t.error||"续费失败";return}const d=t.data?.cardType,_=t.data?.card;e.renewalSuccess=d==="quota"?"续费成功，账号额度已更新":`续费成功，有效期已更新${_?.expiresAt?`至 ${new Date(_.expiresAt).toLocaleString("zh-CN")}`:""}`,e.username=e.renewalUsername.trim()}catch(t){const d=S(t,"续费失败");e.renewalError=d.error||"续费失败"}finally{e.renewalLoading=!1}},e.openResetVerifyModal=()=>{e.resetUsername=e.username.trim(),e.resetCardCode="",e.resetNewPassword="",e.resetConfirmPassword="",e.resetError="",e.resetPasswordTouched=!1,e.showResetVerifyModal=!0},e.closeResetVerifyModal=()=>{e.resetLoading||(e.showResetVerifyModal=!1,e.resetError="")},e.closeResetPasswordModal=()=>{e.resetLoading||(e.showResetPasswordModal=!1,e.resetNewPassword="",e.resetConfirmPassword="",e.resetError="",e.resetPasswordTouched=!1)},e.verifyResetPassword=async()=>{if(!e.resetUsername.trim()){e.resetError="请输入用户名";return}if(!e.resetCardCode.trim()){e.resetError="请输入注册时使用的卡密";return}e.resetLoading=!0,e.resetError="";try{const{data:t}=await ue({username:e.resetUsername.trim(),cardCode:e.resetCardCode.trim()});if(!t.ok){e.resetError=t.error||"验证失败";return}e.showResetVerifyModal=!1,e.showResetPasswordModal=!0}catch(t){const d=S(t,"验证失败");e.resetError=d.error||"验证失败"}finally{e.resetLoading=!1}},e.submitResetPassword=async()=>{if(e.resetPasswordTouched=!0,!e.resetNewPassword){e.resetError="请输入新密码";return}if(e.resetNewPassword.length<6){e.resetError="密码长度至少6位";return}if(!e.resetPasswordStrength.valid){e.resetError="密码强度不足：需包含大写字母、小写字母、数字、特殊符号中的至少两种";return}if(e.resetNewPassword!==e.resetConfirmPassword){e.resetError="两次输入的密码不一致";return}e.resetLoading=!0,e.resetError="";try{const{data:t}=await me({username:e.resetUsername.trim(),cardCode:e.resetCardCode.trim(),newPassword:e.resetNewPassword});if(!t.ok){e.resetError=t.error||"重置失败";return}e.showResetPasswordModal=!1,e.username=e.resetUsername.trim(),e.password="",e.isLogin=!0,e.success="密码重置成功，请使用新密码登录",e.resetNewPassword="",e.resetConfirmPassword=""}catch(t){const d=S(t,"重置失败");e.resetError=d.error||"重置失败"}finally{e.resetLoading=!1}},e.claimFreeCard=async()=>{if(!e.cardClaimLoading){e.cardClaimLoading=!0,e.error="";try{const{data:t}=await fe(),d=t;d.ok?(e.cardCode=d.cardCode||"",e.claimModalContent={success:!0,title:"领取成功",message:`成功领取 ${ge(d)}卡密！`,cardCode:d.cardCode||"",days:d.days||0}):e.claimModalContent={success:!1,title:"领取失败",message:d.error||"领取失败，请稍后重试",cardCode:"",days:0},e.showClaimModal=!0}catch(t){const d=S(t,"领取失败");e.claimModalContent={success:!1,title:"领取失败",message:d.error||"领取失败",cardCode:"",days:0},e.showClaimModal=!0}finally{e.cardClaimLoading=!1}}},e.closeClaimModal=()=>{e.showClaimModal=!1};async function b(){try{const{data:t}=await we();e.cardClaimEnabled=t.ok&&t.enabled===!0}catch(t){console.error("检查卡密领取状态失败:",t)}}async function y(){try{const{data:t}=await ce();t.ok&&(e.gameVersion=String(t.clientVersion||""))}catch(t){console.error("获取游戏版本失败:",t)}}return T(()=>e.password,()=>{!e.isLogin&&e.password&&(e.showPasswordStrength=!0)}),T(()=>String(f.query.username||"").trim(),t=>{t&&!e.username.trim()&&(e.username=t)},{immediate:!0}),T(()=>e.loginLinks.logoUrl,()=>{e.logoLoadFailed=!1}),W(()=>{b(),y(),r.fetchLoginPageConfig()}),e}function Pe(){const m=oe(K);if(!m)throw new Error("useAuthFlowContext must be used inside Login.vue");return m}const _e={class:"strength-bar"},Se=V({__name:"PasswordStrengthMeter",props:{strength:{},compact:{type:Boolean,default:!1}},setup(m){return(r,f)=>(i(),c("div",{class:N(["password-strength",{compact:m.compact}])},[a("div",_e,[a("div",{class:"strength-fill",style:Q({width:`${Math.min(m.strength.score*12.5,100)}%`,backgroundColor:m.strength.color})},null,4)]),a("span",{class:"strength-text",style:Q({color:m.strength.color})},u(m.strength.level),5)],2))}}),Y=U(Se,[["__scopeId","data-v-49ae948e"]]),Ee={class:"claim-modal"},Le={class:"claim-modal-header"},$e={class:"claim-modal-icon"},Ve={class:"claim-modal-title"},Ue={class:"claim-modal-body"},Te={class:"claim-modal-message"},Ie={key:0,class:"claim-modal-card-info"},Ne={class:"card-code-value"},xe={class:"claim-modal-footer"},Ae={class:"claim-modal reset-modal"},Be={key:0,class:"reset-modal-error"},Fe={key:1,class:"reset-modal-success"},qe={class:"reset-modal-actions"},De=["disabled"],Oe=["disabled"],je={class:"claim-modal reset-modal"},Qe={key:0,class:"reset-modal-error"},Ge={class:"reset-modal-actions"},We=["disabled"],He=["disabled"],ze={class:"claim-modal reset-modal"},Ke={key:1,class:"reset-modal-error"},Ye={class:"reset-modal-actions"},Ze=["disabled"],Je=["disabled"],Xe=V({__name:"LoginModals",setup(m){const r=Pe();return(f,o)=>(i(),c(H,null,[(i(),R(L,{to:"body"},[p(P,{name:"modal"},{default:M(()=>[s(r).showClaimModal?(i(),c("div",{key:0,class:"claim-modal-overlay",onClick:o[1]||(o[1]=h(l=>s(r).closeClaimModal(),["self"]))},[a("div",Ee,[a("div",Le,[a("span",$e,u(s(r).claimModalContent.success?"🎉":"⚠️"),1),a("h3",Ve,u(s(r).claimModalContent.title),1)]),a("div",Ue,[a("p",Te,u(s(r).claimModalContent.message),1),s(r).claimModalContent.success&&s(r).claimModalContent.cardCode?(i(),c("div",Ie,[o[18]||(o[18]=a("div",{class:"card-code-label"}," 卡密已自动填入 ",-1)),a("div",Ne,u(s(r).claimModalContent.cardCode),1)])):g("",!0)]),a("div",xe,[a("button",{class:"claim-modal-btn",onClick:o[0]||(o[0]=l=>s(r).closeClaimModal())},u(s(r).claimModalContent.success?"开始注册":"我知道了"),1)])])])):g("",!0)]),_:1})])),(i(),R(L,{to:"body"},[p(P,{name:"modal"},{default:M(()=>[s(r).showRenewalModal?(i(),c("div",{key:0,class:"claim-modal-overlay",onClick:o[6]||(o[6]=h(l=>s(r).closeRenewalModal(),["self"]))},[a("div",Ae,[o[20]||(o[20]=a("div",{class:"claim-modal-header"},[a("span",{class:"claim-modal-icon"},[a("span",{class:"i-carbon-ticket"})]),a("h3",{class:"claim-modal-title"}," 账号续费 ")],-1)),a("form",{class:"reset-modal-body",onSubmit:o[5]||(o[5]=h(l=>s(r).submitRenewal(),["prevent"]))},[o[19]||(o[19]=a("p",{class:"reset-modal-tip"}," 输入用户名和续费卡密，确认后会直接为该账号续费。 ",-1)),p(k,{modelValue:s(r).renewalUsername,"onUpdate:modelValue":o[2]||(o[2]=l=>s(r).renewalUsername=l),label:"用户名",placeholder:"请输入用户名"},null,8,["modelValue"]),p(k,{modelValue:s(r).renewalCardCode,"onUpdate:modelValue":o[3]||(o[3]=l=>s(r).renewalCardCode=l),label:"续费卡密",placeholder:"请输入续费卡密"},null,8,["modelValue"]),s(r).renewalError?(i(),c("div",Be,u(s(r).renewalError),1)):g("",!0),s(r).renewalSuccess?(i(),c("div",Fe,u(s(r).renewalSuccess),1)):g("",!0),a("div",qe,[a("button",{type:"button",class:"claim-modal-btn secondary",disabled:s(r).renewalLoading,onClick:o[4]||(o[4]=l=>s(r).closeRenewalModal())},u(s(r).renewalSuccess?"关闭":"取消"),9,De),a("button",{type:"submit",class:"claim-modal-btn",disabled:s(r).renewalLoading},u(s(r).renewalLoading?"续费中...":"确认续费"),9,Oe)])],32)])])):g("",!0)]),_:1})])),(i(),R(L,{to:"body"},[p(P,{name:"modal"},{default:M(()=>[s(r).showResetVerifyModal?(i(),c("div",{key:0,class:"claim-modal-overlay",onClick:o[11]||(o[11]=h(l=>s(r).closeResetVerifyModal(),["self"]))},[a("div",je,[o[22]||(o[22]=a("div",{class:"claim-modal-header"},[a("span",{class:"claim-modal-icon"},[a("span",{class:"i-carbon-password"})]),a("h3",{class:"claim-modal-title"}," 找回密码 ")],-1)),a("form",{class:"reset-modal-body",onSubmit:o[10]||(o[10]=h(l=>s(r).verifyResetPassword(),["prevent"]))},[o[21]||(o[21]=a("p",{class:"reset-modal-tip"}," 输入用户名和注册时使用的卡密，通过验证后即可设置新密码。 ",-1)),p(k,{modelValue:s(r).resetUsername,"onUpdate:modelValue":o[7]||(o[7]=l=>s(r).resetUsername=l),label:"用户名",placeholder:"请输入用户名"},null,8,["modelValue"]),p(k,{modelValue:s(r).resetCardCode,"onUpdate:modelValue":o[8]||(o[8]=l=>s(r).resetCardCode=l),label:"卡密",placeholder:"请输入注册时使用的卡密"},null,8,["modelValue"]),s(r).resetError?(i(),c("div",Qe,u(s(r).resetError),1)):g("",!0),a("div",Ge,[a("button",{type:"button",class:"claim-modal-btn secondary",disabled:s(r).resetLoading,onClick:o[9]||(o[9]=l=>s(r).closeResetVerifyModal())}," 取消 ",8,We),a("button",{type:"submit",class:"claim-modal-btn",disabled:s(r).resetLoading},u(s(r).resetLoading?"验证中...":"验证"),9,He)])],32)])])):g("",!0)]),_:1})])),(i(),R(L,{to:"body"},[p(P,{name:"modal"},{default:M(()=>[s(r).showResetPasswordModal?(i(),c("div",{key:0,class:"claim-modal-overlay",onClick:o[17]||(o[17]=h(l=>s(r).closeResetPasswordModal(),["self"]))},[a("div",ze,[o[23]||(o[23]=a("div",{class:"claim-modal-header"},[a("span",{class:"claim-modal-icon"},[a("span",{class:"i-carbon-security"})]),a("h3",{class:"claim-modal-title"}," 设置新密码 ")],-1)),a("form",{class:"reset-modal-body",onSubmit:o[16]||(o[16]=h(l=>s(r).submitResetPassword(),["prevent"]))},[p(k,{modelValue:s(r).resetNewPassword,"onUpdate:modelValue":o[12]||(o[12]=l=>s(r).resetNewPassword=l),label:"新密码",type:"password",placeholder:"请输入新密码",onInput:o[13]||(o[13]=l=>s(r).resetPasswordTouched=!0)},null,8,["modelValue"]),s(r).resetPasswordTouched&&s(r).resetNewPassword?(i(),R(Y,{key:0,strength:s(r).resetPasswordStrength},null,8,["strength"])):g("",!0),p(k,{modelValue:s(r).resetConfirmPassword,"onUpdate:modelValue":o[14]||(o[14]=l=>s(r).resetConfirmPassword=l),label:"确认密码",type:"password",placeholder:"请再次输入新密码"},null,8,["modelValue"]),s(r).resetError?(i(),c("div",Ke,u(s(r).resetError),1)):g("",!0),a("div",Ye,[a("button",{type:"button",class:"claim-modal-btn secondary",disabled:s(r).resetLoading,onClick:o[15]||(o[15]=l=>s(r).closeResetPasswordModal())}," 取消 ",8,Ze),a("button",{type:"submit",class:"claim-modal-btn",disabled:s(r).resetLoading},u(s(r).resetLoading?"提交中...":"确认修改"),9,Je)])],32)])])):g("",!0)]),_:1})]))],64))}}),es=U(Xe,[["__scopeId","data-v-8464d8c8"]]),ss=`# QQ 农场更新日志

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
`,as={class:"update-log-panel"},rs={class:"update-log-header"},os={class:"update-log-body"},ts=["innerHTML"],ns={class:"update-log-footer"},ls=V({__name:"UpdateLogModal",props:{show:{type:Boolean}},emits:["close"],setup(m,{emit:r}){const f=r,o=z(),l=E(()=>pe.parse(ss,{gfm:!0,breaks:!1}));function e(C){C.key==="Escape"&&f("close")}return W(()=>window.addEventListener("keydown",e)),te(()=>window.removeEventListener("keydown",e)),(C,b)=>(i(),R(L,{to:"body"},[p(P,{name:"update-log-modal"},{default:M(()=>[m.show?(i(),c("div",{key:0,class:"update-log-overlay",role:"dialog","aria-modal":"true","aria-labelledby":"update-log-title",onClick:b[2]||(b[2]=h(y=>f("close"),["self"]))},[a("section",as,[a("header",rs,[a("div",null,[b[3]||(b[3]=a("h2",{id:"update-log-title"}," 更新日志 ",-1)),a("p",null,u(s(o).loginPageConfig.title||"QQ农场智能助手")+"版本记录",1)]),a("button",{type:"button",class:"update-log-close","aria-label":"关闭更新日志",title:"关闭",onClick:b[0]||(b[0]=y=>f("close"))},[...b[4]||(b[4]=[a("span",{class:"i-carbon-close"},null,-1)])])]),a("div",os,[a("article",{class:"markdown-content",innerHTML:l.value},null,8,ts)]),a("footer",ns,[a("button",{type:"button",onClick:b[1]||(b[1]=y=>f("close"))}," 关闭 ")])])])):g("",!0)]),_:1})]))}}),is=U(ls,[["__scopeId","data-v-a9810b1d"]]),ds={class:"auth-page"},us={class:"auth-shell"},ms={class:"auth-brand"},cs={class:"auth-card"},fs={class:"auth-tabs",role:"tablist","aria-label":"账号操作"},gs={class:"auth-card-body"},ws={class:"auth-intro"},ps={class:"auth-intro-kicker"},ys={class:"auth-field"},bs={key:0,class:"auth-field-error"},vs={class:"auth-field"},Cs={key:0,class:"auth-field"},hs={class:"auth-label-row"},ks=["disabled"],Ms={key:0,class:"i-svg-spinners-90-ring-with-bg"},Rs={key:0},Ps={key:1},_s={key:0,class:"i-carbon-arrow-right"},Ss={class:"auth-actions"},Es={key:0,class:"auth-secondary-actions"},Ls={class:"auth-footer"},$s={class:"auth-footer-links"},Vs=["href"],Us=["href"],Ts=V({__name:"Login",setup(m){const r=Re();le(K,r);const{gameVersion:f,loginLinks:o,showUpdateLog:l,username:e,password:C,cardCode:b,isLogin:y,error:t,success:d,loading:_,showPasswordStrength:Z,lockoutRemaining:x,rateLimitRemaining:A,cardClaimEnabled:J,cardClaimLoading:B,passwordStrength:X,usernameValid:F}=ne(r),{handleSubmit:q,toggleMode:$,openResetVerifyModal:D,openRenewal:O,claimFreeCard:j}=r;return(Is,n)=>(i(),c("div",ds,[a("div",us,[a("header",ms,[n[13]||(n[13]=a("div",{class:"auth-brand-mark"},[a("span",{class:"i-carbon-sprout"})],-1)),a("div",null,[a("strong",null,u(s(o).title||"QQ农场智能助手"),1),n[12]||(n[12]=a("span",null,"FARMBOT / CONTROL CENTER",-1))])]),a("section",cs,[n[24]||(n[24]=a("div",{class:"auth-card-heading"},[a("div",{class:"auth-card-kicker"}," 账号访问 "),a("div",{class:"auth-card-status"},[a("span",{class:"status-dot status-dot--live"}),v(" SECURE SESSION ")])],-1)),a("div",fs,[a("button",{type:"button",class:N({"auth-tab--active":s(y)}),onClick:n[0]||(n[0]=w=>!s(y)&&s($)())}," 登录 ",2),a("button",{type:"button",class:N({"auth-tab--active":!s(y)}),onClick:n[1]||(n[1]=w=>s(y)&&s($)())}," 注册 ",2)]),a("div",gs,[a("div",ws,[a("span",ps,u(s(y)?"WELCOME BACK":"NEW WORKSPACE"),1),a("h1",null,u(s(y)?"欢迎回来":"创建账号"),1),a("p",null,u(s(y)?s(o).loginSubtitle||"登录后继续管理你的农场":s(o).registerSubtitle||"创建账号，开始使用 FarmBot"),1)]),a("form",{class:"auth-form",onSubmit:n[6]||(n[6]=h((...w)=>s(q)&&s(q)(...w),["prevent"]))},[a("div",ys,[n[14]||(n[14]=a("label",{for:"username"},[a("span",{class:"i-carbon-user"}),v("用户名")],-1)),p(k,{id:"username",modelValue:s(e),"onUpdate:modelValue":n[2]||(n[2]=w=>I(e)?e.value=w:null),type:"text",placeholder:"请输入用户名",required:""},null,8,["modelValue"]),s(e)&&!s(F).valid?(i(),c("p",bs,u(s(F).message),1)):g("",!0)]),a("div",vs,[n[15]||(n[15]=a("label",{for:"password"},[a("span",{class:"i-carbon-password"}),v("密码")],-1)),p(k,{id:"password",modelValue:s(C),"onUpdate:modelValue":n[3]||(n[3]=w=>I(C)?C.value=w:null),type:"password",placeholder:"请输入密码",required:""},null,8,["modelValue"]),s(Z)&&s(C)?(i(),R(Y,{key:0,strength:s(X),compact:""},null,8,["strength"])):g("",!0)]),s(y)?g("",!0):(i(),c("div",Cs,[a("div",hs,[n[18]||(n[18]=a("label",{for:"cardCode"},[a("span",{class:"i-carbon-ticket"}),v("卡密")],-1)),s(J)?(i(),c("button",{key:0,type:"button",class:"auth-free-card",disabled:s(B),onClick:n[4]||(n[4]=(...w)=>s(j)&&s(j)(...w))},[s(B)?(i(),c("span",Ms)):(i(),c(H,{key:1},[n[16]||(n[16]=a("span",{class:"i-carbon-gift"},null,-1)),n[17]||(n[17]=v("免费领取 ",-1))],64))],8,ks)):g("",!0)]),p(k,{id:"cardCode",modelValue:s(b),"onUpdate:modelValue":n[5]||(n[5]=w=>I(b)?b.value=w:null),type:"text",placeholder:"请输入卡密",required:!s(y)},null,8,["modelValue","required"])])),p(P,{name:"auth-message"},{default:M(()=>[s(t)?(i(),c("div",{key:`error-${s(t)}`,class:"auth-message auth-message--error"},[n[19]||(n[19]=a("span",{class:"i-carbon-warning-alt"},null,-1)),a("span",null,[v(u(s(t)),1),s(x)>0?(i(),c("small",Rs,u(s(x))+" 分钟后解锁",1)):g("",!0),s(A)>0?(i(),c("small",Ps,u(s(A))+" 秒后可重试",1)):g("",!0)])])):g("",!0)]),_:1}),p(P,{name:"auth-message"},{default:M(()=>[s(d)?(i(),c("div",{key:`success-${s(d)}`,class:"auth-message auth-message--success"},[n[20]||(n[20]=a("span",{class:"i-carbon-checkmark-filled"},null,-1)),v(u(s(d)),1)])):g("",!0)]),_:1}),p(ye,{type:"submit",variant:"primary",block:"",loading:s(_),class:"auth-submit"},{default:M(()=>[a("span",null,u(s(y)?"登录工作台":"创建账号"),1),s(_)?g("",!0):(i(),c("span",_s))]),_:1},8,["loading"])],32),a("div",Ss,[a("button",{type:"button",class:"auth-switch",onClick:n[7]||(n[7]=(...w)=>s($)&&s($)(...w))},[a("span",null,u(s(y)?"没有账号？":"已有账号？"),1),v(u(s(y)?"立即注册":"立即登录"),1),n[21]||(n[21]=a("span",{class:"i-carbon-arrow-right"},null,-1))]),s(y)?(i(),c("div",Es,[a("button",{type:"button",onClick:n[8]||(n[8]=(...w)=>s(D)&&s(D)(...w))},[...n[22]||(n[22]=[a("span",{class:"i-carbon-reset"},null,-1),v("忘记密码 ",-1)])]),a("button",{type:"button",onClick:n[9]||(n[9]=(...w)=>s(O)&&s(O)(...w))},[...n[23]||(n[23]=[a("span",{class:"i-carbon-renew"},null,-1),v("账号续费 ",-1)])])])):g("",!0)])])]),a("footer",Ls,[a("div",$s,[s(o).purchaseUrl?(i(),c("a",{key:0,href:s(o).purchaseUrl},[...n[25]||(n[25]=[a("span",{class:"i-carbon-shopping-cart"},null,-1),v("购买卡密",-1)])],8,Vs)):g("",!0),s(o).qqGroupUrl?(i(),c("a",{key:1,href:s(o).qqGroupUrl,target:"_blank",rel:"noopener noreferrer"},[...n[26]||(n[26]=[a("span",{class:"i-carbon-logo-qq"},null,-1),v("加入QQ群",-1)])],8,Us)):g("",!0),a("button",{type:"button",onClick:n[10]||(n[10]=w=>l.value=!0)},[...n[27]||(n[27]=[a("span",{class:"i-carbon-document"},null,-1),v("更新日志 ",-1)])])]),a("span",null,u(s(f)?`游戏版本 ${s(f)}`:"FarmBot operations"),1)])]),p(es),p(is,{show:s(l),onClose:n[11]||(n[11]=w=>l.value=!1)},null,8,["show"])]))}}),Os=U(Ts,[["__scopeId","data-v-d7f81fdc"]]);export{Os as default};
