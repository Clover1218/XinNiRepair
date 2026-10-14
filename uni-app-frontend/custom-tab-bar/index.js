Component({
  properties: {
    // 微信框架注入：当前选中项在【完整 3 项】列表中的索引（0 首页 / 1 统计 / 2 我的）
    selected: {
      type: Number,
      value: 0
    },
    // 微信框架注入：pages.json 的 tabBar.list（本组件不使用，仅声明以免告警）
    list: {
      type: Array,
      value: []
    },
    // 自定义字段：是否显示中间「统计」标签。
    // 企业审核员 / 维修业务员 = true（3 个标签）；普通用户 = false（只留首页 + 我的）。
    // 微信不会管理此字段，页面通过 setData 注入，避免被框架回写 list 覆盖。
    statsVisible: {
      type: Boolean,
      value: true
    }
  },
  data: {
    renderList: []
  },
  observers: {
    // selected / statsVisible 任一变化都重建渲染列表：
    // 每项自带 on（是否选中）与 i（在完整列表中的原始索引）
    'selected, statsVisible': function (selected, statsVisible) {
      var full = [
        {
          i: 0,
          pagePath: '/pages/home/index',
          text: '首页',
          iconPath: '/static/tabbar/home.png',
          selectedIconPath: '/static/tabbar/home-active.png'
        },
        {
          i: 1,
          pagePath: '/pages/stats/index',
          text: '统计',
          iconPath: '/static/tabbar/stats.png',
          selectedIconPath: '/static/tabbar/stats-active.png'
        },
        {
          i: 2,
          pagePath: '/pages/profile/index',
          text: '我的',
          iconPath: '/static/tabbar/profile.png',
          selectedIconPath: '/static/tabbar/profile-active.png'
        }
      ]
      var out = []
      for (var k = 0; k < full.length; k++) {
        if (k === 1 && !statsVisible) continue
        out.push({
          i: full[k].i,
          pagePath: full[k].pagePath,
          text: full[k].text,
          iconPath: full[k].iconPath,
          selectedIconPath: full[k].selectedIconPath,
          on: full[k].i === selected
        })
      }
      this.setData({ renderList: out })
    }
  },
  methods: {
    switchTab: function (e) {
      var path = e.currentTarget.dataset.path
      wx.switchTab({ url: path })
    }
  }
})
