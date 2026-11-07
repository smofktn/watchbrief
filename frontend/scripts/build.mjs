import { mkdir, copyFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
const root = fileURLToPath(new URL('../', import.meta.url))
await mkdir(`${root}dist`, { recursive: true })
for (const [source, destination] of [
  ['index.html', 'index.html'], ['src/app.js', 'app.js'], ['src/client.mjs', 'client.mjs'],
  ['src/style.css', 'style.css'], ['node_modules/vue/dist/vue.runtime.esm-browser.prod.js', 'vue.js'],
]) await copyFile(`${root}${source}`, `${root}dist/${destination}`)
