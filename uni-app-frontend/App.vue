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

	/* 弹窗内 wd-textarea：微信原生 textarea 默认白底，强制与外层灰底统一，
	   修复「内容区白、四周灰」的色差（覆盖 wot 组件内 .wd-textarea__inner 默认 transparent）。 */
	.wd-textarea__inner {
		background-color: #f5f6f8 !important;
	}
</style>
