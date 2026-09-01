import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.beforeEach(async ({ page }) => {
  const resp = await page.request.get('/api/auth/login')
  if (resp.status() >= 500) test.skip(true, 'cluster not available')
})

test('non-admin user sees access denied', async ({ page }) => {
  await login(page, 'bob')
  await page.goto('/admin')
  await expect(page.getByText('Access Denied')).toBeVisible()
  await expect(page.getByText('Admin access required')).toBeVisible()
})

test('non-admin can navigate back from access denied', async ({ page }) => {
  await login(page, 'bob')
  await page.goto('/admin')
  await expect(page.getByText('Access Denied')).toBeVisible()
  await page.getByRole('button', { name: 'Back to Dashboard' }).click()
  await expect(page.getByText('Workspaces')).toBeVisible()
})

test('admin user sees admin panel', async ({ page }) => {
  await login(page, 'alice')
  const meResp = await page.request.get('/api/auth/me')
  const me = await meResp.json()
  if (!me.isAdmin) test.skip(true, 'alice is not admin — run: tostada-cli user set-admin alice true')

  await page.goto('/admin')
  await expect(page.getByText('Users')).toBeVisible()
  await expect(page.getByText('Devices')).toBeVisible()
  await expect(page.getByText('Sessions')).toBeVisible()
})

test('admin users tab lists users', async ({ page }) => {
  await login(page, 'alice')
  const meResp = await page.request.get('/api/auth/me')
  const me = await meResp.json()
  if (!me.isAdmin) test.skip(true, 'alice is not admin')

  await page.goto('/admin')
  // Users tab is default
  await expect(page.getByRole('cell', { name: 'alice' })).toBeVisible()
})

test('admin devices tab lists devices', async ({ page }) => {
  await login(page, 'alice')
  const meResp = await page.request.get('/api/auth/me')
  const me = await meResp.json()
  if (!me.isAdmin) test.skip(true, 'alice is not admin')

  await page.goto('/admin')
  await page.getByRole('tab', { name: 'Devices' }).click()
  // Should show the devices table (may be empty)
  await expect(page.getByRole('table')).toBeVisible()
})

test('admin sessions tab shows empty or sessions', async ({ page }) => {
  await login(page, 'alice')
  const meResp = await page.request.get('/api/auth/me')
  const me = await meResp.json()
  if (!me.isAdmin) test.skip(true, 'alice is not admin')

  await page.goto('/admin')
  await page.getByRole('tab', { name: 'Sessions' }).click()
  const empty = page.getByText('No active sessions')
  const table = page.getByRole('table')
  await expect(empty.or(table)).toBeVisible()
})

test('admin can navigate back to dashboard', async ({ page }) => {
  await login(page, 'alice')
  const meResp = await page.request.get('/api/auth/me')
  const me = await meResp.json()
  if (!me.isAdmin) test.skip(true, 'alice is not admin')

  await page.goto('/admin')
  await page.getByRole('link', { name: '' }).first().click()
  await expect(page.getByText('Workspaces')).toBeVisible()
})
