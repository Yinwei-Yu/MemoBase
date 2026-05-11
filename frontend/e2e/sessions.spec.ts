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

const mockSessions = {
  data: {
    items: [
      { session_id: 'sess_1', kb_id: 'kb_001', title: '操作系统复习', created_at: '', updated_at: '' },
      { session_id: 'sess_2', kb_id: 'kb_002', title: '数据库问答', created_at: '', updated_at: '' },
    ],
    pagination: { page: 1, page_size: 100, total: 2 },
  },
};

const mockMessages = {
  data: {
    items: [
      { message_id: 'msg_1', session_id: 'sess_1', role: 'user', content: '什么是死锁？', created_at: '' },
      { message_id: 'msg_2', session_id: 'sess_1', role: 'assistant', content: '死锁是多个进程互相等待...', created_at: '' },
    ],
    pagination: { page: 1, page_size: 200, total: 2 },
  },
};

test.describe('Sessions Page', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page);
    await page.route('**/api/v1/**', (route) => {
      const url = route.request().url();
      if (url.includes('/messages')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockMessages),
        });
      }
      if (url.includes('/sessions') && route.request().method() === 'GET') {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockSessions),
        });
      }
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: {} }),
      });
    });
    await page.goto('/sessions');
  });

  test('renders heading', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '会话管理' })).toBeVisible();
  });

  test('displays session list', async ({ page }) => {
    await expect(page.getByText('操作系统复习')).toBeVisible();
    await expect(page.getByText('数据库问答')).toBeVisible();
  });

  test('shows kb_id for each session', async ({ page }) => {
    await expect(page.getByText('kb: kb_001')).toBeVisible();
    await expect(page.getByText('kb: kb_002')).toBeVisible();
  });

  test('shows action buttons', async ({ page }) => {
    await expect(page.getByRole('button', { name: '查看消息' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: '删除' }).first()).toBeVisible();
  });

  test('shows placeholder when no session selected', async ({ page }) => {
    await expect(page.getByText('选择一个会话查看消息')).toBeVisible();
  });

  test('loads messages when session clicked', async ({ page }) => {
    await page.locator('.list-item').first().getByRole('button', { name: '查看消息' }).click();
    await expect(page.getByText('什么是死锁？')).toBeVisible();
    await expect(page.getByText('死锁是多个进程互相等待...')).toBeVisible();
  });

  test('shows role labels in messages', async ({ page }) => {
    await page.locator('.list-item').first().getByRole('button', { name: '查看消息' }).click();
    await expect(page.getByText('你：')).toBeVisible();
    await expect(page.getByText('助手：')).toBeVisible();
  });

  test('delete triggers API call', async ({ page }) => {
    let deleteCalled = false;
    await page.route('**/api/v1/sessions/sess_1', (route) => {
      if (route.request().method() === 'DELETE') {
        deleteCalled = true;
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data: { deleted: true } }),
        });
      }
      return route.continue();
    });

    await page.locator('.list-item').first().getByRole('button', { name: '删除' }).click();
    expect(deleteCalled).toBe(true);
  });
});

test.describe('Sessions Page - Empty State', () => {
  test('shows empty state when no sessions', async ({ page }) => {
    await setupAuth(page);
    await page.route('**/api/v1/**', (route) => {
      const url = route.request().url();
      if (url.includes('/sessions') && route.request().method() === 'GET') {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: { items: [], pagination: { page: 1, page_size: 100, total: 0 } },
          }),
        });
      }
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: {} }),
      });
    });
    await page.goto('/sessions');
    await expect(page.getByText('暂无会话')).toBeVisible();
  });
});
