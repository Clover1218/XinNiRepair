// 轻量 SFC 校验：用 @vue/compiler-sfc 解析并编译 <script> 与 <template>，
// 捕获语法错误（足以发现编辑引入的破坏）。用法: node validate_sfc.cjs <file.vue>
const { parse, compileScript, compileTemplate } = require('@vue/compiler-sfc')
const fs = require('fs')
const path = require('path')

const file = process.argv[2]
if (!file) { console.error('usage: node validate_sfc.cjs <file.vue>'); process.exit(2) }
const source = fs.readFileSync(file, 'utf8')
const filename = path.basename(file)

const { descriptor, errors: parseErrors } = parse(source, { filename })
let failed = false

if (parseErrors && parseErrors.length) {
  failed = true
  console.error(`[PARSE] ${filename} 解析错误:`)
  for (const e of parseErrors) console.error('  ' + (e.message || e))
}

// 编译 <script> / <script setup>
for (const block of [descriptor.script, descriptor.scriptSetup]) {
  if (!block) continue
  try {
    const r = compileScript(descriptor, { id: filename, inlineTemplate: false })
    if (r.errors && r.errors.length) {
      failed = true
      console.error(`[SCRIPT] ${filename} 编译错误:`)
      for (const e of r.errors) console.error('  ' + (e.message || e))
    }
  } catch (e) {
    failed = true
    console.error(`[SCRIPT] ${filename} 抛出异常: ${e.message}`)
  }
}

// 编译 <template>
if (descriptor.template) {
  try {
    const r = compileTemplate({
      source: descriptor.template.content,
      filename,
      id: filename,
      compilerOptions: { bindingMetadata: {} },
    })
    if (r.errors && r.errors.length) {
      failed = true
      console.error(`[TEMPLATE] ${filename} 编译错误:`)
      for (const e of r.errors) console.error('  ' + (e.message || e))
    }
  } catch (e) {
    failed = true
    console.error(`[TEMPLATE] ${filename} 抛出异常: ${e.message}`)
  }
}

if (failed) {
  console.error(`\n❌ ${filename} 校验失败`)
  process.exit(1)
} else {
  console.log(`✅ ${filename} SFC 校验通过 (template + script 均无语法错误)`)
  process.exit(0)
}
