import { cp, symlink, writeFile } from "node:fs/promises";
import { resolve, join } from "node:path";

// MyGo reads mygo.json from the project root. Stage a separate project so
// debug metadata reaches both the executable and Info.plist without ever
// rewriting the release configuration or the live source tree.
export async function stageDebugProject(source, staging) {
  const root = resolve(source);
  const config = await Bun.file(join(root, "mygo.json")).json();
  config.name += "-debug";
  config.identifier += ".debug";
  config.out = join(root, "build/debug");
  if (config.macos?.infoPlist?.CFBundleDisplayName) config.macos.infoPlist.CFBundleDisplayName += " 调试版";
  delete config.updates;
  // go:embed rejects symlinked files and directories. Copy the small resource
  // tree (including the staged helpers) while Go sources remain live links.
  await cp(join(root, "resources"), join(staging, "resources"), { recursive: true });
  for (const name of ["main.go", "go.mod", "go.sum", "internal", "firmware", "licenses", "docs"]) {
    await symlink(join(root, name), join(staging, name));
  }
  await writeFile(join(staging, "mygo.json"), JSON.stringify(config, null, 2) + "\n");
}

if (import.meta.main) {
  if (process.argv.length !== 4) throw new Error("usage: stage-debug-project.mjs <source> <staging>");
  await stageDebugProject(process.argv[2], process.argv[3]);
}
