import base from '/home/omnigent/workspace/b9s/web/playwright.config';
export default {
  ...base,
  testDir: '/tmp/bd-iiem-current-browser-tests',
  testMatch: /header\.spec\.ts/,
  globalSetup: undefined,
  outputDir: '/tmp/bd-iiem-current-browser-r2-output',
  reporter: [['list'], ['json', { outputFile: '/tmp/bd-iiem-current-browser-r2-results.json' }]],
  projects: base.projects!.filter(p => ['phone-360', 'tablet-768', 'desktop-1440', 'wide-2560'].includes(p.name!)).map(p => ({ ...p, testMatch: /header\.spec\.ts/ })),
};
