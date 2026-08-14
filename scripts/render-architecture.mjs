#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { readdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const arch = join(root, "architecture");
const puppeteerConfig = join(arch, "puppeteer.json");

const files = readdirSync(arch).filter((f) => f.endsWith(".mmd"));
for (const file of files) {
  const input = join(arch, file);
  const output = join(arch, file.replace(/\.mmd$/, ".png"));
  console.log(`Rendering ${file} -> ${output}`);
  const result = spawnSync(
    "npx",
    [
      "--yes",
      "@mermaid-js/mermaid-cli",
      "-i",
      input,
      "-o",
      output,
      "-b",
      "transparent",
      "-p",
      puppeteerConfig,
    ],
    { stdio: "inherit", cwd: root }
  );
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}
