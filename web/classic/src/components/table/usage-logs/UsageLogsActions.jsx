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
import {
  Button,
  Radio,
  RadioGroup,
  Tag,
  Space,
  Skeleton,
  Typography,
} from '@douyinfe/semi-ui';
import { BarChart3, ChevronDown, ChevronUp } from 'lucide-react';
import { renderNumber, renderQuota } from '../../../helpers';
import CompactModeToggle from '../../common/ui/CompactModeToggle';
import { useMinimumLoadingTime } from '../../../hooks/common/useMinimumLoadingTime';

const LogsActions = ({
  stat,
  loadingStat,
  showStat,
  compactMode,
  setCompactMode,
  minuteIncome,
  isAdminUser,
  modelRequestPeriod,
  setModelRequestPeriod,
  showModelRequestStats,
  setShowModelRequestStats,
  modelRequestStats,
  loadingModelRequestStats,
  t,
}) => {
  const showSkeleton = useMinimumLoadingTime(loadingStat);
  const needSkeleton = !showStat || showSkeleton;

  const placeholder = (
    <Space>
      <Skeleton.Title style={{ width: 108, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 65, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 64, height: 21, borderRadius: 6 }} />
    </Space>
  );

  const totalModelRequests = modelRequestStats.reduce(
    (total, item) => total + Number(item.request_count || 0),
    0,
  );

  return (
    <div className='flex w-full flex-col gap-3'>
      <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
        <Skeleton loading={needSkeleton} active placeholder={placeholder}>
          <Space>
            <Tag
              color='blue'
              style={{
                fontWeight: 500,
                boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                padding: 13,
              }}
              className='!rounded-lg'
            >
              {t('消耗额度')}: {renderQuota(stat.quota)}
            </Tag>
            <Tag
              color='pink'
              style={{
                fontWeight: 500,
                boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                padding: 13,
              }}
              className='!rounded-lg'
            >
              RPM: {stat.rpm}
            </Tag>
            <Tag
              color='white'
              style={{
                border: 'none',
                boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                fontWeight: 500,
                padding: 13,
              }}
              className='!rounded-lg'
            >
              TPM: {stat.tpm}
            </Tag>
          </Space>
        </Skeleton>

        <div className='flex items-center gap-2'>
          {isAdminUser && (
            <Tag
              color='green'
              title={t('当前自然分钟内非管理员用户的总消耗金额')}
              style={{ fontWeight: 600, padding: 13 }}
              className='!rounded-lg whitespace-nowrap'
            >
              MPM: {renderQuota(minuteIncome || 0)}
            </Tag>
          )}
          <CompactModeToggle
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        </div>
      </div>

      {isAdminUser && (
        <div
          className='w-full overflow-hidden rounded-lg border'
          style={{
            borderColor: 'var(--semi-color-border)',
            background: 'var(--semi-color-bg-0)',
          }}
        >
          <div className='flex min-h-10 items-center justify-between gap-3 px-3 py-2'>
            <div className='flex min-w-0 items-center gap-2'>
              <span
                className='flex h-7 w-7 shrink-0 items-center justify-center rounded-md'
                style={{
                  color: 'var(--semi-color-primary)',
                  background: 'var(--semi-color-primary-light-default)',
                }}
              >
                <BarChart3 size={15} />
              </span>
              <Typography.Text strong>{t('模型请求排行')}</Typography.Text>
              {modelRequestStats.length > 0 && (
                <Typography.Text type='tertiary' size='small'>
                  {t('总计')} {renderNumber(totalModelRequests)}
                </Typography.Text>
              )}
            </div>
            <Button
              aria-label={showModelRequestStats ? t('收起') : t('展开')}
              icon={
                showModelRequestStats ? (
                  <ChevronUp size={16} />
                ) : (
                  <ChevronDown size={16} />
                )
              }
              size='small'
              theme='borderless'
              type='tertiary'
              onClick={() => setShowModelRequestStats((visible) => !visible)}
            />
          </div>

          {showModelRequestStats && (
            <div
              className='border-t px-3 pb-3 pt-2'
              style={{ borderColor: 'var(--semi-color-border)' }}
            >
              <div className='mb-2 flex items-center justify-end'>
                <RadioGroup
                  type='button'
                  buttonSize='small'
                  value={modelRequestPeriod}
                  onChange={(event) =>
                    setModelRequestPeriod(event.target.value)
                  }
                >
                  <Radio value='total'>{t('累计')}</Radio>
                  <Radio value='today'>{t('今日')}</Radio>
                </RadioGroup>
              </div>
              <Skeleton
                loading={
                  loadingModelRequestStats && modelRequestStats.length === 0
                }
                active
                placeholder={placeholder}
              >
                <div className='grid max-h-52 grid-cols-1 gap-2 overflow-y-auto pr-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
                  {modelRequestStats.map((item, index) => (
                    <div
                      key={item.model_name}
                      className='flex min-w-0 items-center gap-2 rounded-md border px-2.5 py-2'
                      style={{
                        borderColor:
                          index < 3
                            ? 'var(--semi-color-primary-light-active)'
                            : 'var(--semi-color-border)',
                        background:
                          index < 3
                            ? 'var(--semi-color-primary-light-default)'
                            : 'var(--semi-color-fill-0)',
                      }}
                    >
                      <span
                        className='flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-xs font-semibold'
                        style={{
                          color:
                            index < 3
                              ? 'var(--semi-color-primary)'
                              : 'var(--semi-color-text-2)',
                          background: 'var(--semi-color-bg-0)',
                        }}
                      >
                        {index + 1}
                      </span>
                      <Typography.Text
                        className='min-w-0 flex-1'
                        ellipsis={{ showTooltip: true }}
                      >
                        {item.model_name}
                      </Typography.Text>
                      <Typography.Text strong className='shrink-0 tabular-nums'>
                        {renderNumber(item.request_count)}
                      </Typography.Text>
                    </div>
                  ))}
                  {!loadingModelRequestStats &&
                    modelRequestStats.length === 0 && (
                      <Typography.Text type='tertiary'>
                        {t('暂无数据')}
                      </Typography.Text>
                    )}
                </div>
              </Skeleton>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default LogsActions;
