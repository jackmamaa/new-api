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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import {
  createResponseOverrideRule,
  parseResponseOverrideRules,
  stringifyResponseOverrideRules,
  type ResponseOverrideRule,
} from '../../lib/response-override'

type ResponseOverrideEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

type ResponseOverrideRow = ResponseOverrideRule & { id: string }

export function ResponseOverrideEditor(props: ResponseOverrideEditorProps) {
  const { t } = useTranslation()
  const rowIdCounterRef = useRef(0)
  const [rows, setRows] = useState<ResponseOverrideRow[]>(() =>
    parseResponseOverrideRules(props.value).map((rule, index) => ({
      ...rule,
      id: `response-override-initial-${index}`,
    }))
  )

  const nextRowId = (): string => {
    rowIdCounterRef.current += 1
    return `response-override-${rowIdCounterRef.current}`
  }

  useEffect(() => {
    setRows((previousRows) => {
      if (props.value === stringifyResponseOverrideRules(previousRows)) {
        return previousRows
      }
      return parseResponseOverrideRules(props.value).map((rule) => ({
        ...rule,
        id: nextRowId(),
      }))
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.value])

  const commitRows = (nextRows: ResponseOverrideRow[]): void => {
    setRows(nextRows)
    props.onChange(stringifyResponseOverrideRules(nextRows))
  }

  const updateRule = (
    id: string,
    patch: Partial<ResponseOverrideRule>
  ): void => {
    commitRows(rows.map((row) => (row.id === id ? { ...row, ...patch } : row)))
  }

  const removeRule = (id: string): void => {
    commitRows(rows.filter((row) => row.id !== id))
  }

  const addRule = (): void => {
    commitRows([...rows, { ...createResponseOverrideRule(), id: nextRowId() }])
  }

  return (
    <div className='space-y-3'>
      {rows.length === 0 ? (
        <p className='text-muted-foreground text-xs'>
          {t(
            'No response override rules configured. Click "Add Rule" to get started.'
          )}
        </p>
      ) : (
        rows.map((row) => (
          <div
            key={row.id}
            className='border-border/60 space-y-2 rounded-md border p-3'
          >
            <div className='grid gap-2 sm:grid-cols-2'>
              <Input
                value={row.match}
                onChange={(event) =>
                  updateRule(row.id, { match: event.target.value })
                }
                disabled={props.disabled}
                aria-label={t('Match keyword')}
                placeholder={t('Match keyword, e.g. gpt-image-2.5-flare')}
              />
              <Input
                value={row.replace}
                onChange={(event) =>
                  updateRule(row.id, { replace: event.target.value })
                }
                disabled={props.disabled}
                aria-label={t('Replace with')}
                placeholder={t('Replace with, e.g. gpt-image-2.5-sunburst')}
              />
            </div>
            <div className='grid gap-2 sm:grid-cols-[1fr_1fr_auto]'>
              <Input
                value={row.request_param}
                onChange={(event) =>
                  updateRule(row.id, { request_param: event.target.value })
                }
                disabled={props.disabled}
                aria-label={t('Request parameter')}
                placeholder={t('Optional request parameter, e.g. quality')}
              />
              <Input
                value={row.request_value}
                onChange={(event) =>
                  updateRule(row.id, { request_value: event.target.value })
                }
                disabled={props.disabled}
                aria-label={t('Request value')}
                placeholder={t('Optional expected value, e.g. max')}
              />
              <Button
                type='button'
                variant='ghost'
                size='icon'
                onClick={() => removeRule(row.id)}
                disabled={props.disabled}
                aria-label={t('Delete rule')}
              >
                <Trash2 className='h-4 w-4' aria-hidden='true' />
              </Button>
            </div>
          </div>
        ))
      )}
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={addRule}
        disabled={props.disabled}
      >
        <Plus className='mr-2 h-4 w-4' aria-hidden='true' />
        {t('Add Rule')}
      </Button>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Both request condition fields can be left empty to always replace. When set, replacement only applies if the request parameter equals the expected value.'
        )}
      </p>
    </div>
  )
}
