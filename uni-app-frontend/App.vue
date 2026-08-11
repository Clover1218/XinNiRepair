<script>
	export default {
		onLaunch: function() {
			console.log('App Launch')
			// 启动时校验登录态，未登录跳转登录页
			const token = uni.getStorageSync('token')
			if (!token) {
				uni.reLaunch({ url: '/pages/auth/login' })
			}
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
