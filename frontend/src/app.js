import { createApp, h, ref, onMounted } from './vue.js'
import { createClient } from './client.mjs'

const client = createClient()
createApp({
  setup() {
    const loginID = ref(''), password = ref(''), user = ref(null)
    const busy = ref(true), message = ref(''), authenticated = ref(false)
    async function loadMe() {
      user.value = await client.me()
      authenticated.value = true
    }
    onMounted(async () => {
      try { await loadMe() } catch (error) { if (error.status !== 401) message.value = error.message }
      finally { busy.value = false }
    })
    async function submit(event) {
      event.preventDefault()
      if (busy.value) return
      busy.value = true
      message.value = ''
      try {
        await client.login(loginID.value, password.value)
        authenticated.value = true
        await loadMe()
      } catch (error) { message.value = error.message }
      finally { password.value = ''; busy.value = false }
    }
    async function retry() {
      busy.value = true
      message.value = ''
      try { await loadMe() }
      catch (error) {
        if (error.status === 401) { authenticated.value = false; user.value = null }
        message.value = error.message
      } finally { busy.value = false }
    }
    const field = (label, properties) => h('label', [label, h('input', properties)])
    return () => h('main', [
      h('h1', 'WatchBrief'),
      busy.value ? h('p', { role: 'status' }, '確認中…') : null,
      message.value ? h('p', { role: 'alert' }, message.value) : null,
      user.value
        ? h('section', [h('h2', 'ログイン中'), h('p', { 'data-testid': 'login-id' }, user.value.login_id)])
        : authenticated.value
          ? h('button', { onClick: retry, disabled: busy.value }, '本人情報を再取得')
          : h('form', { onSubmit: submit }, [
              h('h2', 'ログイン'),
              field('ログインID', { name:'username', autocomplete:'username', required:true, maxlength:64,
                pattern:'[a-zA-Z0-9_-]{1,64}', value:loginID.value,
                onInput:event => { loginID.value = event.target.value } }),
              field('パスワード', { name:'password', type:'password', autocomplete:'current-password', required:true,
                value:password.value, onInput:event => { password.value = event.target.value } }),
              h('p', 'ログインID：英数字・_・-の1〜64文字。パスワード：15〜128文字。'),
              h('button', { type:'submit', disabled:busy.value }, 'ログイン'),
            ]),
    ])
  },
}).mount('#app')
