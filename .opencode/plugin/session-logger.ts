import type { Plugin } from "@opencode-ai/plugin";

type Shell = (
  strings: TemplateStringsArray,
  ...values: unknown[]
) => Promise<unknown>

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

function getSessionId(value: unknown): string | null {
  if (!isRecord(value)) return null

  const session = isRecord(value.session) ? value.session : null

  const candidates: unknown[] = [
    (value as Record<string, unknown>).sessionID,
    value.sessionId,
    value.session_id,
    session?.id,
    session?.sessionId,
    session?.session_id,
    (session as Record<string, unknown> | null)?.sessionID,
    session?.key,
    value.id,
  ]

  for (const candidate of candidates) {
    if (typeof candidate === "string" && candidate.length > 0) return candidate
  }

  return null
}

function truncate(value: string, max = 500) {
  if (value.length <= max) return value
  return value.slice(0, max) + "…"
}

function getArgsPreview(tool: unknown, input: unknown): Record<string, unknown> {
  if (!isRecord(input)) return {}

  const args = (input as Record<string, unknown>).args
  if (!isRecord(args)) return {}

  if (tool === "bash") {
    return {
      command: typeof args.command === "string" ? truncate(args.command) : undefined,
    }
  }

  if (tool === "read") {
    return {
      filePath: typeof args.filePath === "string" ? args.filePath : undefined,
    }
  }

  if (tool === "edit" || tool === "write") {
    return {
      filePath: typeof args.filePath === "string" ? args.filePath : undefined,
    }
  }

  if (tool === "grep") {
    return {
      pattern: typeof args.pattern === "string" ? truncate(args.pattern) : undefined,
      include: typeof args.include === "string" ? args.include : undefined,
      path: typeof args.path === "string" ? args.path : undefined,
    }
  }

  if (tool === "glob") {
    return {
      pattern: typeof args.pattern === "string" ? truncate(args.pattern) : undefined,
      path: typeof args.path === "string" ? args.path : undefined,
    }
  }

  return { ...args }
}

function safeShallow(value: unknown): unknown {
  if (typeof value === "string") return truncate(value)
  if (typeof value === "number" || typeof value === "boolean" || value === null) return value

  if (Array.isArray(value)) return value.slice(0, 20).map(safeShallow)

  if (isRecord(value)) {
    const out: Record<string, unknown> = {}
    for (const [key, v] of Object.entries(value)) {
      if (key === "args" && isRecord(v)) {
        out.args = { ...v }
        continue
      }
      out[key] = safeShallow(v)
    }
    return out
  }

  return String(value)
}

export const SessionLoggerPlugin: Plugin = async ({ directory, $ }) => {
  const logDir = `${directory}/.opencode/session-log`
  const sessionsIndexPath = `${logDir}/sessions.jsonl`
  const sh = $ as unknown as Shell

  function toSafeFilename(value: string) {
    return value.replaceAll(/[^a-zA-Z0-9_.-]/g, "_")
  }

  async function appendLine(filePath: string, line: string) {
    await sh`mkdir -p ${logDir}`

    // Try to avoid quoting issues by passing the payload via env vars.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const bash: any = sh`bash -lc 'printf "%s\\n" "$LINE" >> "$LOG_PATH"'`

    if (typeof bash?.env === "function") {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      await bash.env({ LINE: line, LOG_PATH: filePath } as any)
      return
    }

    // Fallback: best-effort quoting.
    await sh`bash -lc 'printf "%s\\n" "${line}" >> "${filePath}"'`
  }

  return {
    "tool.execute.before": async (input) => {
      const sessionId = getSessionId(input) ?? "unknown"
      const sessionFilePath = `${logDir}/${toSafeFilename(sessionId)}.jsonl`

      const payload = {
        ts: new Date().toISOString(),
        type: "tool.execute.before",
        sessionId,
        directory,
        tool: input.tool,
        argsPreview: getArgsPreview(input.tool, input),
        debug: {
          inputKeys: isRecord(input) ? Object.keys(input).sort() : [],
          input: safeShallow(input),
        },
      }

      await appendLine(sessionFilePath, JSON.stringify(payload))

      const indexLine = {
        ts: payload.ts,
        sessionId,
      }

      await appendLine(sessionsIndexPath, JSON.stringify(indexLine))
    },
  }
}
