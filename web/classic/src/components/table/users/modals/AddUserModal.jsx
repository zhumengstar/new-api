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
import { API, showError, showSuccess } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  Button,
  SideSheet,
  Space,
  Spin,
  Typography,
  Card,
  Tag,
  Avatar,
  Form,
  Row,
  Col,
  Checkbox,
  InputNumber,
} from '@douyinfe/semi-ui';
import { IconSave, IconClose, IconUserAdd } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

const joinUserGroups = (groups) => {
  return Array.from(
    new Set(
      (Array.isArray(groups) ? groups : String(groups || '').split(','))
        .map((item) => String(item).trim())
        .filter(Boolean),
    ),
  ).join(',');
};

const parseUserGroups = (group) => {
  if (Array.isArray(group)) {
    return group.map((item) => String(item).trim()).filter(Boolean);
  }
  return String(group || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
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

const AddUserModal = (props) => {
  const { t } = useTranslation();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [groupOptions, setGroupOptions] = useState([]);
  const [userGroupRatios, setUserGroupRatios] = useState({});
  const isMobile = useIsMobile();

  const getInitValues = () => ({
    username: '',
    display_name: '',
    password: '',
    group: [],
    remark: '',
  });

  const fetchGroups = async () => {
    try {
      const res = await API.get('/api/group/detail');
      const payload = res.data.data;
      const groups = Array.isArray(payload) ? payload : payload?.groups || [];
      const meta = Array.isArray(payload) ? {} : payload?.meta || {};
      setGroupOptions(
        groups.map((g) => ({
          label: g,
          value: g,
          ratio: meta[g]?.ratio,
          isPublic: Boolean(meta[g]?.is_public),
        })),
      );
    } catch (e) {
      showError(e.message);
    }
  };

  useEffect(() => {
    if (props.visible) {
      fetchGroups();
    }
  }, [props.visible]);

  const submit = async (values) => {
    setLoading(true);
    const selectedGroups = parseUserGroups(values.group);
    const payload = {
      ...values,
      group: joinUserGroups(selectedGroups),
      user_group_ratios: selectedGroups.reduce((ratios, group) => {
        const ratio = Number(userGroupRatios[group]);
        if (Number.isFinite(ratio) && ratio >= 0) {
          ratios[group] = ratio;
        }
        return ratios;
      }, {}),
    };
    const res = await API.post(`/api/user/`, payload);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('用户账户创建成功！'));
      formApiRef.current?.setValues(getInitValues());
      setUserGroupRatios({});
      props.refresh();
      props.handleClose();
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const handleCancel = () => {
    setUserGroupRatios({});
    props.handleClose();
  };

  return (
    <>
      <SideSheet
        placement={'left'}
        title={
          <Space>
            <Tag color='green' shape='circle'>
              {t('新建')}
            </Tag>
            <Title heading={4} className='m-0'>
              {t('添加用户')}
            </Title>
          </Space>
        }
        bodyStyle={{ padding: '0' }}
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
        onCancel={() => handleCancel()}
      >
        <Spin spinning={loading}>
          <Form
            initValues={getInitValues()}
            getFormApi={(api) => (formApiRef.current = api)}
            onSubmit={submit}
            onSubmitFail={(errs) => {
              const first = Object.values(errs)[0];
              if (first) showError(Array.isArray(first) ? first[0] : first);
              formApiRef.current?.scrollToError();
            }}
          >
            {({ values }) => (
              <div className='p-2'>
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='blue'
                      className='mr-2 shadow-md'
                    >
                      <IconUserAdd size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('用户信息')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('创建新用户账户')}
                      </div>
                    </div>
                  </div>

                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Input
                        field='username'
                        label={t('用户名')}
                        placeholder={t('请输入用户名')}
                        rules={[{ required: true, message: t('请输入用户名') }]}
                        showClear
                      />
                    </Col>
                    <Col span={24}>
                      <Form.Input
                        field='display_name'
                        label={t('显示名称')}
                        placeholder={t('请输入显示名称')}
                        showClear
                      />
                    </Col>
                    <Col span={24}>
                      <Form.Input
                        field='password'
                        label={t('密码')}
                        type='password'
                        placeholder={t('请输入密码')}
                        rules={[{ required: true, message: t('请输入密码') }]}
                        showClear
                      />
                    </Col>
                    <Col span={24}>
                      <Form.Slot label={t('分组')}>
                        <div className='border border-gray-200 rounded-lg p-3'>
                          <div className='flex flex-wrap gap-2'>
                            {groupOptions.map((option) => {
                              const selectedGroups = parseUserGroups(
                                values.group,
                              );
                              const checked = selectedGroups.includes(
                                option.value,
                              );
                              return (
                                <div
                                  key={option.value}
                                  className='flex items-center gap-2 flex-wrap'
                                >
                                  <Checkbox
                                    checked={checked}
                                    onChange={(event) => {
                                      const nextGroups = toggleUserGroup(
                                        values.group,
                                        option.value,
                                        event.target.checked,
                                      );
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
                                              nextRatios[option.value] = ratio;
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
                          {groupOptions.length === 0 && (
                            <div className='text-xs text-gray-500'>
                              {t('暂无分组')}
                            </div>
                          )}
                        </div>
                      </Form.Slot>
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
              </div>
            )}
          </Form>
        </Spin>
      </SideSheet>
    </>
  );
};

export default AddUserModal;
