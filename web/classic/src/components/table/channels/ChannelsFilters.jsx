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
import { Button, Form, Select } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';

const MODEL_FAMILY_ORDER = [
  'OpenAI',
  'Claude',
  'Gemini',
  'Grok',
  'DeepSeek',
  'Qwen',
  'ByteDance',
  'Zhipu',
  'Kimi',
  'MiniMax',
  'Mistral',
  'Meta',
  'Cohere',
  'Baidu',
  'Hunyuan',
  'Other',
];

const ChannelsFilters = ({
  setEditingChannel,
  setShowEdit,
  refresh,
  setShowColumnSelector,
  formInitValues,
  setFormApi,
  searchChannels,
  enableTagMode,
  formApi,
  groupOptions,
  loading,
  searching,
  activeModelFamily,
  activeModelType,
  activeBillingType,
  modelFamilyCounts,
  modelTypeCounts,
  billingTypeCounts,
  handleChannelFacetChange,
  t,
}) => {
  return (
    <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
      <div className='flex gap-2 w-full md:w-auto order-2 md:order-1'>
        <Button
          size='small'
          theme='light'
          type='primary'
          className='w-full md:w-auto'
          onClick={() => {
            setEditingChannel({
              id: undefined,
            });
            setShowEdit(true);
          }}
        >
          {t('添加渠道')}
        </Button>

        <Button
          size='small'
          type='tertiary'
          className='w-full md:w-auto'
          onClick={refresh}
        >
          {t('刷新')}
        </Button>

        <Button
          size='small'
          type='tertiary'
          onClick={() => setShowColumnSelector(true)}
          className='w-full md:w-auto'
        >
          {t('列设置')}
        </Button>
      </div>

      <div className='flex flex-col md:flex-row items-center gap-2 w-full md:w-auto order-1 md:order-2'>
        <Form
          initValues={formInitValues}
          getFormApi={(api) => setFormApi(api)}
          onSubmit={() => searchChannels(enableTagMode)}
          allowEmpty={true}
          autoComplete='off'
          layout='horizontal'
          trigger='change'
          stopValidateWithError={false}
          className='flex flex-col md:flex-row items-center gap-2 w-full'
        >
          <div className='w-full md:w-36'>
            <Select
              size='small'
              value={activeModelFamily}
              onChange={(value) => handleChannelFacetChange('family', value)}
              optionList={[
                {
                  label: `${t('全部家族')} (${modelFamilyCounts.all || 0})`,
                  value: 'all',
                },
                ...MODEL_FAMILY_ORDER.filter(
                  (family) => modelFamilyCounts[family] > 0,
                ).map((family) => ({
                  label: `${family} (${modelFamilyCounts[family]})`,
                  value: family,
                })),
              ]}
              pure
            />
          </div>
          <div className='w-full md:w-32'>
            <Select
              size='small'
              value={activeModelType}
              onChange={(value) => handleChannelFacetChange('modelType', value)}
              optionList={[
                {
                  label: `${t('全部类型')} (${modelTypeCounts.all || 0})`,
                  value: 'all',
                },
                {
                  label: `${t('文本')} (${modelTypeCounts.Text || 0})`,
                  value: 'Text',
                },
                {
                  label: `${t('图片')} (${modelTypeCounts.Image || 0})`,
                  value: 'Image',
                },
                {
                  label: `${t('视频')} (${modelTypeCounts.Video || 0})`,
                  value: 'Video',
                },
              ]}
              pure
            />
          </div>
          <div className='w-full md:w-32'>
            <Select
              size='small'
              value={activeBillingType}
              onChange={(value) =>
                handleChannelFacetChange('billingType', value)
              }
              optionList={[
                {
                  label: `${t('全部计费')} (${billingTypeCounts.all || 0})`,
                  value: 'all',
                },
                {
                  label: `${t('按次')} (${billingTypeCounts.PerRequest || 0})`,
                  value: 'PerRequest',
                },
                {
                  label: `${t('按量')} (${billingTypeCounts.PerToken || 0})`,
                  value: 'PerToken',
                },
              ]}
              pure
            />
          </div>
          <div className='relative w-full md:w-64'>
            <Form.Input
              size='small'
              field='searchKeyword'
              prefix={<IconSearch />}
              placeholder={t('渠道ID，名称，密钥，API地址')}
              showClear
              pure
            />
          </div>
          <div className='w-full md:w-48'>
            <Form.Input
              size='small'
              field='searchModel'
              prefix={<IconSearch />}
              placeholder={t('模型关键字')}
              showClear
              pure
            />
          </div>
          <div className='w-full md:w-32'>
            <Form.Select
              size='small'
              field='searchGroup'
              placeholder={t('选择分组')}
              optionList={[
                { label: t('选择分组'), value: null },
                ...groupOptions,
              ]}
              className='w-full'
              showClear
              pure
              onChange={() => {
                // 延迟执行搜索，让表单值先更新
                setTimeout(() => {
                  searchChannels(enableTagMode);
                }, 0);
              }}
            />
          </div>
          <Button
            size='small'
            type='tertiary'
            htmlType='submit'
            loading={loading || searching}
            className='w-full md:w-auto'
          >
            {t('查询')}
          </Button>
          <Button
            size='small'
            type='tertiary'
            onClick={() => {
              if (formApi) {
                formApi.reset();
                // 重置后立即查询，使用setTimeout确保表单重置完成
                setTimeout(() => {
                  handleChannelFacetChange('reset', 'all');
                }, 100);
              }
            }}
            className='w-full md:w-auto'
          >
            {t('重置')}
          </Button>
        </Form>
      </div>
    </div>
  );
};

export default ChannelsFilters;
