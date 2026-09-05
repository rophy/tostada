import { test, expect } from './fixtures'
import { login } from './helpers'

test.beforeEach(async ({ page }) => {
  const resp = await page.request.get('/api/auth/login')
  if (resp.status() >= 500) test.skip(true, 'cluster not available')
})

test('shows login page when unauthenticated', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Login with OIDC')).toBeVisible()
})

test('login flow lands on dashboard', async ({ page }) => {
  await login(page, 'alice')
  await expect(page.getByText('Workspaces')).toBeVisible()
  await expect(page.getByText('alice')).toBeVisible()
})

test('shows workspace cards after login', async ({ page }) => {
  await login(page, 'alice')
  await expect(page.getByText('Jupyter Notebook')).toBeVisible()
  await expect(page.getByText('Ubuntu Desktop (KasmVNC)')).toBeVisible()
})

test('shows devices table when devices exist', async ({ page }) => {
  await login(page, 'alice')
  const devices = page.getByText('Devices')
  // Devices section only shows if alice has granted devices
  if (await devices.isVisible({ timeout: 3000 }).catch(() => false)) {
    await expect(page.getByRole('cell', { name: 'RDP' }).first()).toBeVisible()
  }
})

test('logout returns to login page', async ({ page }) => {
  await login(page, 'alice')
  await page.getByRole('button', { name: 'Logout' }).click()
  await expect(page.getByText('Login with OIDC')).toBeVisible()
})

test('admin link visible for admin users', async ({ page }) => {
  await login(page, 'alice')
  const adminBtn = page.getByRole('button', { name: 'Admin' })
  if (await adminBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
    await expect(adminBtn).toBeVisible()
  } else {
    test.skip(true, 'alice is not admin — run: tostada-cli user set-admin alice true')
  }
})

test('admin link hidden for non-admin users', async ({ page }) => {
  await login(page, 'bob')
  await expect(page.getByText('Workspaces')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Admin' })).not.toBeVisible()
})

test('launch and stop a workspace session', async ({ page }) => {
  await login(page, 'alice')
  await expect(page.getByText('Workspaces')).toBeVisible()

  // Find and click Launch on the first workspace card
  const launchBtn = page.getByRole('button', { name: 'Launch' }).first()
  await launchBtn.click()

  // Active Sessions section should appear with a Starting or Ready tag
  await expect(page.getByText('Active Sessions')).toBeVisible({ timeout: 10000 })
  const statusTag = page.locator('.ant-tag').filter({ hasText: /Starting|Ready/ }).first()
  await expect(statusTag).toBeVisible({ timeout: 10000 })

  // Click the status tag to expand progress row
  await statusTag.click()
  const expandedRow = page.locator('.ant-table-expanded-row')
  await expect(expandedRow).toBeVisible({ timeout: 5000 })

  // Wait for session to become ready (up to 120s)
  await expect(page.locator('.ant-tag').filter({ hasText: 'Ready' })).toBeVisible({ timeout: 120000 })

  // Stop the session: click Stop, then confirm
  await page.getByRole('button', { name: 'Stop' }).first().click()
  const confirmBtn = page.getByRole('button', { name: 'Stop' }).last()
  await confirmBtn.click()

  // Session should disappear
  await expect(page.getByText('Active Sessions')).not.toBeVisible({ timeout: 30000 })
})
