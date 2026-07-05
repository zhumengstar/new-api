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
import { createFileRoute } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Main } from '@/components/layout'

/* oxlint-disable react/iframe-missing-sandbox */

export const Route = createFileRoute('/_authenticated/canvas/')({
  component: CanvasPage,
})

function CanvasPage() {
  const { t } = useTranslation()

  return (
    <Main className='p-0'>
      {/* The cross-origin Canvas app needs its own origin for browser storage. */}
      <iframe
        src='https://canvas.muling.store/'
        title={t('Infinite Canvas')}
        className='h-full min-h-[calc(100svh-var(--app-header-height,0px))] w-full border-0'
        sandbox='allow-downloads allow-forms allow-popups allow-same-origin allow-scripts'
        allow='clipboard-read; clipboard-write'
      />
    </Main>
  )
}
