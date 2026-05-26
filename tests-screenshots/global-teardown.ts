import * as fs from 'node:fs';
import * as path from 'node:path';

const RUNTIME_FILE = path.join(__dirname, '.runtime');

export default async function globalTeardown() {
  if (!fs.existsSync(RUNTIME_FILE)) return;
  try {
    const { pid } = JSON.parse(fs.readFileSync(RUNTIME_FILE, 'utf-8'));
    if (typeof pid === 'number') {
      try { process.kill(pid, 'SIGTERM'); } catch { /* already gone */ }
    }
  } finally {
    fs.unlinkSync(RUNTIME_FILE);
  }
}
