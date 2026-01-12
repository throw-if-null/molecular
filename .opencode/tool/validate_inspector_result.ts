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

function validateInspectorResult(data: unknown): ValidationResult {
  const errors: ValidationError[] = [];

  if (data === null || typeof data !== "object" || Array.isArray(data)) {
    errors.push({
      path: "<root>",
      code: "type_error",
      message: "inspector_result must be a JSON object",
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

  const status = workObj.status;
  if (status !== "approved" && status !== "changes_requested") {
    errors.push({
      path: "work.status",
      code: "invalid_enum",
      message: "work.status must be 'approved' or 'changes_requested'",
    });
  }

  let issues = workObj.issues as unknown;
  if (!Array.isArray(issues)) {
    errors.push({
      path: "work.issues",
      code: "required",
      message: "work.issues must be an array",
    });
    issues = [];
  }

  if (status === "changes_requested" && Array.isArray(issues) && issues.length === 0) {
    errors.push({
      path: "work.issues",
      code: "empty_for_changes_requested",
      message: "work.issues must be non-empty when work.status is 'changes_requested'",
    });
  }

  if (Array.isArray(issues)) {
    issues.forEach((issue, idx) => {
      const pathPrefix = `work.issues[${idx}]`;
      if (issue === null || typeof issue !== "object" || Array.isArray(issue)) {
        errors.push({
          path: pathPrefix,
          code: "type_error",
          message: "each issue must be an object",
        });
        return;
      }

      const issueObj = issue as Record<string, unknown>;

      const severity = issueObj.severity;
      if (severity !== "blocker" && severity !== "major" && severity !== "minor") {
        errors.push({
          path: `${pathPrefix}.severity`,
          code: "invalid_enum",
          message: "severity must be one of 'blocker', 'major', 'minor'",
        });
      }

      const description = issueObj.description;
      if (typeof description !== "string" || description.trim() === "") {
        errors.push({
          path: `${pathPrefix}.description`,
          code: "required",
          message: "description must be a non-empty string",
        });
      }

      const paths = issueObj.paths as unknown;
      if (!Array.isArray(paths) || paths.length === 0) {
        errors.push({
          path: `${pathPrefix}.paths`,
          code: "required",
          message: "paths must be a non-empty array of strings",
        });
      } else {
        paths.forEach((p, pIdx) => {
          if (typeof p !== "string" || p.trim() === "") {
            errors.push({
              path: `${pathPrefix}.paths[${pIdx}]`,
              code: "type_error",
              message: "each path must be a non-empty string",
            });
          }
        });
      }
    });
  }

  let nextTasks = workObj.next_tasks as unknown;
  if (!Array.isArray(nextTasks)) {
    errors.push({
      path: "work.next_tasks",
      code: "type_error",
      message: "work.next_tasks must be an array of strings",
    });
    nextTasks = [];
  }

  if (Array.isArray(nextTasks)) {
    nextTasks.forEach((task, idx) => {
      if (typeof task !== "string" || task.trim() === "") {
        errors.push({
          path: `work.next_tasks[${idx}]`,
          code: "type_error",
          message: "each work.next_tasks entry must be a non-empty string",
        });
      }
    });
  }

  return { ok: errors.length === 0, errors };
}

export default tool({
  description:
    "Validate an inspector_result object already loaded in memory (no file I/O). Pass the parsed JSON as 'data'.",
  args: {
    data: tool.schema
      .any()
      .optional()
      .describe("Parsed inspector_result JSON object to validate (required)."),
  },
  async execute(args, _context) {
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
      const result = validateInspectorResult(args.data);
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
