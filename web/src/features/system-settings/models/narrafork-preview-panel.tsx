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
/*
web/src/features/system-settings/models/narrafork-preview-panel.tsx
组件：NarraFork 额度事件实时预览与测试
职责：
- 防抖请求后端预览接口，展示当前策略渲染结果
- 通过一次性 SSE 测试接口验证 quotaBalanceEvent 输出
*/
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'

import { previewNarraForkEvent, sendNarraForkTestEvent } from '../api'
import type { NarraForkPolicyPatch, NarraForkPreview } from '../types'

type Props = {
  config: NarraForkPolicyPatch
  disabled?: boolean
}

export function NarraForkPreviewPanel({ config, disabled = false }: Props) {
  const { t } = useTranslation()
  const [preview, setPreview] = useState<NarraForkPreview | null>(null)
  const [isPreviewing, setIsPreviewing] = useState(false)
  const [isTesting, setIsTesting] = useState(false)
  const [testResult, setTestResult] = useState('')

  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(async () => {
      setIsPreviewing(true)
      try {
        const response = await previewNarraForkEvent(config)
        if (!cancelled && response.success && response.data) {
          setPreview(response.data)
        }
      } catch {
        // 预览是辅助功能，输入未完成时不打扰表单操作。
      } finally {
        if (!cancelled) setIsPreviewing(false)
      }
    }, 350)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [config])

  const sendTestEvent = async () => {
    setIsTesting(true)
    setTestResult('')
    try {
      const response = await sendNarraForkTestEvent(config)
      setTestResult(response)
      toast.success(t('NarraFork test event received'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to send test event')
      )
    } finally {
      setIsTesting(false)
    }
  }

  return (
    <div className='bg-muted/20 space-y-3 rounded-md border p-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div>
          <div className='font-medium'>{t('NarraFork live preview')}</div>
          <div className='text-muted-foreground text-xs'>
            {t('Preview uses safe sample data and never consumes real quota.')}
          </div>
        </div>
        <Button
          type='button'
          variant='outline'
          onClick={() => void sendTestEvent()}
          disabled={disabled || isTesting}
        >
          {isTesting ? t('Sending...') : t('Send test event')}
        </Button>
      </div>

      {isPreviewing && (
        <div className='text-muted-foreground text-xs'>
          {t('Updating preview...')}
        </div>
      )}
      {preview && (
        <div className='space-y-2'>
          <div className='text-xs font-medium'>
            {t('Event')}: {preview.eventName}
          </div>
          <pre className='bg-background max-h-64 overflow-auto rounded border p-3 text-xs whitespace-pre-wrap'>
            {JSON.stringify(preview.payload, null, 2)}
          </pre>
          <details>
            <summary className='text-muted-foreground cursor-pointer text-xs'>
              {t('Show raw SSE')}
            </summary>
            <pre className='bg-background mt-2 max-h-40 overflow-auto rounded border p-3 text-xs whitespace-pre-wrap'>
              {preview.sse}
            </pre>
          </details>
        </div>
      )}
      {testResult && (
        <div>
          <div className='mb-1 text-xs font-medium'>
            {t('Test SSE response')}
          </div>
          <pre className='bg-background max-h-40 overflow-auto rounded border p-3 text-xs whitespace-pre-wrap'>
            {testResult}
          </pre>
        </div>
      )}
    </div>
  )
}
