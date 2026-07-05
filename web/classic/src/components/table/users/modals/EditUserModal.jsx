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

import React, { useEffect, useState, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showSuccess,
  renderQuota,
  getCurrencyConfig,
  isRoot,
} from '../../../../helpers';
import {
  quotaToDisplayAmount,
  displayAmountToQuota,
} from '../../../../helpers/quota';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  Button,
  Modal,
  SideSheet,
  Space,
  Spin,
  Typography,
  Card,
  Tag,
  Form,
  Avatar,
  Row,
  Col,
  InputNumber,
  RadioGroup,
  Radio,
  Checkbox,
} from '@douyinfe/semi-ui';
import {
  IconUser,
  IconSave,
  IconClose,
  IconLink,
  IconUserGroup,
  IconEdit,
} from '@douyinfe/semi-icons';
import UserBindingManagementModal from './UserBindingManagementModal';

const { Text, Title } = Typography;

const parseUserGroups = (group) => {
  if (Array.isArray(group)) {
    return group.map((item) => String(item).trim()).filter(Boolean);
  }
  return String(group || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
};

const joinUserGroups = (groups) => {
  return Array.from(new Set(parseUserGroups(groups))).join(',');
};

const toggleUserGroup = (currentGroups, group, checked) => {
  const current = parseUserGroups(currentGroups);
  if (checked) {
    return Array.from(new Set([...current, group]));
  }
  return current.filter((item) => item !== group);
};

const formatRatio = (ratio) => {
  const value = Number(ratio);
  if (!Number.isFinite(value)) return '';
  return `${Number.parseFloat(value.toFixed(6))}x`;
};

const getPublicGroups = (groupOptions) =>
  groupOptions
    .filter((option) => option.isPublic)
    .map((option) => option.value);

const collectUserGroupRatios = (
  selectedGroups,
  groupOptions,
  userGroupRatios,
) =>
  Array.from(
    new Set([...selectedGroups, ...getPublicGroups(groupOptions)]),
  ).reduce((ratios, group) => {
    const ratio = Number(userGroupRatios[group]);
    if (Number.isFinite(ratio) && ratio >= 0) {
      ratios[group] = ratio;
    }
    return ratios;
  }, {});

const parseUserGroupRatios = (setting) => {
  let parsed = setting;
  if (typeof setting === 'string' && setting.trim()) {
    try {
      parsed = JSON.parse(setting);
    } catch {
      parsed = {};
    }
  }
  const source = parsed?.user_group_ratios || {};
  return Object.entries(source).reduce((ratios, [group, ratio]) => {
    const value = Number(ratio);
    if (group && Number.isFinite(value) && value >= 0) {
      ratios[group] = value;
    }
    return ratios;
  }, {});
};

const EditUserModal = (props) => {
  const { t } = useTranslation();
  const userId = props.editingUser.id;
  const [loading, setLoading] = useState(true);
  const [adjustModalOpen, setAdjustModalOpen] = useState(false);
  const [adjustQuotaLocal, setAdjustQuotaLocal] = useState('');
  const [adjustAmountLocal, setAdjustAmountLocal] = useState('');
  const [adjustMode, setAdjustMode] = useState('add');
  const [adjustLoading, setAdjustLoading] = useState(false);
  const isMobile = useIsMobile();
  const [groupOptions, setGroupOptions] = useState([]);
  const [bindingModalVisible, setBindingModalVisible] = useState(false);
  const formApiRef = useRef(null);
  const [showAdjustQuotaRaw, setShowAdjustQuotaRaw] = useState(false);
  const [showQuotaInput, setShowQuotaInput] = useState(false);
  const [inputs, setInputs] = useState(null);
  const [selectedGroups, setSelectedGroups] = useState(['default']);
  const [userGroupRatios, setUserGroupRatios] = useState({});
  const isRootUser = isRoot();
  const quotaLabel = isRootUser ? t('额度') : t('虚拟额度');
  const adjustQuotaLabel = isRootUser ? t('调整额度') : t('调整虚拟额度');

  const isEdit = Boolean(userId);

  const getInitValues = () => ({
    username: '',
    display_name: '',
    password: '',
    github_id: '',
    oidc_id: '',
    discord_id: '',
    wechat_id: '',
    telegram_id: '',
    linux_do_id: '',
    email: '',
    quota: 0,
    quota_amount: 0,
    group: ['default'],
    remark: '',
  });

  const fetchGroups = async () => {
    try {
      let res = await API.get(`/api/group/detail`);
      const payload = res.data.data;
      const groups = Array.isArray(payload) ? payload : payload?.groups || [];
      const meta = Array.isArray(payload) ? {} : payload?.meta || {};
      setGroupOptions(
        groups.map((g) => ({
          label: g,
          value: g,
          ratio: meta[g]?.ratio,
          adminRatio: meta[g]?.admin_ratio,
          isPublic: Boolean(meta[g]?.is_public),
        })),
      );
    } catch (e) {
      showError(e.message);
    }
  };

  const handleCancel = () => props.handleClose();

  const loadUser = async () => {
    setLoading(true);
    const url = userId ? `/api/user/${userId}` : `/api/user/self`;
    const res = await API.get(url);
    const { success, message, data } = res.data;
    if (success) {
      data.password = '';
      data.quota_amount = Number(
        quotaToDisplayAmount(data.quota || 0).toFixed(6),
      );
      const groups = parseUserGroups(data.group || 'default');
      data.group = groups;
      setSelectedGroups(groups);
      setUserGroupRatios(parseUserGroupRatios(data.setting));
      setInputs({ ...getInitValues(), ...data });
    } else {
      showError(message);
    }
    setLoading(false);
  };

  useEffect(() => {
    if (inputs && formApiRef.current) {
      formApiRef.current.setValues(inputs);
      setSelectedGroups(parseUserGroups(inputs.group));
    }
  }, [inputs]);

  useEffect(() => {
    loadUser();
    if (userId) fetchGroups();
    setBindingModalVisible(false);
  }, [props.editingUser.id]);

  const openBindingModal = () => {
    setBindingModalVisible(true);
  };

  const closeBindingModal = () => {
    setBindingModalVisible(false);
  };

  /* ----------------------- submit ----------------------- */
  const submit = async (values) => {
    setLoading(true);
    let payload = { ...values };
    delete payload.quota;
    delete payload.quota_amount;
    const publicGroups = new Set(getPublicGroups(groupOptions));
    const privateGroups = selectedGroups.filter(
      (group) => !publicGroups.has(group),
    );
    payload.group = joinUserGroups(privateGroups);
    payload.user_group_ratios = collectUserGroupRatios(
      privateGroups,
      groupOptions,
      userGroupRatios,
    );
    if (!isRootUser) {
      const invalidGroup = Object.entries(payload.user_group_ratios).find(
        ([group, ratio]) => {
          const option = groupOptions.find((item) => item.value === group);
          const adminRatio = Number(option?.adminRatio);
          return Number.isFinite(adminRatio) && Number(ratio) <= adminRatio;
        },
      );
      if (invalidGroup) {
        const option = groupOptions.find(
          (item) => item.value === invalidGroup[0],
        );
        showError(
          `${invalidGroup[0]}：${t('用户倍率必须大于管理员自身倍率')} ${formatRatio(option?.adminRatio)}`,
        );
        setLoading(false);
        return;
      }
    }
    if (userId) {
      payload.id = parseInt(userId);
    }
    const url = userId ? `/api/user/` : `/api/user/self`;
    const res = await API.put(url, payload);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('用户信息更新成功！'));
      props.refresh();
      props.handleClose();
    } else {
      showError(message);
    }
    setLoading(false);
  };

  /* --------------------- atomic quota adjust -------------------- */
  const adjustQuota = async () => {
    const quotaVal = parseInt(adjustQuotaLocal) || 0;
    if (quotaVal <= 0 && adjustMode !== 'override') return;
    if (
      adjustMode === 'override' &&
      (adjustQuotaLocal === '' || adjustQuotaLocal == null)
    )
      return;
    setAdjustLoading(true);
    try {
      const res = await API.post('/api/user/manage', {
        id: parseInt(userId),
        action: 'add_quota',
        mode: adjustMode,
        value: adjustMode === 'override' ? quotaVal : Math.abs(quotaVal),
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(isRootUser ? t('调整额度成功') : t('调整虚拟额度成功'));
        setAdjustModalOpen(false);
        setAdjustQuotaLocal('');
        setAdjustAmountLocal('');
        const userRes = await API.get(`/api/user/${userId}`);
        if (userRes.data.success) {
          const data = userRes.data.data;
          data.password = '';
          data.quota_amount = Number(
            quotaToDisplayAmount(data.quota || 0).toFixed(6),
          );
          const groups = parseUserGroups(data.group || 'default');
          data.group = groups;
          setSelectedGroups(groups);
          setUserGroupRatios(parseUserGroupRatios(data.setting));
          setInputs({ ...getInitValues(), ...data });
        }
        props.refresh();
      } else {
        showError(message);
      }
    } catch (e) {
      showError(e.message);
    }
    setAdjustLoading(false);
  };

  const getPreviewText = () => {
    const current = formApiRef.current?.getValue('quota') || 0;
    const val = parseInt(adjustQuotaLocal) || 0;
    let result;
    switch (adjustMode) {
      case 'add':
        result = current + Math.abs(val);
        return `${t('当前额度')}：${renderQuota(current)}，+${renderQuota(Math.abs(val))} = ${renderQuota(result)}`;
      case 'subtract':
        result = current - Math.abs(val);
        return `${t('当前额度')}：${renderQuota(current)}，-${renderQuota(Math.abs(val))} = ${renderQuota(result)}`;
      case 'override':
        return `${t('当前额度')}：${renderQuota(current)} → ${renderQuota(val)}`;
      default:
        return '';
    }
  };

  /* --------------------------- UI --------------------------- */
  return (
    <>
      <SideSheet
        placement='right'
        title={
          <Space>
            <Tag color='blue' shape='circle'>
              {t(isEdit ? '编辑' : '新建')}
            </Tag>
            <Title heading={4} className='m-0'>
              {isEdit ? t('编辑用户') : t('创建用户')}
            </Title>
          </Space>
        }
        bodyStyle={{ padding: 0 }}
        visible={props.visible}
        width={isMobile ? '100%' : 600}
        footer={
          <div className='flex justify-end bg-white'>
            <Space>
              <Button
                theme='solid'
                onClick={() => formApiRef.current?.submitForm()}
                icon={<IconSave />}
                loading={loading}
              >
                {t('提交')}
              </Button>
              <Button
                theme='light'
                type='primary'
                onClick={handleCancel}
                icon={<IconClose />}
              >
                {t('取消')}
              </Button>
            </Space>
          </div>
        }
        closeIcon={null}
        onCancel={handleCancel}
      >
        <Spin spinning={loading}>
          <Form
            initValues={getInitValues()}
            getFormApi={(api) => (formApiRef.current = api)}
            onSubmit={submit}
          >
            {({ values }) => (
              <div className='p-2 space-y-3'>
                {/* 基本信息 */}
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='blue'
                      className='mr-2 shadow-md'
                    >
                      <IconUser size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('基本信息')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('用户的基本账户信息')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Input
                        field='username'
                        label={t('用户名')}
                        placeholder={t('请输入新的用户名')}
                        rules={[{ required: true, message: t('请输入用户名') }]}
                        showClear
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='password'
                        label={t('密码')}
                        placeholder={t('请输入新的密码，最短 8 位')}
                        mode='password'
                        showClear
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='display_name'
                        label={t('显示名称')}
                        placeholder={t('请输入新的显示名称')}
                        showClear
                      />
                    </Col>

                    <Col span={24}>
                      <Form.Input
                        field='remark'
                        label={t('备注')}
                        placeholder={t('请输入备注（仅管理员可见）')}
                        showClear
                      />
                    </Col>
                  </Row>
                </Card>

                {/* 权限设置 */}
                {userId && (
                  <Card className='!rounded-2xl shadow-sm border-0'>
                    <div className='flex items-center mb-2'>
                      <Avatar
                        size='small'
                        color='green'
                        className='mr-2 shadow-md'
                      >
                        <IconUserGroup size={16} />
                      </Avatar>
                      <div>
                        <Text className='text-lg font-medium'>
                          {t('权限设置')}
                        </Text>
                        <div className='text-xs text-gray-600'>
                          {t('用户分组和额度管理')}
                        </div>
                      </div>
                    </div>

                    <Row gutter={12}>
                      <Col span={24}>
                        <Form.Slot label={t('分组')}>
                          <div className='border border-gray-200 rounded-lg p-3'>
                            {!isRootUser && (
                              <Text
                                type='tertiary'
                                size='small'
                                className='block mb-2'
                              >
                                {t(
                                  '管理员设置的用户倍率必须严格大于管理员自身对应分组倍率',
                                )}
                              </Text>
                            )}
                            <div className='flex flex-wrap gap-2'>
                              {groupOptions.map((option) => {
                                const checked =
                                  option.isPublic ||
                                  selectedGroups.includes(option.value);
                                return (
                                  <div
                                    key={option.value}
                                    className='flex items-center gap-2 flex-wrap'
                                  >
                                    <Checkbox
                                      checked={checked}
                                      disabled={option.isPublic}
                                      onChange={(event) => {
                                        if (option.isPublic) return;
                                        const nextGroups = toggleUserGroup(
                                          selectedGroups,
                                          option.value,
                                          event.target.checked,
                                        );
                                        setSelectedGroups(nextGroups);
                                        formApiRef.current?.setValue(
                                          'group',
                                          nextGroups,
                                        );
                                        if (!event.target.checked) {
                                          setUserGroupRatios((ratios) => {
                                            const nextRatios = { ...ratios };
                                            delete nextRatios[option.value];
                                            return nextRatios;
                                          });
                                        }
                                      }}
                                    >
                                      <span>{option.label}</span>
                                      {option.ratio !== undefined && (
                                        <Tag
                                          size='small'
                                          color='blue'
                                          className='ml-1'
                                        >
                                          {formatRatio(option.ratio)}
                                        </Tag>
                                      )}
                                      {option.isPublic && (
                                        <Tag
                                          size='small'
                                          color='green'
                                          className='ml-1'
                                        >
                                          {t('公开')}
                                        </Tag>
                                      )}
                                    </Checkbox>
                                    {checked && (
                                      <InputNumber
                                        size='small'
                                        min={0}
                                        precision={6}
                                        step={0.001}
                                        value={userGroupRatios[option.value]}
                                        placeholder={formatRatio(option.ratio)}
                                        style={{ width: 96 }}
                                        onChange={(value) => {
                                          setUserGroupRatios((ratios) => {
                                            const nextRatios = { ...ratios };
                                            if (
                                              value === '' ||
                                              value === null ||
                                              value === undefined
                                            ) {
                                              delete nextRatios[option.value];
                                            } else {
                                              const ratio = Number(value);
                                              if (
                                                Number.isFinite(ratio) &&
                                                ratio >= 0
                                              ) {
                                                nextRatios[option.value] =
                                                  ratio;
                                              }
                                            }
                                            return nextRatios;
                                          });
                                        }}
                                      />
                                    )}
                                  </div>
                                );
                              })}
                            </div>
                            {selectedGroups.length === 0 && (
                              <div className='text-xs text-red-500 mt-2'>
                                {t('请选择分组')}
                              </div>
                            )}
                          </div>
                        </Form.Slot>
                      </Col>

                      <Col span={10}>
                        <Form.InputNumber
                          field='quota_amount'
                          label={t('金额')}
                          prefix={getCurrencyConfig().symbol}
                          precision={6}
                          step={0.000001}
                          style={{ width: '100%' }}
                          readonly
                        />
                      </Col>

                      <Col span={14}>
                        <Form.Slot label={adjustQuotaLabel}>
                          <Button
                            icon={<IconEdit />}
                            onClick={() => setAdjustModalOpen(true)}
                          >
                            {adjustQuotaLabel}
                          </Button>
                        </Form.Slot>
                      </Col>

                      <Col span={24}>
                        <div
                          className='text-xs cursor-pointer'
                          style={{ color: 'var(--semi-color-text-2)' }}
                          onClick={() => setShowQuotaInput((v) => !v)}
                        >
                          {showQuotaInput
                            ? `▾ ${t('收起原生额度输入')}`
                            : `▸ ${t('使用原生额度输入')}`}
                        </div>
                        <div
                          style={{ display: showQuotaInput ? 'block' : 'none' }}
                          className='mt-2'
                        >
                          <Form.InputNumber
                            field='quota'
                            label={t('额度')}
                            placeholder={t('请输入额度')}
                            style={{ width: '100%' }}
                            readonly
                          />
                        </div>
                      </Col>
                    </Row>
                  </Card>
                )}

                {/* 绑定信息入口 */}
                {userId && (
                  <Card className='!rounded-2xl shadow-sm border-0'>
                    <div className='flex items-center justify-between gap-3'>
                      <div className='flex items-center min-w-0'>
                        <Avatar
                          size='small'
                          color='purple'
                          className='mr-2 shadow-md'
                        >
                          <IconLink size={16} />
                        </Avatar>
                        <div className='min-w-0'>
                          <Text className='text-lg font-medium'>
                            {t('绑定信息')}
                          </Text>
                          <div className='text-xs text-gray-600'>
                            {t('管理用户已绑定的第三方账户，支持筛选与解绑')}
                          </div>
                        </div>
                      </div>
                      <Button
                        type='primary'
                        theme='outline'
                        onClick={openBindingModal}
                      >
                        {t('管理绑定')}
                      </Button>
                    </div>
                  </Card>
                )}
              </div>
            )}
          </Form>
        </Spin>
      </SideSheet>

      <UserBindingManagementModal
        visible={bindingModalVisible}
        onCancel={closeBindingModal}
        userId={userId}
        isMobile={isMobile}
        formApiRef={formApiRef}
      />

      {/* 调整额度模态框 */}
      <Modal
        centered
        visible={adjustModalOpen}
        onOk={adjustQuota}
        onCancel={() => {
          setAdjustModalOpen(false);
          setAdjustQuotaLocal('');
          setAdjustAmountLocal('');
          setAdjustMode('add');
        }}
        confirmLoading={adjustLoading}
        closable={null}
        title={
          <div className='flex items-center'>
            <IconEdit className='mr-2' />
            {adjustQuotaLabel}
          </div>
        }
      >
        <div className='mb-4'>
          <Text type='secondary' className='block mb-2'>
            {getPreviewText()}
          </Text>
        </div>
        <div className='mb-3'>
          <div className='mb-1'>
            <Text size='small'>{t('操作')}</Text>
          </div>
          <RadioGroup
            type='button'
            value={adjustMode}
            onChange={(e) => {
              setAdjustMode(e.target.value);
              setAdjustQuotaLocal('');
              setAdjustAmountLocal('');
            }}
            style={{ width: '100%' }}
          >
            <Radio value='add'>{t('添加')}</Radio>
            <Radio value='subtract'>{t('减少')}</Radio>
            <Radio value='override'>{t('覆盖')}</Radio>
          </RadioGroup>
        </div>
        <div className='mb-3'>
          <div className='mb-1'>
            <Text size='small'>{t('金额')}</Text>
          </div>
          <InputNumber
            prefix={getCurrencyConfig().symbol}
            placeholder={t('输入金额')}
            value={adjustAmountLocal}
            precision={6}
            min={adjustMode === 'override' ? undefined : 0}
            step={0.000001}
            onChange={(val) => {
              const amount = val === '' || val == null ? '' : val;
              setAdjustAmountLocal(amount);
              setAdjustQuotaLocal(
                amount === ''
                  ? ''
                  : adjustMode === 'override'
                    ? displayAmountToQuota(amount)
                    : displayAmountToQuota(Math.abs(amount)),
              );
            }}
            style={{ width: '100%' }}
            showClear
          />
        </div>
        <div
          className='text-xs cursor-pointer mt-2'
          style={{ color: 'var(--semi-color-text-2)' }}
          onClick={() => setShowAdjustQuotaRaw((v) => !v)}
        >
          {showAdjustQuotaRaw
            ? `▾ ${t('收起原生额度输入')}`
            : `▸ ${t('使用原生额度输入')}`}
        </div>
        <div
          style={{ display: showAdjustQuotaRaw ? 'block' : 'none' }}
          className='mt-2'
        >
          <div className='mb-1'>
            <Text size='small'>{quotaLabel}</Text>
          </div>
          <InputNumber
            placeholder={t('输入额度')}
            value={adjustQuotaLocal}
            min={adjustMode === 'override' ? undefined : 0}
            onChange={(val) => {
              const quota = val === '' || val == null ? '' : val;
              setAdjustQuotaLocal(quota);
              setAdjustAmountLocal(
                quota === ''
                  ? ''
                  : adjustMode === 'override'
                    ? Number(quotaToDisplayAmount(quota).toFixed(6))
                    : Number(quotaToDisplayAmount(Math.abs(quota)).toFixed(6)),
              );
            }}
            style={{ width: '100%' }}
            showClear
            step={500000}
          />
        </div>
      </Modal>
    </>
  );
};

export default EditUserModal;
