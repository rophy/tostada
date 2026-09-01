import { test as base, Page } from '@playwright/test'
import * as fs from 'fs'
import * as path from 'path'

const coverageDir = path.join(__dirname, '.nyc_output')

async function collectCoverage(page: Page, testTitle: string) {
  const coverage = await page.evaluate(() => (window as any).__coverage__)
  if (!coverage) return
  fs.mkdirSync(coverageDir, { recursive: true })
  const filename = `${testTitle.replace(/\W+/g, '-')}-${Date.now()}.json`
  fs.writeFileSync(path.join(coverageDir, filename), JSON.stringify(coverage))
}

export const test = base.extend({
  page: async ({ page }, use, testInfo) => {
    await use(page)
    await collectCoverage(page, testInfo.title)
  },
})

export { expect } from '@playwright/test'
