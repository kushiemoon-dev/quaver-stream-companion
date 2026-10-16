#!/usr/bin/env node
// Builds the website: site/ plus the overlay pages and themes its demo frames load, in one folder.
// Usage (repo root): node scripts/build-site.mjs [outDir]   (default .site-out)
import { cpSync, existsSync, rmSync } from 'node:fs';
import { join, resolve, sep } from 'node:path';

const root = process.cwd();
const out = resolve(root, process.argv[2] || '.site-out');
const inside = (dir, parent) => dir === parent || dir.startsWith(parent + sep);

if (!existsSync(join(root, 'site', 'index.html'))) {
  console.error('build-site: run it from the repository root');
  process.exit(2);
}
if (inside(root, out) || ['site', 'web', '.git'].some((d) => inside(out, join(root, d)))) {
  console.error('build-site: refusing to write to ' + out);
  process.exit(2);
}

rmSync(out, { recursive: true, force: true });
cpSync(join(root, 'site'), out, { recursive: true });
cpSync(join(root, 'web', 'overlays'), join(out, 'overlays'), { recursive: true });
cpSync(join(root, 'web', 'themes'), join(out, 'themes'), { recursive: true });
console.log('build-site: wrote ' + out);
