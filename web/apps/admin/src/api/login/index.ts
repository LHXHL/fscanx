import request from '@/request'
import type { LoginParams, LoginResult } from './types'

export const loginApi = (data: LoginParams) => {
  return request.post<LoginResult>({ url: '/api/user/login', data })
}

export const logoutApi = () => {
  return request.get({ url: '/api/user/logout' })
}

export const getRouteListApi = () => {
  return request.get<AppCustomRouteRecordRaw[]>({ url: '/api/routes' })
}
