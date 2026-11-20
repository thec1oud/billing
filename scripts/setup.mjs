import { spawnSync } from 'node:child_process';
import { copyFileSync, existsSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const isWindows = process.platform === 'win32';

function run(command, args, { cwd = root, optional = false } = {}) {
  const executable = isWindows && command === 'npm'
    ? process.env.ComSpec || 'cmd.exe'
    : command;
  const executableArgs = isWindows && command === 'npm'
    ? ['/d', '/c', `npm ${args.join(' ')}`]
    : args;
  const result = spawnSync(executable, executableArgs, {
    cwd,
    stdio: 'inherit',
  });

  if (result.error || result.status !== 0) {
    if (optional) return false;
    const reason = result.error?.message || `exited with status ${result.status}`;
    throw new Error(`Command failed: ${command} ${args.join(' ')} (${reason})`);
  }
  return true;
}

function prepareEnvironment() {
  const envPath = resolve(root, '.env');
  if (!existsSync(envPath)) {
    copyFileSync(resolve(root, '.env.example'), envPath);
    console.log('Created .env from .env.example with local-development defaults.');
  } else {
    console.log('Using existing .env; it was not modified.');
  }

  const env = readFileSync(envPath, 'utf8');
  const appEnv = env.match(/^APP_ENV\s*=\s*(.*)\s*$/m)?.[1]?.trim();
  if (appEnv?.toLowerCase() === 'production') {
    throw new Error('make setup is for local development; .env currently sets APP_ENV=production.');
  }
  const webHostPort = env.match(/^WEB_HOST_PORT\s*=\s*(.*)\s*$/m)?.[1]?.trim() || '8081';
  return webHostPort;
}

function checkPrerequisites() {
  console.log('Checking required tools...');
  run('npm', ['--version']);
  run('go', ['version']);
  run('docker', ['compose', 'version']);
  run('docker', ['info', '--format', '{{.ServerVersion}}']);
}

function installDependencies() {
  console.log('Installing BFF dependencies...');
  run('npm', ['ci'], { cwd: resolve(root, 'platform', 'bff') });

  console.log('Installing web dependencies...');
  run('npm', ['ci'], { cwd: resolve(root, 'platform', 'web') });

  console.log('Downloading Go modules...');
  run('go', ['mod', 'download']);
}

function startServices() {
  console.log('Building and starting the local development stack...');
  run('docker', ['compose', 'up', '--build', '-d']);
}

try {
  checkPrerequisites();
  const webHostPort = prepareEnvironment();
  installDependencies();
  startServices();
  console.log(`\nSetup complete. Open http://localhost:${webHostPort}.`);
} catch (error) {
  console.error(`\nSetup failed: ${error.message}`);
  console.error('Install the missing tool or fix the reported configuration, then run make setup again.');
  process.exitCode = 1;
}
