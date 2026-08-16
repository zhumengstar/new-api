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

import type { User } from '../types'

const QQ_PATTERN = /^[1-9]\d{4,11}$/

export function isQQContact(value?: string): boolean {
  return QQ_PATTERN.test(String(value || '').trim())
}

export function parseUserContact(value?: string): {
  qqContact: string
  wechatContact: string
} {
  const contacts = String(value || '')
    .split(/[,，;；]/)
    .map((item) => item.trim())
    .filter(Boolean)

  return contacts.reduce(
    (result, contact) => {
      if (isQQContact(contact)) {
        if (!result.qqContact) result.qqContact = contact
      } else if (!result.wechatContact) {
        result.wechatContact = contact
      }
      return result
    },
    { qqContact: '', wechatContact: '' }
  )
}

export function getUserContactValue(
  user: Pick<User, 'username' | 'qq_contact' | 'wechat_contact'>
): string {
  const contacts = [user.qq_contact, user.wechat_contact].filter(Boolean)
  if (contacts.length > 0) return contacts.join(', ')
  return isQQContact(user.username) ? user.username : ''
}

export function getUserContactItems(
  user: Pick<User, 'username' | 'qq_contact' | 'wechat_contact'>
) {
  const contacts: Array<{
    type: 'QQ' | 'WeChat'
    value: string
    inferred: boolean
  }> = []
  if (user.qq_contact) {
    contacts.push({ type: 'QQ', value: user.qq_contact, inferred: false })
  }
  if (user.wechat_contact) {
    contacts.push({
      type: 'WeChat',
      value: user.wechat_contact,
      inferred: false,
    })
  }
  if (contacts.length === 0 && isQQContact(user.username)) {
    contacts.push({ type: 'QQ', value: user.username, inferred: true })
  }
  return contacts
}
