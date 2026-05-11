import { test, expect } from '@playwright/test';

test.describe('Login Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
  });

  test('renders login form with default values', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '登录控制台' })).toBeVisible();
    await expect(page.getByText('MVP 默认账户: demo / demo123')).toBeVisible();

    const usernameInput = page.getByLabel('用户名');
    const passwordInput = page.getByLabel('密码');
    await expect(usernameInput).toHaveValue('demo');
    await expect(passwordInput).toHaveValue('demo123');
    await expect(page.getByRole('button', { name: '登录' })).toBeVisible();
  });

  test('shows error on invalid credentials', async ({ page }) => {
    await page.route('**/api/v1/auth/login', (route) =>
      route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({
          error: { code: 'UNAUTHORIZED', message: 'invalid credentials' },
        }),
      }),
    );

    await page.getByLabel('用户名').fill('wrong');
    await page.getByLabel('密码').fill('wrong');
    await page.getByRole('button', { name: '登录' }).click();

    await expect(page.getByText('UNAUTHORIZED: invalid credentials')).toBeVisible();
  });

  test('successful login redirects to /kbs', async ({ page }) => {
    await page.route('**/api/v1/auth/login', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'test-token-123',
            refresh_token: '',
            expires_in: 7200,
            user: {
              user_id: 'u_demo',
              username: 'demo',
              display_name: 'Demo User',
            },
          },
          request_id: 'req_test',
          timestamp: new Date().toISOString(),
        }),
      }),
    );

    await page.getByRole('button', { name: '登录' }).click();
    await page.waitForURL('/kbs');
    await expect(page).toHaveURL('/kbs');
  });

  test('shows loading state during login', async ({ page }) => {
    await page.route('**/api/v1/auth/login', async (route) => {
      await new Promise((r) => setTimeout(r, 500));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            access_token: 'test-token',
            user: { user_id: 'u_demo', username: 'demo', display_name: 'Demo' },
          },
        }),
      });
    });

    await page.getByRole('button', { name: '登录' }).click();
    await expect(page.getByRole('button', { name: '登录中...' })).toBeVisible();
  });
});

test.describe('Protected Routes', () => {
  test('redirects to /login when not authenticated', async ({ page }) => {
    await page.goto('/kbs');
    await expect(page).toHaveURL('/login');
  });

  test('redirects root to /login when not authenticated', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL('/login');
  });
});
