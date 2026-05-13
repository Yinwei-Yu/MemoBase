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

const mockMessages = {
  data: {
    items: [
      { message_id: 'msg_1', session_id: 'sess_1', role: 'user', content: '什么是死锁？', created_at: '2026-03-18T12:00:00Z' },
      { message_id: 'msg_2', session_id: 'sess_1', role: 'assistant', content: '死锁是多个进程互相等待对方持有的资源...', created_at: '2026-03-18T12:00:01Z' },
    ],
    pagination: { page: 1, page_size: 200, total: 2 },
  },
};

test.describe('Chat Page', () => {
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
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { items: [], pagination: { page: 1, page_size: 20, total: 0 } } }),
      });
    });
    await page.goto('/chat/kb_001');
  });

  test('renders chat heading and empty state', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '智能问答' })).toBeVisible();
    await expect(page.getByPlaceholderText('输入你的问题...')).toBeVisible();
    await expect(page.getByRole('button', { name: '发送' })).toBeVisible();
  });

  test('shows document link', async ({ page }) => {
    await expect(page.getByRole('link', { name: '文档' })).toHaveAttribute('href', '/kbs/kb_001/documents');
  });

  test('disables send button when empty', async ({ page }) => {
    await expect(page.getByRole('button', { name: '发送' })).toBeDisabled();
  });

  test('enables send button with text', async ({ page }) => {
    await page.getByPlaceholderText('输入你的问题...').fill('什么是进程？');
    await expect(page.getByRole('button', { name: '发送' })).not.toBeDisabled();
  });

  test('displays messages', async ({ page }) => {
    await expect(page.getByText('什么是死锁？')).toBeVisible();
    await expect(page.getByText('死锁是多个进程互相等待对方持有的资源...')).toBeVisible();
  });

  test('shows role labels', async ({ page }) => {
    await expect(page.getByText('你')).toBeVisible();
    await expect(page.getByText('AI')).toBeVisible();
  });

  test('submits question and shows loading', async ({ page }) => {
    await page.route('**/api/v1/chat/completions', async (route) => {
      await new Promise((r) => setTimeout(r, 1000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            session_id: 'sess_1',
            answer: '回答内容',
            citations: [],
            memory_used: [],
            degraded: false,
            latency_ms: 500,
            token_usage: { prompt_tokens: 100, completion_tokens: 50, total_tokens: 150 },
          },
        }),
      });
    });

    await page.getByPlaceholderText('输入你的问题...').fill('测试问题');
    await page.getByRole('button', { name: '发送' }).click();
    await expect(page.getByRole('button', { name: '生成中...' })).toBeVisible();
  });

  test('shows error on chat failure', async ({ page }) => {
    await page.route('**/api/v1/chat/completions', (route) =>
      route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({
          error: { code: 'MODEL_UNAVAILABLE', message: 'ollama chat failed' },
        }),
      }),
    );

    await page.getByPlaceholderText('输入你的问题...').fill('test');
    await page.getByRole('button', { name: '发送' }).click();
    await expect(page.getByText(/MODEL_UNAVAILABLE/)).toBeVisible();
  });
});
