import { spawn, spawnSync, ChildProcess } from 'node:child_process';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';

const REPO_ROOT = path.resolve(__dirname, '..');
const RUNTIME_FILE = path.join(__dirname, '.runtime');
const FIXTURE_DIR = path.join(os.tmpdir(), 'snapdiff-screenshots-fixture');
const BIN_PATH = path.join(__dirname, '.runtime-bin', 'snapdiff');

/**
 * globalSetup:
 *   1. Builds a deterministic fixture git repo at FIXTURE_DIR.
 *   2. Builds the snapdiff binary at BIN_PATH.
 *   3. Spawns it via `serve --repo FIXTURE_DIR`.
 *   4. Parses the bound URL from stderr.
 *   5. Sets process.env.SNAPDIFF_URL so the test suite picks it up.
 *   6. Writes the pid + url to .runtime so globalTeardown can kill it.
 */
export default async function globalSetup() {
  // Clean previous runs.
  if (fs.existsSync(FIXTURE_DIR)) fs.rmSync(FIXTURE_DIR, { recursive: true, force: true });
  if (fs.existsSync(RUNTIME_FILE)) fs.unlinkSync(RUNTIME_FILE);

  // 1. Build fixture.
  const fix = spawnSync('go', ['run', './tests-screenshots/fixtures/build_fixture', '--out', FIXTURE_DIR], {
    cwd: REPO_ROOT,
    encoding: 'utf-8',
  });
  if (fix.status !== 0) {
    throw new Error(`fixture build failed: ${fix.stderr}\n${fix.stdout}`);
  }

  // 2. Build the snapdiff binary.
  fs.mkdirSync(path.dirname(BIN_PATH), { recursive: true });
  const build = spawnSync('go', ['build', '-o', BIN_PATH, './cmd/snapdiff'], {
    cwd: REPO_ROOT,
    encoding: 'utf-8',
  });
  if (build.status !== 0) {
    throw new Error(`snapdiff build failed: ${build.stderr}`);
  }

  // 3. Spawn snapdiff serve.
  const proc: ChildProcess = spawn(BIN_PATH, ['serve', '--repo', FIXTURE_DIR], {
    cwd: FIXTURE_DIR,
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: false,
  });

  // 4. Parse `review at http://...` from stderr.
  const url = await new Promise<string>((resolve, reject) => {
    let stderrBuf = '';
    const timeout = setTimeout(() => reject(new Error(`timeout waiting for URL from snapdiff. stderr=${stderrBuf}`)), 10_000);

    proc.stderr?.on('data', (chunk: Buffer) => {
      stderrBuf += chunk.toString();
      const m = stderrBuf.match(/review at (https?:\/\/\S+)/);
      if (m) {
        clearTimeout(timeout);
        resolve(m[1].trim());
      }
    });

    proc.on('error', reject);
    proc.on('exit', (code, sig) =>
      reject(new Error(`snapdiff exited early (code=${code} signal=${sig}). stderr=${stderrBuf}`)),
    );
  });

  // 5. Hand off via env.
  process.env.SNAPDIFF_URL = url;

  // 6. Persist runtime info for teardown + downstream readers.
  fs.writeFileSync(RUNTIME_FILE, JSON.stringify({ pid: proc.pid, url, fixture: FIXTURE_DIR }, null, 2));

  // Probe /healthz to be sure it's actually serving.
  for (let i = 0; i < 20; i++) {
    try {
      const r = await fetch(`${url}/healthz`);
      if (r.ok) break;
    } catch {
      // not ready yet
    }
    await new Promise(r => setTimeout(r, 100));
  }

  // Detach so it survives a parent exit during dev re-runs; teardown
  // will kill it explicitly.
  proc.unref();
  proc.stderr?.unref();
  proc.stdout?.unref();
}
