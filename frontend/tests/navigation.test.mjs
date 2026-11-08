import assert from 'node:assert/strict'
import { test } from 'vitest'
import { canonicalLoginPath, isProtectedPath, loginDestination, loginPathFor, safeReturnPath } from '../src/navigation.ts'

test('only implemented protected paths can be a login destination', () => {
  assert.equal(isProtectedPath('/me'), true)
  assert.equal(safeReturnPath('/me'), '/me')
  for (const path of [null, '', '/', '/login', '/api/me', '/reports/unknown', '/missing']) {
    assert.equal(safeReturnPath(path), '/me')
  }
})

test('external, normalized, encoded, credential-like and malformed destinations fall back', () => {
  const invalid = [
    'https://example.com/me', 'http://example.com', '//example.com', '///example.com',
    '\\example.com', '/\\example.com', '/me/../login', '/./me', '/me/',
    '%2f%2fexample.com', '%2fme', '/%6de', '/me?password=secret', '/me#secret',
    '/me\n', '\t/me', 'javascript:alert(1)', 'data:text/html,example', '/ME',
  ]
  for (const destination of invalid) {
    const search = `?${new URLSearchParams({ return_to: destination })}`
    assert.equal(loginDestination(search), '/me')
    assert.equal(canonicalLoginPath(search), '/login')
  }
})

test('query parsing accepts one known path, canonicalizes it and removes unused values', () => {
  assert.equal(loginPathFor('/me'), '/login?return_to=%2Fme')
  assert.equal(loginDestination('?return_to=%2Fme'), '/me')
  assert.equal(canonicalLoginPath('?return_to=%2Fme&password=not-retained'), '/login?return_to=%2Fme')
  assert.equal(canonicalLoginPath('?return_to=%2Fme&return_to=%2Fme'), '/login')
  assert.equal(canonicalLoginPath('?return_to=%E0%A4%A'), '/login')
  assert.equal(canonicalLoginPath('?return_to=%252Fme'), '/login')
  assert.equal(canonicalLoginPath('?return_to=%2Fme%3Fnext%3Dhttps%3A%2F%2Fexample.com'), '/login')
  assert.equal(canonicalLoginPath(''), '/login')
})

 test('Report routes and return destinations require canonical ULIDs', () => {
  for (const path of ['/reports', '/reports/01ARZ3NDEKTSV4RRFFQ69G5FAV']) {
    assert.equal(isProtectedPath(path), true)
    assert.equal(loginDestination(`?${new URLSearchParams({ return_to: path })}`), path)
  }
  for (const path of ['/reports/invalid', '/reports/81ARZ3NDEKTSV4RRFFQ69G5FAV', '/reports/01arz3ndektsv4rrffq69g5fav', '/reports/', '/reports?x=1', '/reports/01ARZ3NDEKTSV4RRFFQ69G5FAV#x']) {
    assert.equal(safeReturnPath(path), '/me')
  }
})
