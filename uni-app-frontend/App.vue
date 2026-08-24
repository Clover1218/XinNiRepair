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
		background-color: #f5f6f8;
		font-size: 28rpx;
		color: #333;
		-webkit-font-smoothing: antialiased;
	}
</style>
