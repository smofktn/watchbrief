import { expect, test } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import PrimeVue from 'primevue/config'
import ErrorNotice from '../src/components/ErrorNotice.vue'
import { provideSession, useSession } from '../src/features/auth/session'
import { createAppRouter } from '../src/router'

test('ErrorNotice renders its message as text and emits retry on user action', async () => {
  const app = mount(ErrorNotice, { props: { message: '<script>private</script>' }, global: { plugins: [[PrimeVue, { unstyled: true }]] } })
  expect(app.get('[role="alert"]').text()).toBe('<script>private</script>')
  expect(app.find('script').exists()).toBe(false)
  await app.get('button').trigger('click')
  expect(app.emitted('retry')).toHaveLength(1)
  app.unmount()
})

test('session consumers share one provider instance; unmount clears form memory', async () => {
  const router = createAppRouter(createMemoryHistory())
  await router.push('/missing')
  const Consumer = defineComponent({
    setup: () => {
      const session = useSession()
      return () => h('input', { value: session.loginId.value, onInput: (event: Event) => { session.loginId.value = (event.target as HTMLInputElement).value } })
    },
  })
  let session: ReturnType<typeof provideSession> | undefined
  const Root = defineComponent({ setup: () => {
    session = provideSession(router)
    return () => h('div', [h(Consumer), h(Consumer)])
  } })
  const app = mount(Root, { global: { plugins: [router] } })
  await flushPromises()
  await app.findAll('input')[0]!.setValue('fictional_user')
  await nextTick()
  expect(app.findAll('input')[1]!.element.value).toBe('fictional_user')
  session!.password.value = 'fictional-password'
  app.unmount()
  expect(session!.password.value).toBe('')
  expect(session!.loginId.value).toBe('')
})
