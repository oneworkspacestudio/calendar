import { readFile, writeFile } from "node:fs/promises";
import { parse, compileReact, CORE_REGISTRY } from "@codedia/parser";
const registry = structuredClone(CORE_REGISTRY);
for (const def of Object.values(registry)) {
  def.props = {
    ...def.props,
    role: { type: "string" },
    datum: { type: "binding" },
    field: { type: "string" },
    text: { type: "binding" },
    color: { type: "binding" },
    today: { type: "binding" },
    muted: { type: "binding" },
    selected: { type: "binding" },
    icon: { type: "string" },
    options: { type: "binding" },
  };
}
const source = await readFile(
  new URL("../src/Calendar.aui", import.meta.url),
  "utf8",
);
// Reject malformed structural branches before the compiler can flatten them.
const lines = source.split(/\r?\n/).filter((line) => line.trim());
for (let i = 0; i < lines.length; i++) {
  const indent = lines[i].length - lines[i].trimStart().length;
  if (indent % 2) throw new Error("AUI indentation must use pairs of spaces");
  const next = lines[i + 1];
  if (next) {
    const depth = next.length - next.trimStart().length;
    if (depth > indent + 2)
      throw new Error("AUI skipped a parent indentation at line " + (i + 2));
    if (/^(If |Else$|For )/.test(lines[i].trim()) && depth !== indent + 2)
      throw new Error("AUI structural branch has no child at line " + (i + 1));
  }
}
const ancestors = [];
for (const line of lines) {
  const depth = line.length - line.trimStart().length;
  while (ancestors.length && ancestors.at(-1).depth >= depth) ancestors.pop();
  const type = line.trim().split(/\s/)[0];
  if (type === "Button" && ancestors.some((a) => a.type === "Button"))
    throw new Error("AUI cannot nest interactive buttons");
  ancestors.push({ depth, type });
}
const result = compileReact(parse(source), {
  registry,
  componentName: "CalendarScreen",
});
await writeFile(
  new URL("../src/Calendar.generated.tsx", import.meta.url),
  "// Generated from Calendar.aui. Do not edit.\n" + result,
);
console.log("Compiled Calendar.aui");
