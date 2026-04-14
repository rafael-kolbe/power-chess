const path = require("path");
const { defineConfig } = require("@playwright/test");

// Force browser cache under the repo. Some agent/CI environments pre-set PLAYWRIGHT_BROWSERS_PATH
// to an empty sandbox path; always override so `npx playwright install` matches test runs.
process.env.PLAYWRIGHT_BROWSERS_PATH = path.join(__dirname, ".pw-browsers");

module.exports = defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: false,
  retries: 0,
  timeout: 60000,
  use: {
    baseURL: "http://127.0.0.1:8080"
  },
  webServer: {
    // Run without a database so the server starts in stateless mode (no postgres required).
    // DATABASE_URL= overrides the value loaded from .env via godotenv.
    command: "DATABASE_URL= ADMIN_DEBUG_MATCH=1 go run ./cmd/server",
    url: "http://127.0.0.1:8080/healthz",
    reuseExistingServer: false,
    timeout: 120000
  }
});
