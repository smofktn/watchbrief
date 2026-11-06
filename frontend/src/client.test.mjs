import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createClient } from './client.mjs'

test('login sends same-origin JSON and CSRF header, then fetches identity', async () => {
  const calls = []
  const client = createClient(async (url, options) => {
    calls.push({ url, options })
    return { ok:true, json: async () => url === '/api/me' ? {login_id:'fictional'} : {result:'ok'} }
  })
  await client.login('Fictional', 'fictional test password')
  assert.equal((await client.me()).login_id, 'fictional')
  assert.equal(calls[0].options.headers['X-WatchBrief-CSRF'], '1')
  assert.equal(calls[0].options.credentials, 'same-origin')
  assert.equal(calls[1].options.cache, 'no-store')
  assert.equal(JSON.parse(calls[0].options.body).login_id, 'Fictional')
  assert.equal(calls.length, 2)
})

test('failure shows fixed messages without exposing response internals or retrying', async () => {
  let calls = 0
  const client = createClient(async () => { calls++; return {ok:false,status:500,json:async()=>({error:'NEVER_PRINT_THIS'})} })
  await assert.rejects(client.login('fictional','fictional test password'), error => error.status === 500 && !error.message.includes('NEVER_PRINT'))
  assert.equal(calls, 1)
})

test('network error is actionable and sanitized', async () => {
  const client = createClient(async () => { throw new Error('NEVER_PRINT_THIS') })
  await assert.rejects(client.me(), error => error.message.includes('接続') && !error.message.includes('NEVER_PRINT'))
})
