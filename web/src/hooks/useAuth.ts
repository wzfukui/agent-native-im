import { useState, useCallback } from 'react'
import { login as apiLogin, setToken, getToken } from '../api'
import type { User } from '../types'

export function useAuth() {
  const [token, _setToken] = useState<string | null>(getToken())
  const [user, setUser] = useState<User | null>(null)

  const login = useCallback(async (username: string, password: string) => {
    const res = await apiLogin(username, password)
    setToken(res.token)
    _setToken(res.token)
    setUser(res.user)
  }, [])

  const logout = useCallback(() => {
    setToken(null)
    _setToken(null)
    setUser(null)
  }, [])

  return { token, user, login, logout }
}
