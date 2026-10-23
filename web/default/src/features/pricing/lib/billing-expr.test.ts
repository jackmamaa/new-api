import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  buildRequestRuleExpr,
  combineBillingExpr,
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
  type RequestRuleGroup,
} from './billing-expr'

const durationPath = 'metadata.durationSeconds'

describe('request rule expressions round-trip through the editor parser', () => {
  const cases: Array<{ name: string; groups: RequestRuleGroup[] }> = [
    {
      name: 'param equals',
      groups: [
        {
          conditions: [
            { source: 'param', path: durationPath, mode: 'eq', value: '8' },
          ],
          multiplier: '2',
        },
      ],
    },
    {
      name: 'param greater than',
      groups: [
        {
          conditions: [
            { source: 'param', path: durationPath, mode: 'gt', value: '4' },
          ],
          multiplier: '2',
        },
      ],
    },
    {
      name: 'param greater than or equal',
      groups: [
        {
          conditions: [
            { source: 'param', path: durationPath, mode: 'gte', value: '4' },
          ],
          multiplier: '2',
        },
      ],
    },
    {
      name: 'param less than',
      groups: [
        {
          conditions: [
            { source: 'param', path: durationPath, mode: 'lt', value: '8' },
          ],
          multiplier: '0.5',
        },
      ],
    },
    {
      name: 'param less than or equal',
      groups: [
        {
          conditions: [
            { source: 'param', path: durationPath, mode: 'lte', value: '8' },
          ],
          multiplier: '0.5',
        },
      ],
    },
    {
      name: 'param contains',
      groups: [
        {
          conditions: [
            { source: 'param', path: 'prompt', mode: 'contains', value: 'fast' },
          ],
          multiplier: '1.5',
        },
      ],
    },
    {
      name: 'existence check followed by a comparison on the same param',
      groups: [
        {
          conditions: [
            { source: 'param', path: durationPath, mode: 'exists', value: '' },
            { source: 'param', path: durationPath, mode: 'gt', value: '4' },
          ],
          multiplier: '2',
        },
      ],
    },
    {
      name: 'comparisons on two params with an equality check',
      groups: [
        {
          conditions: [
            { source: 'param', path: durationPath, mode: 'gt', value: '4' },
            { source: 'param', path: 'n', mode: 'lte', value: '2' },
            { source: 'header', path: 'x-tier', mode: 'eq', value: 'fast' },
          ],
          multiplier: '3',
        },
      ],
    },
    {
      name: 'several rule groups multiplied together',
      groups: [
        {
          conditions: [
            { source: 'param', path: durationPath, mode: 'eq', value: '8' },
          ],
          multiplier: '2',
        },
        {
          conditions: [
            { source: 'param', path: 'n', mode: 'gte', value: '2' },
          ],
          multiplier: '1.2',
        },
      ],
    },
  ]

  for (const { name, groups } of cases) {
    test(name, () => {
      assert.deepEqual(tryParseRequestRuleExpr(buildRequestRuleExpr(groups)), groups)
    })
  }
})

describe('splitBillingExprAndRequestRules', () => {
  test('separates a comparison rule from the base expression', () => {
    const base = 'tier("base", 500000)'
    const rules = buildRequestRuleExpr([
      {
        conditions: [
          { source: 'param', path: durationPath, mode: 'gt', value: '4' },
        ],
        multiplier: '2',
      },
    ])

    assert.deepEqual(
      splitBillingExprAndRequestRules(combineBillingExpr(base, rules)),
      { billingExpr: base, requestRuleExpr: rules }
    )
  })
})
