import { test, expect } from '@playwright/test';

async function setupAuth(page: import('@playwright/test').Page) {
  await page.addInitScript(() => {
    localStorage.setItem('memo_token', 'test-token-123');
    localStorage.setItem(
      'memo_user',
      JSON.stringify({ user_id: 'u_demo', username: 'demo', display_name: 'Demo User' }),
    );
  });
}

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page);
    await page.route('**/api/v1/**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: { items: [], pagination: { page: 1, page_size: 20, total: 0 } },
        }),
      }),
    );
  });

  test('top nav shows brand and user info', async ({ page }) => {
    await page.goto('/kbs');
    await expect(page.locator('.brand')).toContainText('KnowledgeAI');
    await expect(page.locator('.user-chip')).toContainText('Demo User');
    await expect(page.getByRole('button', { name: '退出' })).toBeVisible();
  });

  test('top nav has correct links', async ({ page }) => {
    await page.goto('/kbs');
    await expect(page.getByRole('link', { name: '知识库' })).toBeVisible();
    await expect(page.getByRole('link', { name: '会话' })).toBeVisible();
    await expect(page.getByRole('link', { name: '运维状态' })).toBeVisible();
  });

  test('side nav has workspace links', async ({ page }) => {
    await page.goto('/kbs');
    await expect(page.getByText('Workspace')).toBeVisible();
    await expect(page.getByText('KnowledgeAI Console')).toBeVisible();
    await expect(page.locator('.side-link').first()).toBeVisible();
  });

  test('logout clears auth and redirects to login', async ({ page }) => {
    await page.goto('/kbs');
    await page.getByRole('button', { name: '退出' }).click();
    await expect(page).toHaveURL('/login');
  });

  test('side nav toggle collapses sidebar', async ({ page }) => {
    await page.goto('/kbs');
    const toggle = page.getByLabel('收起侧边栏');
    await expect(toggle).toBeVisible();
    await toggle.click();
    await expect(page.getByLabel('展开侧边栏')).toBeVisible();
  });

  test('navigating between pages updates active link', async ({ page }) => {
    await page.goto('/kbs');
    await page.getByRole('link', { name: '会话' }).click();
    await expect(page).toHaveURL('/sessions');
  });
});

test.describe('App Root Redirect', () => {
  test('root redirects to /kbs when authenticated', async ({ page }) => {
    await setupAuth(page);
    await page.route('**/api/v1/**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: { items: [], pagination: { page: 1, page_size: 20, total: 0 } },
        }),
      }),
    );
    await page.goto('/');
    await expect(page).toHaveURL('/kbs');
  });
});
