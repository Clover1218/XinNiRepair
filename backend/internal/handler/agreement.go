package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ── HTML 模板: 公共 head + body 容器 ──
const htmlHead = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
  body { font-family: -apple-system, "Helvetica Neue", sans-serif; line-height: 1.8; color: #333; max-width: 680px; margin: 0 auto; padding: 24px 16px 48px; }
  h1 { font-size: 22px; text-align: center; margin-bottom: 8px; }
  h2 { font-size: 17px; margin-top: 28px; border-bottom: 1px solid #eee; padding-bottom: 6px; }
  p { font-size: 15px; margin: 12px 0; }
  ul { padding-left: 20px; }
  li { font-size: 15px; margin: 6px 0; }
  .text-muted { color: #999; font-size: 13px; text-align: center; margin-top: 4px; }
</style>
</head>
<body>
`

const htmlTail = `
</body>
</html>`

// ── 用户协议 HTML 内容 ──
const userAgreementHTML = `
<h1>用户服务协议</h1>
<p class="text-muted">生效日期：2026年8月24日</p>

<p>温州市龙湾永中新泥百电脑经营部（以下简称"新泥百电脑"）根据以下服务条款为您服务。</p>
<p>欢迎您使用"新泥百电脑"小程序！本协议由<strong>您（以下称"用户"）</strong>与<strong>新泥百电脑</strong>共同签订。请您在使用本服务前仔细阅读并确认接受本协议的全部内容，您点击"同意"或实际使用本服务，即视为您已充分理解并接受本协议。</p>

<h2>一、服务说明</h2>
<p>新泥百电脑致力于为企业客户提供便捷的电脑维修报修服务。本小程序（以下简称"本服务"）核心功能包括：</p>
<ul>
  <li><strong>企业管理</strong>：企业管理员可创建企业账号，并管理本企业员工。</li>
  <li><strong>加入企业</strong>：员工通过企业码或管理员邀请加入企业，获得报修权限。</li>
  <li><strong>在线报修</strong>：员工提交维修申请，填写报修类别、门牌号、联系电话等信息。</li>
  <li><strong>上门维修</strong>：维修工程师根据报修信息上门提供维修服务。</li>
  <li><strong>进度跟踪</strong>：报修人可查询维修订单的处理状态。</li>
</ul>

<h2>二、用户角色与账号</h2>
<ul>
  <li><strong>维修工程师</strong>：负责创建企业、邀请员工、查看企业所有报修记录。</li>
  <li><strong>报修人</strong>：需先加入企业，方可提交报修申请。</li>
</ul>
<p>您授权获取的微信昵称、头像及手机号，将用于身份识别和服务沟通。</p>

<h2>三、服务规则</h2>
<ul>
  <li><strong>报修信息真实性</strong>：您应如实填写报修类别、故障描述、门牌号及联系电话。因信息不实导致的维修延误或失败，由您自行承担责任。</li>
  <li><strong>服务范围</strong>：上门维修服务仅限企业注册地址所在城市。超出服务范围，我们有权拒绝接单。</li>
  <li><strong>费用说明</strong>：维修费用以工程师现场检测后报价为准，服务前会与您确认。</li>
  <li><strong>禁止行为</strong>：您不得利用本服务提交虚假报修、恶意刷单或干扰平台正常运行。</li>
</ul>

<h2>四、企业管理的特别约定</h2>
<p>维修工程师有权查看企业所有员工的报修记录，以便进行内部管理和费用核算。</p>
<p>当员工退出企业后，该员工将无法查看历史报修记录，但维修工程师仍可查看。</p>

<h2>五、服务变更与终止</h2>
<p>若您违反本协议，我们有权暂停或终止向您提供服务。</p>
<p>我们保留因业务调整、技术升级等原因修改或暂停服务的权利。</p>

<h2>六、免责声明</h2>
<p>维修服务由技术人员提供，因设备硬件老化、数据丢失等非人为因素造成的损失，我们不予赔偿。请报修人提前备份重要数据。</p>
<p>本服务按"现状"提供，我们不保证服务永无中断或错误。</p>

<h2>七、知识产权</h2>
<p>本小程序的所有内容（包括文字、图片、界面设计等）的知识产权归新泥百所有，未经许可不得使用。</p>

<h2>八、协议修改</h2>
<p>我们有权适时修改本协议，修改后的协议将公布于小程序内。若您继续使用，视为接受修改。</p>

<h2>九、争议解决</h2>
<p>因本协议引起的争议，双方应友好协商解决；协商不成的，提交至新泥百电脑所在地人民法院诉讼解决。</p>

<h2>十、联系方式</h2>
<p>联系邮箱：Q121666464@qq.com</p>
`

// ── 隐私政策 HTML 内容 ──
const privacyPolicyHTML = `
<h1>隐私政策</h1>
<p class="text-muted">生效日期：2026年8月24日</p>

<p>温州市龙湾永中新泥百电脑经营部（以下简称"新泥百电脑"）深知个人信息对您的重要性，将严格遵守法律法规保护您的隐私安全。</p>

<h2>一、我们收集的信息及用途</h2>
<p>我们仅会出于以下目的，收集和使用您的个人信息：</p>
<ul>
  <li><strong>微信昵称、头像</strong>：用于身份识别，区分报修人。显示在报修工单中，便于工程师确认身份。</li>
  <li><strong>手机号</strong>：维修前与您沟通确认，发送维修进度通知，紧急情况联系（仅用于服务沟通，不用于营销）</li>
  <li><strong>报修信息</strong>（设备类型、故障描述、故障图片、门牌号、联系人等）：为您提供上门维修服务，用于维修工程师安排时间、准备配件、跟踪进度。</li>
  <li><strong>企业信息</strong>（您所属的企业名称）：确认您的报修权限，便于管理。</li>
</ul>

<h2>二、信息收集的合法依据</h2>
<p>信息是为您提供报修服务所必需的；</p>
<p>您主动填写并提交报修信息，视为明示同意我们收集。</p>

<h2>三、数据存储与保护</h2>
<p><strong>存储地点</strong>：您的个人信息将存储于中华人民共和国境内。</p>
<p><strong>存储期限</strong>：维修完成后，订单信息保留2年用于售后查询，期满后匿名化处理。</p>
<p><strong>安全措施</strong>：我们采用数据加密、访问权限控制等措施保护您的信息安全。</p>

<h2>四、信息共享</h2>
<p>我们承诺不会出售或出租您的个人信息。仅在以下情况共享：</p>
<ul>
  <li>为完成您的维修服务，您的联系方式、门牌号等信息将提供给接单的维修工程师。</li>
  <li>维修工程师可查看企业的报修记录（含报修人信息）。</li>
  <li>法律法规要求或行政/司法机关依法要求提供。</li>
</ul>

<h2>五、您的权利</h2>
<p>您有权：</p>
<ul>
  <li><strong>访问和更正</strong>：在"我的报修"中查看订单信息，如有误可联系客服修改。</li>
  <li><strong>退出企业</strong>：可自行退出所属企业，退出后将无法提交报修。</li>
  <li><strong>撤回授权</strong>：在微信"设置"->"授权管理"中撤回授权。</li>
  <li><strong>注销账号</strong>：联系我们申请注销，我们将删除您的个人信息（法律法规要求保留的除外）。</li>
</ul>

<h2>六、未成年人保护</h2>
<p>本服务主要面向企业员工，不主动收集14周岁以下儿童信息。</p>

<h2>七、政策更新</h2>
<p>我们可能适时修订本政策，并通过小程序公告、弹窗等方式通知您。</p>

<h2>八、联系我们</h2>
<p>联系邮箱：Q121666464@qq.com</p>
`

// UserAgreement 返回用户服务协议 HTML (GET /agreement/user)
// 公开接口, 不需要 JWT 认证
func UserAgreement(c *gin.Context) {
	html := htmlHead + userAgreementHTML + htmlTail
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// PrivacyPolicy 返回隐私政策 HTML (GET /agreement/privacy)
// 公开接口, 不需要 JWT 认证
func PrivacyPolicy(c *gin.Context) {
	html := htmlHead + privacyPolicyHTML + htmlTail
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
