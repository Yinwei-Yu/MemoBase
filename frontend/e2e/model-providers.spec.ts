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

const mockProviders = {
  data: [
    {
      provider_id: 'prov_001',
      user_id: 'u_demo',
      name: 'DeepSeek',
      provider_type: 'openai_compatible',
      api_base_url: 'https://api.deepseek.com',
      api_key_masked: 'sk-****abc',
      default_model: 'deepseek-chat',
      embedding_model: '',
      is_default: true,
      created_at: '2026-03-18T12:00:00Z',
      updated_at: '2026-03-18T12:00:00Z',
    },
    {
      provider_id: 'prov_002',
      user_id: 'u_demo',
      name: 'OpenAI',
      provider_type: 'openai_compatible',
      api_base_url: 'https://api.openai.com',
      api_key_masked: 'sk-****xyz',
      default_model: 'gpt-4o',
      embedding_model: 'text-embedding-3-small',
      is_default: false,
      created_at: '2026-03-17T10:00:00Z',
      updated_at: '2026-03-17T10:00:00Z',
    },
  ],
  request_id: 'req_test',
  timestamp: '2026-03-18T12:00:00Z',
};

const emptyProviders = {
  data: [],
  request_id: 'req_test',
  timestamp: '2026-03-18T12:00:00Z',
};

function fulfillJson(route: import('@playwright/test').Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
}

/** Register default route mocks BEFORE navigation to prevent hitting real backend. */
async function mockAllProviders(page: import('@playwright/test').Page, providers = mockProviders) {
  await page.route('**/api/v1/model-providers', (route) => {
    if (route.request().method() === 'GET') return fulfillJson(route, 200, providers);
    return fulfillJson(route, 200, { data: {} });
  });
}

test.describe('Model Providers Page', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page);
    await mockAllProviders(page);
    await page.goto('/settings/providers');
  });

  test('renders page heading and add button', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '模型提供商' })).toBeVisible();
    await expect(page.getByRole('button', { name: '添加提供商' })).toBeVisible();
  });

  test('shows form fields when clicking add', async ({ page }) => {
    await page.getByRole('button', { name: '添加提供商' }).click();
    await expect(page.getByRole('heading', { name: '添加提供商' })).toBeVisible();
    await expect(page.getByLabel('名称')).toBeVisible();
    await expect(page.getByLabel('API Base URL')).toBeVisible();
    await expect(page.getByLabel('API Key')).toBeVisible();
    await expect(page.getByLabel('默认模型')).toBeVisible();
    await expect(page.locator('label').filter({ hasText: 'Embedding' }).locator('input')).toBeVisible();
  });

  test('displays provider list', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'DeepSeek' })).toBeVisible();
    await expect(page.getByText('https://api.deepseek.com')).toBeVisible();
    await expect(page.getByText('deepseek-chat')).toBeVisible();
    await expect(page.getByText('sk-****abc')).toBeVisible();
    await expect(page.getByText('默认', { exact: true })).toBeVisible();
  });

  test('shows embedding model in provider list', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'OpenAI' })).toBeVisible();
    await expect(page.locator('.card').nth(1).getByText(/Embedding/)).toBeVisible();
  });

  test('creates a new provider', async ({ page }) => {
    // Override POST handler
    await page.route('**/api/v1/model-providers', (route) => {
      if (route.request().method() === 'POST') {
        return fulfillJson(route, 201, {
          data: {
            provider_id: 'prov_new',
            name: '新提供商',
            api_base_url: 'https://api.new.com',
            api_key_masked: 'sk-****new',
            default_model: 'new-model',
            embedding_model: '',
            is_default: false,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        });
      }
      // GET will be caught by beforeEach handler (LIFO: this handler runs first for POST, beforeEach for GET)
      return fulfillJson(route, 200, mockProviders);
    });

    await page.getByRole('button', { name: '添加提供商' }).click();
    await page.getByLabel('名称').fill('新提供商');
    await page.getByLabel('API Base URL').fill('https://api.new.com');
    await page.getByLabel('API Key').fill('sk-test-key');
    await page.getByLabel('默认模型').fill('new-model');
    await page.getByRole('button', { name: '保存' }).click();

    // Form should be hidden after success
    await expect(page.getByRole('heading', { name: '添加提供商' })).not.toBeVisible();
  });

  test('creates provider with embedding model', async ({ page }) => {
    let postedBody: unknown = null;
    await page.route('**/api/v1/model-providers', (route) => {
      if (route.request().method() === 'POST') {
        postedBody = route.request().postDataJSON();
        return fulfillJson(route, 201, {
          data: {
            provider_id: 'prov_emb',
            name: 'Embed Provider',
            api_base_url: 'https://api.emb.com',
            api_key_masked: 'sk-****emb',
            default_model: 'chat-model',
            embedding_model: 'embed-model-v1',
            is_default: false,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        });
      }
      return fulfillJson(route, 200, mockProviders);
    });

    await page.getByRole('button', { name: '添加提供商' }).click();
    await page.getByLabel('名称').fill('Embed Provider');
    await page.getByLabel('API Base URL').fill('https://api.emb.com');
    await page.getByLabel('API Key').fill('sk-emb-key');
    await page.getByLabel('默认模型').fill('chat-model');
    await page.locator('label').filter({ hasText: 'Embedding' }).locator('input').fill('embed-model-v1');
    await page.getByRole('button', { name: '保存' }).click();

    await expect(page.getByRole('heading', { name: '添加提供商' })).not.toBeVisible();
    expect(postedBody).toBeTruthy();
    expect((postedBody as Record<string, string>).embedding_model).toBe('embed-model-v1');
  });

  test('shows error on create failure', async ({ page }) => {
    await page.route('**/api/v1/model-providers', (route) => {
      if (route.request().method() === 'POST') {
        return fulfillJson(route, 422, {
          error: { code: 'VALIDATION_ERROR', message: 'name already exists' },
        });
      }
      return fulfillJson(route, 200, mockProviders);
    });

    await page.getByRole('button', { name: '添加提供商' }).click();
    await page.getByLabel('名称').fill('dup');
    await page.getByLabel('API Base URL').fill('https://api.dup.com');
    await page.getByLabel('API Key').fill('sk-dup');
    await page.getByLabel('默认模型').fill('model');
    await page.getByRole('button', { name: '保存' }).click();

    await expect(page.getByText('VALIDATION_ERROR: name already exists')).toBeVisible();
  });

  test('edits a provider', async ({ page }) => {
    await page.route('**/api/v1/model-providers/prov_001', (route) => {
      if (route.request().method() === 'PATCH') {
        return fulfillJson(route, 200, {
          data: {
            provider_id: 'prov_001',
            name: 'DeepSeek Updated',
            api_base_url: 'https://api.deepseek.com',
            api_key_masked: 'sk-****abc',
            default_model: 'deepseek-chat',
            embedding_model: '',
            is_default: true,
            created_at: '2026-03-18T12:00:00Z',
            updated_at: new Date().toISOString(),
          },
        });
      }
      return fulfillJson(route, 200, { data: {} });
    });

    const firstCard = page.locator('.card').first();
    await firstCard.getByRole('button', { name: '编辑' }).click();
    await expect(page.getByRole('heading', { name: '编辑提供商' })).toBeVisible();
    await expect(page.getByLabel('名称')).toHaveValue('DeepSeek');
    await page.getByLabel('名称').fill('DeepSeek Updated');
    await page.getByRole('button', { name: '保存' }).click();
  });

  test('deletes a provider', async ({ page }) => {
    await page.route('**/api/v1/model-providers/prov_002', (route) => {
      if (route.request().method() === 'DELETE') {
        return fulfillJson(route, 200, { data: { deleted: true } });
      }
      return fulfillJson(route, 200, { data: {} });
    });

    page.on('dialog', (dialog) => dialog.accept());

    const secondCard = page.locator('.card').nth(1);
    await secondCard.getByRole('button', { name: '删除' }).click();
  });

  test('sets provider as default', async ({ page }) => {
    await page.route('**/api/v1/model-providers/prov_002', (route) => {
      if (route.request().method() === 'PATCH') {
        return fulfillJson(route, 200, {
          data: {
            provider_id: 'prov_002',
            name: 'OpenAI',
            api_base_url: 'https://api.openai.com',
            api_key_masked: 'sk-****xyz',
            default_model: 'gpt-4o',
            embedding_model: 'text-embedding-3-small',
            is_default: true,
            created_at: '2026-03-17T10:00:00Z',
            updated_at: new Date().toISOString(),
          },
        });
      }
      return fulfillJson(route, 200, { data: {} });
    });

    const secondCard = page.locator('.card').nth(1);
    await secondCard.getByRole('button', { name: '设为默认' }).click();
  });

  test('tests provider connection', async ({ page }) => {
    await page.route('**/api/v1/model-providers/prov_001/test', (route) => {
      if (route.request().method() === 'POST') {
        return fulfillJson(route, 200, {
          data: { success: true, answer: 'pong', latency_ms: 120, model: 'deepseek-chat' },
        });
      }
      return fulfillJson(route, 200, { data: {} });
    });

    const firstCard = page.locator('.card').first();
    await firstCard.getByRole('button', { name: '测试' }).click();
    await expect(page.getByText('连接成功')).toBeVisible();
    await expect(page.getByText('120ms')).toBeVisible();
  });

  test('shows test failure result', async ({ page }) => {
    await page.route('**/api/v1/model-providers/prov_001/test', (route) => {
      if (route.request().method() === 'POST') {
        return fulfillJson(route, 200, {
          data: { success: false, error: 'API key invalid', latency_ms: 50, model: '' },
        });
      }
      return fulfillJson(route, 200, { data: {} });
    });

    const firstCard = page.locator('.card').first();
    await firstCard.getByRole('button', { name: '测试' }).click();
    await expect(page.getByText('连接失败')).toBeVisible();
    await expect(page.getByText('API key invalid')).toBeVisible();
  });
});

test.describe('Model Providers - Empty State', () => {
  test('shows empty state when no providers', async ({ page }) => {
    await setupAuth(page);
    await mockAllProviders(page, emptyProviders);
    await page.goto('/settings/providers');
    await expect(page.getByText('尚未配置任何模型提供商')).toBeVisible();
  });
});
