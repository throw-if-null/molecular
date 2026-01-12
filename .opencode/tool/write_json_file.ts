import { tool } from "@opencode-ai/plugin";

interface WriteJsonFileResult {
  ok: boolean;
  error?: string;
}

export default tool({
  description:
    "Serialize a value as pretty-printed JSON and write it to disk",
  args: {
    path: tool.schema
      .string()
      .describe("Path to the JSON file to write, relative to the current working directory"),
    data: tool.schema.any().describe("JSON-serializable data to write"),
  },
  async execute(args, _context) {
    const path = args.path;
    const data = args.data;

    try {
      const fs = await import("fs/promises");
      const json = `${JSON.stringify(data, null, 2)}\n`;
      await fs.writeFile(path, json, "utf8");

      const result: WriteJsonFileResult = {
        ok: true,
      };
      return JSON.stringify(result);
    } catch (err) {
      const result: WriteJsonFileResult = {
        ok: false,
        error: `Failed to write file ${path}: ${String(err)}`,
      };
      return JSON.stringify(result);
    }
  },
});
