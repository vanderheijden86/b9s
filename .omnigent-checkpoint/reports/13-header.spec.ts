import { test, expect, open, row } from '/home/omnigent/workspace/b9s/web/tests/harness';
for (const id of ['bd-iiem', 'bd-iiem.2.3', 'long-project-name-iiem.2.3']) {
  test.describe(id, () => {
    test.use({ fixture: { issues: [{ id, title: 'Inspect the detail header', description: 'Header visibility probe.' }] } });
    test('full ID and title are visible in the detail header', async ({ page, project }, info) => {
      const errors: string[] = [];
      page.on('pageerror', e => errors.push(e.message));
      await open(page, project);
      const identity = await (await page.request.get(project.url + 'api/session')).json();
      expect(identity.commit).toBe(process.env.B9S_EXPECTED_SHA);
      expect(identity.modified).toBe(false);
      await info.attach('served-identity', { body: JSON.stringify({ commit: identity.commit, modified: identity.modified }), contentType: 'application/json' });
      await row(page, id).click();
      const detail = page.locator('.detail');
      await expect(detail.locator('.dtop .id')).toHaveText(id);
      await expect(detail.locator('.dtop .id')).toBeInViewport();
      await expect(detail.locator('.dtitle')).toHaveText('Inspect the detail header');
      await expect(detail.locator('.dtitle')).toBeInViewport();
      const facts = await detail.locator('.dtop .id').evaluate(el => ({
        text: el.textContent,
        scrollWidth: el.scrollWidth,
        clientWidth: el.clientWidth,
      }));
      expect(facts.scrollWidth).toBeLessThanOrEqual(facts.clientWidth);
      await page.screenshot({ path: info.outputPath('visible-header.png') });
      await info.attach('header-observation', { body: JSON.stringify(facts), contentType: 'application/json' });
      expect(errors).toEqual([]);
    });
  });
}
