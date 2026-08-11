/** weapp-qrcode 类型声明（该包无内置类型） */
declare module 'weapp-qrcode' {
  interface DrawQrcodeOptions {
    /** 二维码宽度（px），需与 canvas 宽度一致 */
    width: number
    /** 二维码高度（px），需与 canvas 高度一致 */
    height: number
    /** 绘制的 canvasId */
    canvasId?: string
    /** 绘图上下文（wx.createCanvasContext），v1.0.0+ 支持 */
    ctx?: unknown
    /** 二维码内容 */
    text: string
    /** 二维码计算模式，默认 -1 */
    typeNumber?: number
    /** 纠错级别 { L:1, M:0, Q:3, H:2 }，默认 H */
    correctLevel?: number
    /** 背景颜色，默认 #ffffff */
    background?: string
    /** 前景颜色，默认 #000000 */
    foreground?: string
    /** 组件内使用时传入 this */
    _this?: unknown
    /** 绘制完成回调 */
    callback?: (e?: unknown) => void
    /** 二维码绘制的 x 起始位置，默认 0 */
    x?: number
    /** 二维码绘制的 y 起始位置，默认 0 */
    y?: number
    /** 在二维码上绘制图片（层级高于二维码） */
    image?: {
      imageResource: string
      dx: number
      dy: number
      dWidth: number
      dHeight: number
    }
  }

  export default function drawQrcode(options: DrawQrcodeOptions): void
}
