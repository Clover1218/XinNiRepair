<script>
	export default {
		onLaunch: function() {
			console.log('App Launch')
			// 按微信官方要求: 不强制登录, 展示服务内容, 等用户操作时再引导登录
		},
		onShow: function() {
			console.log('App Show')
		},
		onHide: function() {
			console.log('App Hide')
		}
	}
</script>

<style>
	/*每个页面公共css */
	page {
		/* ===== 苍岭绿设计体系（移植自 Web 端 web-frontend/src/style.css） ===== */
		/* 中性 / 背景 */
		--xn-bg: #f4f6f8;
		--xn-surface: #ffffff;
		--xn-border: #e5e7eb;
		--xn-border-soft: #f0f2f5;
		--xn-selected: #e8ecf0;
		--xn-tag-bg: #eef2f4;

		/* 主色：苍岭绿（仅用于主按钮 / 金额 / 选中态 / 已完成） */
		--xn-primary: #0f766e;
		--xn-primary-deep: #1e4d40;
		--xn-primary-soft: #e7f3f1;

		/* 文字四级 */
		--xn-text-900: #111827;
		--xn-text-700: #374151;
		--xn-text-500: #6b7280;
		--xn-text-400: #9ca3af;

		/* 状态六色：命名与 Web 端 web-frontend/src/style.css 保持一致（--st-*），
		   便于两端 CSS 直接复用。按语义区分，不用绿色深浅区分。 */
		--st-amber: #f59e0b;   /* reported 待审核 */
		--st-orange: #ea580c;  /* pending_accept 待接单 */
		--st-blue: #2563eb;    /* processing 处理中 */
		--st-teal: #0f766e;    /* completed 已完成 */
		--st-red: #dc2626;     /* rejected 已退回 */
		--st-gray: #64748b;    /* draft 草稿 / cancelled 已取消 */
		/* 兼容早期写法，旧代码引用 --xn-st-* 时仍可用 */
		--xn-st-amber: var(--st-amber);
		--xn-st-orange: var(--st-orange);
		--xn-st-blue: var(--st-blue);
		--xn-st-teal: var(--st-teal);
		--xn-st-red: var(--st-red);
		--xn-st-gray: var(--st-gray);

		/* 圆角 / 间距 */
		--xn-radius-card: 12px;
		--xn-radius-sm: 8px;
		--xn-radius-pill: 14px;
		--xn-space: 16px;
		--xn-gap: 12px;

		/* 覆盖 wot-design-uni 主题色。
		   其 $-color-theme 定义为 var(--wot-color-theme, $default-theme)，
		   因此在 page 上赋值即可全局生效，无需改编译配置。 */
		--wot-color-theme: #0f766e;

		background-color: var(--xn-bg);
		color: var(--xn-text-900);
		font-size: 28rpx;
		-webkit-font-smoothing: antialiased;
	}

	/* ===== 弹窗内 wd-textarea 背景 + 字数块定位统一 =====
	   wot 默认给 .wd-textarea__value / .wd-textarea__count 设白底($-textarea-bg)，
	   而各弹窗给组件根节点 custom-style 设了灰底(#f5f6f8)，
	   导致「外壳灰、内容区/字数块白」的色差。全部透明化，露出容器自身背景。 */
	.wd-textarea__value,
	.wd-textarea__count {
		background: transparent !important;
	}
	/* 微信原生 textarea 强制透明（同层渲染下生效），避免默认白底色块 */
	.wd-textarea__inner {
		background-color: transparent !important;
	}
	/* 字数块(0/100)：wot 默认把它绝对定位于内部 value/suffix 包裹层底部，
	   但 auto-height 的 textarea 由 JS 动态撑高，包裹层经常没跟着长到真实高度，
	   于是「底部」实际落在文字中间/顶部。这里把计数块改锚定到
	   输入框灰底容器(.wd-textarea 根)的右下角，确保永远在盒子真正底部。 */
	.wd-textarea {
		position: relative !important;
	}
	.wd-textarea__value,
	.wd-textarea__suffix {
		position: static !important;
	}
	.wd-textarea__count {
		position: absolute !important;
		right: 16rpx !important;
		bottom: 12rpx !important;
	}
	/* wd-input 同理（当前页面暂未直接使用，防御性统一，供未来弹窗表单复用） */
	.wd-input__value,
	.wd-input__count {
		background: transparent !important;
	}
</style>
