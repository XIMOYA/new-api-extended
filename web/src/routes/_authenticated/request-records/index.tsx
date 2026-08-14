// web/src/routes/_authenticated/request-records/index.tsx
// 请求记录路由：将独立审计页面挂载到已登录用户布局。

import { createFileRoute } from '@tanstack/react-router'
import z from 'zod'

import { RequestRecords } from '@/features/request-records'

const requestRecordsSearchSchema = z.object({
  page: z.number().int().min(1).optional().catch(1),
  requestId: z.string().optional().catch(''),
})

export const Route = createFileRoute('/_authenticated/request-records/')({
  validateSearch: requestRecordsSearchSchema,
  component: RequestRecords,
})
