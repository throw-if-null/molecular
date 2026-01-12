import { tool } from "@opencode-ai/plugin";

interface ReadJsonFileResult {
  ok: boolean;
  data?: unknown;
  error?: string;
}

export default tool({
  description:
    "Read and parse a JSON file from disk, returning the parsed object",
  args: {
    path: tool.schema
      .string()
      .describe("Path to the JSON file to read, relative to the current working directory"),
  },
  async execute(args, _context) {
    const path = args.path;

    try {
      const fs = await import("fs/promises");
      const content = await fs.readFile(path, "utf8");

      try {
        const data = JSON.parse(content) as unknown;
        const result: ReadJsonFileResult = {
          ok: true,
          data,
        };
        return JSON.stringify(result);
      } catch (parseErr) {
        const result: ReadJsonFileResult = {
          ok: false,
          error: `Failed to parse JSON in ${path}: ${String(parseErr)}`,
        };
        return JSON.stringify(result);
      }
    } catch (err) {
      const result: ReadJsonFileResult = {
        ok: false,
        error: `Failed to read file ${path}: ${String(err)}`,
      };
      return JSON.stringify(result);
    }
  },
});
