const fs = require("node:fs");
const path = require("node:path");

const backendDir = path.resolve(__dirname, "..");
const envFile = path.join(backendDir, ".env");

if (fs.existsSync(envFile) && typeof process.loadEnvFile === "function") {
  process.loadEnvFile(envFile);
}

const appEnv = Object.fromEntries(
  [
    "DATABASE_URL",
    "JWT_ISSUER",
    "JWT_PRIVATE_KEY",
    "JWT_LIFETIME_MINUTES",
    "CORS_ALLOWED_ORIGINS",
    "SUPER_ADMIN_USERNAME",
    "SUPER_ADMIN_PASSWORD",
    "WEB_DIR",
  ].flatMap((key) =>
    process.env[key] === undefined ? [] : [[key, process.env[key]]],
  ),
);
appEnv.WEB_DIR = path.join(backendDir, "build", "bfsm-web");
const productionEnv = {
  ...appEnv,
  APP_ENV: "production",
  API_ADDRESS: ":9562",
};

module.exports = {
  apps: [
    {
      name: "benefit-scheme-api",
      version: "1.0.0",
      script: path.join(backendDir, "build", "bfsm-api"),
      cwd: backendDir,
      instances: 1,
      exec_mode: "fork",
      autorestart: true,
      watch: false,
      env: productionEnv,
      env_production: productionEnv,
    },
  ],
};
