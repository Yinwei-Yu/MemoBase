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

const mockDocs = {
  data: {
    items: [
      { doc_id: 'doc_1', kb_id: 'kb_001', title: 'OS笔记', file_name: 'os-notes.md', status: 'indexed', created_at: '', updated_at: '' },
      { doc_id: 'doc_2', kb_id: 'kb_001', title: '实验报告', file_name: 'lab.txt', status: 'pending', created_at: '', updated_at: '' },
    ],
    pagination: { page: 1, page_size: 100, total: 2 },
  },
};

test.describe('Documents Page', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page);
    await page.route('**/api/v1/**', (route) => {
      const url = route.request().url();
      if (url.includes('/documents') && route.request().method() === 'GET') {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockDocs),
        });
      }
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: {} }),
      });
    });
    await page.goto('/kbs/kb_001/documents');
  });

  test('renders heading and upload form', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '文档与索引' })).toBeVisible();
    await expect(page.getByText('文档上传与索引')).toBeVisible();
    await expect(page.getByText('上传并建立索引')).toBeVisible();
  });

  test('shows file type hint', async ({ page }) => {
    await expect(page.getByText('当前仅支持 .txt / .md 文本文件解析。')).toBeVisible();
  });

  test('displays document list', async ({ page }) => {
    await expect(page.getByText('OS笔记')).toBeVisible();
    await expect(page.getByText('os-notes.md')).toBeVisible();
    await expect(page.getByText('实验报告')).toBeVisible();
  });

  test('shows status for each doc', async ({ page }) => {
    await expect(page.getByText('status: indexed')).toBeVisible();
    await expect(page.getByText('status: pending')).toBeVisible();
  });

  test('shows action buttons', async ({ page }) => {
    await expect(page.getByRole('button', { name: '查看原文' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: '重建索引' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: '删除' }).first()).toBeVisible();
  });

  test('shows chat page link', async ({ page }) => {
    await expect(page.getByRole('link', { name: '进入问答页面' })).toHaveAttribute('href', '/chat/kb_001');
  });

  test('delete triggers API call', async ({ page }) => {
    let deleteCalled = false;
    await page.route('**/api/v1/knowledge-bases/kb_001/documents/doc_1', (route) => {
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

  test('reindex triggers API call', async ({ page }) => {
    let reindexCalled = false;
    await page.route('**/api/v1/knowledge-bases/kb_001/documents/doc_1/reindex', (route) => {
      reindexCalled = true;
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { task_id: 'task_new' } }),
      });
    });

    await page.locator('.list-item').first().getByRole('button', { name: '重建索引' }).click();
    expect(reindexCalled).toBe(true);
  });
});

test.describe('Documents Page - Empty State', () => {
  test('shows empty state when no documents', async ({ page }) => {
    await setupAuth(page);
    await page.route('**/api/v1/**', (route) => {
      const url = route.request().url();
      if (url.includes('/documents') && route.request().method() === 'GET') {
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
    await page.goto('/kbs/kb_001/documents');
    await expect(page.getByText('暂无文档')).toBeVisible();
  });
});
