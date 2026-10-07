import { createApp } from './src/app.js';
import { config } from './src/config/index.js';

const app = createApp();

app.listen(config.port, '0.0.0.0', () => {
  console.log(
    JSON.stringify({
      msg: 'Platform BFF started',
      port: config.port,
      env: config.env,
      billing_service: config.billingServiceUrl,
      trust_proxy: config.trustProxy,
    })
  );
});