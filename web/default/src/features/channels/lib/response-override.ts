/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type ResponseOverrideRule = {
  match: string
  replace: string
  request_param: string
  request_value: string
}

const RESPONSE_OVERRIDE_MAX_RULES = 50
const RESPONSE_OVERRIDE_MAX_MATCH_LENGTH = 512
const RESPONSE_OVERRIDE_MAX_REPLACE_LENGTH = 8192
const RESPONSE_OVERRIDE_MAX_PARAM_LENGTH = 256
const RESPONSE_OVERRIDE_MAX_VALUE_LENGTH = 1024

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function readString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

// Limits are enforced in bytes by the backend; count bytes here too so a
// multibyte keyword cannot pass client validation and fail on save.
function byteLength(value: string): number {
  return new TextEncoder().encode(value).length
}

export function createResponseOverrideRule(): ResponseOverrideRule {
  return {
    match: '',
    replace: '',
    request_param: '',
    request_value: '',
  }
}

export function normalizeResponseOverrideRules(
  value: unknown
): ResponseOverrideRule[] {
  if (!Array.isArray(value)) return []
  return value.filter(isRecord).map((rule) => ({
    match: readString(rule.match),
    replace: readString(rule.replace),
    request_param: readString(rule.request_param),
    request_value: readString(rule.request_value),
  }))
}

export function parseResponseOverrideRules(
  value: string | undefined
): ResponseOverrideRule[] {
  if (!value?.trim()) return []
  try {
    return normalizeResponseOverrideRules(JSON.parse(value))
  } catch {
    return []
  }
}

export function stringifyResponseOverrideRules(
  rules: ResponseOverrideRule[]
): string {
  if (rules.length === 0) return ''
  return JSON.stringify(
    rules.map((rule) => ({
      match: rule.match,
      replace: rule.replace,
      request_param: rule.request_param,
      request_value: rule.request_value,
    })),
    null,
    2
  )
}

/**
 * Drop rows that the user left completely empty so an unused row never blocks
 * saving. Rows with a replacement or condition but no keyword are kept and
 * rejected by validation.
 */
export function normalizeResponseOverrideRulesForSave(
  value: string | undefined
): ResponseOverrideRule[] {
  return parseResponseOverrideRules(value).filter(
    (rule) =>
      Boolean(rule.match.trim()) ||
      Boolean(rule.replace) ||
      Boolean(rule.request_param.trim()) ||
      Boolean(rule.request_value)
  )
}

export function hasResponseOverrideRules(value: string | undefined): boolean {
  return normalizeResponseOverrideRulesForSave(value).length > 0
}

export function getResponseOverrideValidationError(
  value: string | undefined
): string | null {
  if (!value?.trim()) return null

  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    return 'Response override must be a valid JSON array'
  }
  if (!Array.isArray(parsed)) {
    return 'Response override must be a valid JSON array'
  }
  if (parsed.length > RESPONSE_OVERRIDE_MAX_RULES) {
    return `Response override supports at most ${RESPONSE_OVERRIDE_MAX_RULES} rules`
  }

  for (let index = 0; index < parsed.length; index += 1) {
    const rule = parsed[index]
    if (!isRecord(rule)) {
      return `Rule ${index + 1}: must be an object`
    }
    const match = readString(rule.match)
    const replace = readString(rule.replace)
    const requestParam = readString(rule.request_param)
    const requestValue = readString(rule.request_value)
    const rowHasInput = Boolean(
      match.trim() || replace || requestParam.trim() || requestValue
    )
    if (!rowHasInput) continue

    if (!match.trim()) {
      return `Rule ${index + 1}: match keyword is required`
    }
    if (byteLength(match) > RESPONSE_OVERRIDE_MAX_MATCH_LENGTH) {
      return `Rule ${index + 1}: match keyword is too long`
    }
    if (byteLength(replace) > RESPONSE_OVERRIDE_MAX_REPLACE_LENGTH) {
      return `Rule ${index + 1}: replacement is too long`
    }
    if (requestParam && !requestValue) {
      return `Rule ${index + 1}: request value is required when a request parameter is set`
    }
    if (!requestParam && requestValue) {
      return `Rule ${index + 1}: request parameter is required when a request value is set`
    }
    if (byteLength(requestParam) > RESPONSE_OVERRIDE_MAX_PARAM_LENGTH) {
      return `Rule ${index + 1}: request parameter is too long`
    }
    if (byteLength(requestValue) > RESPONSE_OVERRIDE_MAX_VALUE_LENGTH) {
      return `Rule ${index + 1}: request value is too long`
    }
  }

  return null
}
