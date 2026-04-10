const { defineConfig } = require("@playwright/test");

module.exports = defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: false,
  retries: 0,
  use: {
    baseURL: "http://127.0.0.1:8080"
  },
  webServer: {
    command: "go run ./cmd/server",
    url: "http://127.0.0.1:8080/healthz",
    reuseExistingServer: false,
    timeout: 120000
  }
});
