import { test, expect } from '@playwright/test';

// Helper to mock auth state
async function setupAuth(page: import('@playwright/test').Page) {
  await page.addInitScript(() => {
    localStorage.setItem('memo_token', 'test-token-123');
    localStorage.setItem(
      'memo_user',
      JSON.stringify({ user_id: 'u_demo', username: 'demo', display_name: 'Demo User' }),
    );
  });
}

const mockKBList = {
  data: {
    items: [
      {
        kb_id: 'kb_001',
        user_id: 'u_demo',
        name: '操作系统复习',
        description: '课程资料与历年题',
        tags: ['OS', 'exam'],
        doc_count: 3,
        created_at: '2026-03-18T12:00:00Z',
        updated_at: '2026-03-18T12:00:00Z',
      },
      {
        kb_id: 'kb_002',
        user_id: 'u_demo',
        name: '数据库原理',
        description: '',
        tags: ['DB'],
        doc_count: 0,
        created_at: '2026-03-17T10:00:00Z',
        updated_at: '2026-03-17T10:00:00Z',
      },
    ],
    pagination: { page: 1, page_size: 50, total: 2 },
  },
  request_id: 'req_test',
  timestamp: '2026-03-18T12:00:00Z',
};

test.describe('Knowledge Base Page', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page);
    await page.route('**/api/v1/knowledge-bases**', (route) => {
      if (route.request().method() === 'GET') {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockKBList),
        });
      }
      return route.continue();
    });
    await page.goto('/kbs');
  });

  test('renders page heading and create form', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '知识库中心' })).toBeVisible();
    await expect(page.getByRole('heading', { name: '创建知识库' })).toBeVisible();
    await expect(page.getByLabel('名称')).toBeVisible();
    await expect(page.getByLabel('描述')).toBeVisible();
    await expect(page.getByLabel('标签（逗号分隔）')).toBeVisible();
    await expect(page.getByRole('button', { name: '创建知识库' })).toBeVisible();
  });

  test('displays knowledge base list', async ({ page }) => {
    await expect(page.getByText('操作系统复习')).toBeVisible();
    await expect(page.getByText('课程资料与历年题')).toBeVisible();
    await expect(page.getByText('3 个文档')).toBeVisible();
    await expect(page.getByText('数据库原理')).toBeVisible();
    await expect(page.getByText('0 个文档')).toBeVisible();
  });

  test('shows action buttons for each KB', async ({ page }) => {
    const firstItem = page.locator('.list-item').first();
    await expect(firstItem.getByRole('link', { name: '文档' })).toBeVisible();
    await expect(firstItem.getByRole('link', { name: '问答' })).toBeVisible();
    await expect(firstItem.getByRole('button', { name: '删除' })).toBeVisible();
  });

  test('creates a new knowledge base', async ({ page }) => {
    await page.route('**/api/v1/knowledge-bases', (route) => {
      if (route.request().method() === 'POST') {
        return route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              kb_id: 'kb_new',
              name: '新知识库',
              description: 'test',
              tags: ['new'],
              doc_count: 0,
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          }),
        });
      }
      return route.continue();
    });

    await page.getByLabel('名称').fill('新知识库');
    await page.getByLabel('描述').fill('test');
    await page.getByLabel('标签（逗号分隔）').fill('new');
    await page.getByRole('button', { name: '创建知识库' }).click();

    // Form should be cleared after success
    await expect(page.getByLabel('名称')).toHaveValue('');
  });

  test('shows error on create failure', async ({ page }) => {
    await page.route('**/api/v1/knowledge-bases', (route) => {
      if (route.request().method() === 'POST') {
        return route.fulfill({
          status: 422,
          contentType: 'application/json',
          body: JSON.stringify({
            error: { code: 'VALIDATION_ERROR', message: 'name is required' },
          }),
        });
      }
      return route.continue();
    });

    await page.getByLabel('名称').fill('test');
    await page.getByRole('button', { name: '创建知识库' }).click();
    await expect(page.getByText('VALIDATION_ERROR: name is required')).toBeVisible();
  });

  test('search input filters KB list', async ({ page }) => {
    const searchInput = page.getByLabel('按名称搜索知识库');
    await expect(searchInput).toBeVisible();
    await searchInput.fill('操作系统');
  });

  test('navigates to documents page', async ({ page }) => {
    await page.route('**/api/v1/knowledge-bases/kb_001/documents**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: { items: [], pagination: { page: 1, page_size: 20, total: 0 } },
        }),
      }),
    );

    await page.locator('.list-item').first().getByRole('link', { name: '文档' }).click();
    await expect(page).toHaveURL(/\/kbs\/kb_001\/documents/);
  });

  test('deletes a knowledge base', async ({ page }) => {
    await page.route('**/api/v1/knowledge-bases/kb_002', (route) => {
      if (route.request().method() === 'DELETE') {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data: { deleted: true } }),
        });
      }
      return route.continue();
    });

    const secondItem = page.locator('.list-item').nth(1);
    await secondItem.getByRole('button', { name: '删除' }).click();
  });
});

test.describe('Knowledge Base - Empty State', () => {
  test('shows empty state when no KBs', async ({ page }) => {
    await setupAuth(page);
    await page.route('**/api/v1/knowledge-bases**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: { items: [], pagination: { page: 1, page_size: 50, total: 0 } },
        }),
      }),
    );
    await page.goto('/kbs');
    await expect(page.getByText('暂无知识库')).toBeVisible();
  });
});
