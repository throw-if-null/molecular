import { tool } from "@opencode-ai/plugin";
interface ValidationError {
  path: string;
  message: string;
  code?: string;
}
interface ValidationResult {
  ok: boolean;
  errors: ValidationError[];
}
function validateBuilderResult(data: unknown): ValidationResult {
  const errors: ValidationError[] = [];

  if (data === null || typeof data !== "object" || Array.isArray(data)) {
    errors.push({
      path: "<root>",
      code: "type_error",
      message: "builder_result must be a JSON object",
    });
    return { ok: false, errors };
  }

  const obj = data as Record<string, unknown>;

  const run = obj.run as unknown;
  if (run === null || typeof run !== "object" || Array.isArray(run)) {
    errors.push({
      path: "run",
      code: "required",
      message: "run must be an object",
    });
  }

  let runStatus: unknown = undefined;
  if (run && typeof run === "object" && !Array.isArray(run)) {
    const runObj = run as Record<string, unknown>;
    runStatus = runObj.status;
    if (runStatus !== "ok" && runStatus !== "failed") {
      errors.push({
        path: "run.status",
        code: "invalid_enum",
        message: "run.status must be 'ok' or 'failed'",
      });
    }

    const failedStep = runObj.failed_step;
    if (failedStep !== null && failedStep !== undefined && typeof failedStep !== "string") {
      errors.push({
        path: "run.failed_step",
        code: "type_error",
        message: "run.failed_step must be a string or null",
      });
    }

    const error = runObj.error;
    if (error !== null && error !== undefined && typeof error !== "string") {
      errors.push({
        path: "run.error",
        code: "type_error",
        message: "run.error must be a string or null",
      });
    }
  }

  const work = obj.work as unknown;

  // When run.status is failed, work may be null.
  if (runStatus === "failed") {
    if (work !== null) {
      errors.push({
        path: "work",
        code: "invalid",
        message: "work must be null when run.status is 'failed'",
      });
    }

    return { ok: errors.length === 0, errors };
  }

  // When run.status is ok, work must be present and valid.
  if (work === null || typeof work !== "object" || Array.isArray(work)) {
    errors.push({
      path: "work",
      code: "required",
      message: "work must be an object when run.status is 'ok'",
    });
    return { ok: false, errors };
  }

  const workObj = work as Record<string, unknown>;

  const summary = workObj.summary;
  if (typeof summary !== "string" || summary.trim() === "") {
    errors.push({
      path: "work.summary",
      code: "required",
      message: "work.summary must be a non-empty string",
    });
  } else if (summary.length > 300) {
    errors.push({
      path: "work.summary",
      code: "too_long",
      message: "work.summary should be at most 300 characters",
    });
  }

  const complexity = workObj.complexity;
  const allowed = new Set(["low", "medium", "high"]);
  if (typeof complexity !== "string" || !allowed.has(complexity)) {
    errors.push({
      path: "work.complexity",
      code: "invalid_enum",
      message: "work.complexity must be one of 'low', 'medium', 'high'",
    });
  }

  return { ok: errors.length === 0, errors };
}
export default tool({
  description:
    "Validate a builder_result object already loaded in memory (no file I/O). Pass the parsed JSON as 'data'.",
  args: {
    data: tool.schema
      .any()
      .optional()
      .describe("Parsed builder_result JSON object to validate (required)."),
  },
  async execute(args, _context) {
    // Enforce that we only validate provided data
    if (!args.data) {
      const result: ValidationResult = {
        ok: false,
        errors: [
          {
            path: "<root>",
            code: "required",
            message:
              "This tool currently validates only the 'data' object; file path is not supported",
          },
        ],
      };
      return JSON.stringify(result);
    }
    try {
      const result = validateBuilderResult(args.data);
      return JSON.stringify(result);
    } catch (err) {
      const result: ValidationResult = {
        ok: false,
        errors: [
          {
            path: "<tool>",
            code: "unexpected_error",
            message: `validator threw an unexpected error: ${String(err)}`,
          },
        ],
      };
      return JSON.stringify(result);
    }
  },
});
