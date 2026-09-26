// taskSubject names what a task works on: a single target, a target list, or
// nothing when the task covers everything.
export function taskSubject(params?: Record<string, unknown>): string {
  if (!params) {
    return ''
  }
  if (Array.isArray(params.targets) && params.targets.length > 0) {
    return params.targets.join(', ')
  }
  if (typeof params.target === 'string') {
    return params.target
  }
  return ''
}

// The Tasks page marks what it has shown, so the console's global notice can
// report failures that happened while the reader was somewhere else.
const seenKey = 'eget-web-tasks-seen'

export function markTasksSeen(at = Date.now()) {
  try {
    localStorage.setItem(seenKey, String(at))
  } catch {
    // Private mode: the notice then reports every recent failure, which is fine.
  }
}

export function tasksSeenAt(): number {
  try {
    return Number(localStorage.getItem(seenKey) ?? 0) || 0
  } catch {
    return 0
  }
}
