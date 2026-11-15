import request from '@/request'

// fscanx-web 指纹库域 API（BL-POC-004 前端重做）
// textarea 整文编辑：内容即所写（零格式漂移），保存前 JSON 合法性校验在后端。

export interface FingerFiles {
  files: Record<string, string>
}

export interface PackResult {
  src: string
  dst: string
  raw: number
  gz: number
}

export const getFingerApi = () => {
  return request.get<FingerFiles>({ url: '/api/finger' })
}

export const putFingerApi = (file: string, content: string) => {
  return request.put<{ status: string; path: string }>({ url: '/api/finger', data: { file, content } })
}

export const packApi = () => {
  return request.post<{ status: string; results: PackResult[] }>({ url: '/api/pack' })
}
