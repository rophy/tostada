import { Page, expect } from '@playwright/test'

// Mapping from username (sub) to display name shown on the OIDC mock login page
const displayNames: Record<string, string> = {
  alice: 'Alice',
  bob: 'Bob',
}

export async function login(page: Page, username: string) {
  await page.goto('/api/auth/login')
  const displayName = displayNames[username] || username
  await page.getByRole('button', { name: displayName }).click()
  await page.waitForURL('/')
  await expect(page.locator('body')).toContainText(username)
}
