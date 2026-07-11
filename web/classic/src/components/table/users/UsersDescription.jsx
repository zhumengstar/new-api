/*
Copyright (C) 2025 QuantumNous

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

import React from 'react';
import { Typography } from '@douyinfe/semi-ui';
import { IconUserAdd } from '@douyinfe/semi-icons';
import CompactModeToggle from '../../common/ui/CompactModeToggle';
import { renderQuota } from '../../../helpers';

const { Text } = Typography;

const UsersDescription = ({ compactMode, setCompactMode, incomeStats, t }) => {
  return (
    <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
      <div className='flex flex-col gap-1'>
        <div className='flex items-center text-blue-500'>
          <IconUserAdd className='mr-2' />
          <Text>{t('用户管理')}</Text>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Text type='tertiary' size='small' className='whitespace-nowrap'>
            {t('近7天消耗（非管理员）')}
          </Text>
          <div className='flex max-w-full overflow-x-auto rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)]'>
            {incomeStats.map((item, index) => (
              <div
                key={item.date}
                className={`flex min-w-[76px] flex-col px-2 py-1 ${index > 0 ? 'border-l border-[var(--semi-color-border)]' : ''}`}
              >
                <Text type='tertiary' size='small'>
                  {item.date.slice(5)}
                </Text>
                <Text size='small' className='whitespace-nowrap font-medium'>
                  {renderQuota(item.quota || 0)}
                </Text>
              </div>
            ))}
          </div>
        </div>
      </div>
      <CompactModeToggle
        compactMode={compactMode}
        setCompactMode={setCompactMode}
        t={t}
      />
    </div>
  );
};

export default UsersDescription;
