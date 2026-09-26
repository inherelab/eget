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
