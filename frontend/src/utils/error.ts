// Maps backend error strings to user-facing Chinese copy.
export function friendlyError(e: unknown): string {
  const s = String(e).toLowerCase()
  if (s.includes('权限') || s.includes('403') || s.includes('permission')) {
    return '权限不足，请检查 Token 是否包含 Tunnel:Edit / Zone:Read / DNS:Edit 权限'
  }
  if (s.includes('未认证') || s.includes('401') || s.includes('token') || s.includes('unauthorized')) {
    return 'Token 已失效或无效，请重新配置'
  }
  if (s.includes('网络') || s.includes('timeout') || s.includes('connection') || s.includes('dial')) {
    return '无法连接 Cloudflare API，请检查网络'
  }
  if (s.includes('already exists') || s.includes('409') || s.includes('conflict')) {
    return '同名 Tunnel 已存在，请换个名字'
  }
  if (s.includes('invalid name') || s.includes('400')) {
    return '名称格式不正确，只允许字母、数字、- 和 _'
  }
  if (s.includes('last ingress rule') || s.includes('match all urls')) {
    return '最后一条规则必须匹配所有 URL（通常是兜底 404 规则）'
  }
  return String(e)
}

// Whether the error requires re-authentication (show "reconfigure Token" button).
export function needsReauth(e: unknown): boolean {
  const s = String(e).toLowerCase()
  return s.includes('未认证') || s.includes('401') || s.includes('unauthorized')
}
