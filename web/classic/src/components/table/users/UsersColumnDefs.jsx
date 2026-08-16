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

import React, { useEffect, useRef, useState } from 'react';
import {
  Button,
  Space,
  Tag,
  Tooltip,
  Progress,
  Popover,
  Typography,
  Dropdown,
  Input,
} from '@douyinfe/semi-ui';
import { IconCopy, IconEdit, IconMore } from '@douyinfe/semi-icons';
import {
  renderGroup,
  renderNumber,
  renderQuota,
  timestamp2string,
  API,
  copy,
  showError,
  showSuccess,
} from '../../../helpers';
import {
  getUserContactItems,
  getUserContactValue,
  parseUserContact,
} from './userContact';

const renderTimestamp = (text) => (text ? timestamp2string(text) : '-');

/**
 * Render user role
 */
const renderRole = (role, t) => {
  switch (role) {
    case 1:
      return (
        <Tag color='blue' shape='circle'>
          {t('普通用户')}
        </Tag>
      );
    case 10:
      return (
        <Tag color='yellow' shape='circle'>
          {t('管理员')}
        </Tag>
      );
    case 100:
      return (
        <Tag color='orange' shape='circle'>
          {t('超级管理员')}
        </Tag>
      );
    default:
      return (
        <Tag color='red' shape='circle'>
          {t('未知身份')}
        </Tag>
      );
  }
};

const EditableUserField = ({
  record,
  field,
  label,
  successMessage,
  errorMessage,
  t,
  refresh,
  maxLength = 64,
  emptyValue = '-',
  renderValue,
}) => {
  const originalValue = record[field] || '';
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(originalValue);
  const [saving, setSaving] = useState(false);
  const savingRef = useRef(false);
  const skipBlurSaveRef = useRef(false);

  useEffect(() => {
    if (!editing) {
      setValue(originalValue);
    }
  }, [editing, originalValue]);

  const cancel = () => {
    skipBlurSaveRef.current = true;
    setValue(originalValue);
    setEditing(false);
  };

  const save = async () => {
    const nextValue = value.trim();
    if (savingRef.current) return;
    if (nextValue === originalValue) {
      setEditing(false);
      return;
    }

    savingRef.current = true;
    setSaving(true);
    try {
      const res = await API.put('/api/user/', {
        ...record,
        password: '',
        original_password: '',
        [field]: nextValue,
      });
      if (res.data.success) {
        showSuccess(t(successMessage));
        setEditing(false);
        refresh();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(t(errorMessage));
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };

  if (editing) {
    return (
      <Input
        autoFocus
        value={value}
        maxLength={maxLength}
        aria-label={t(label)}
        disabled={saving}
        onChange={setValue}
        onClick={(event) => event.stopPropagation()}
        onBlur={() => {
          if (skipBlurSaveRef.current) {
            skipBlurSaveRef.current = false;
            return;
          }
          void save();
        }}
        onEnterPress={() => void save()}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            event.preventDefault();
            cancel();
          }
        }}
      />
    );
  }

  return (
    <span
      className='cursor-text'
      onClick={(event) => {
        event.stopPropagation();
        setEditing(true);
      }}
    >
      {renderValue ? renderValue(originalValue) : originalValue || emptyValue}
    </span>
  );
};

const EditableUsernameRemark = ({ text, record, t, refresh }) => {
  const renderRemark = (remark) => {
    if (!remark) {
      return (
        <Typography.Text type='tertiary' size='small'>
          + {t('备注')}
        </Typography.Text>
      );
    }
    const displayRemark =
      remark.length > 10 ? `${remark.slice(0, 10)}…` : remark;
    return (
      <Tooltip content={remark} position='top' showArrow>
        <Tag color='white' shape='circle' className='!text-xs'>
          {displayRemark}
        </Tag>
      </Tooltip>
    );
  };

  return (
    <Space spacing={4}>
      <span>{text}</span>
      <EditableUserField
        record={record}
        field='remark'
        label='备注'
        successMessage='备注已更新'
        errorMessage='备注更新失败'
        t={t}
        refresh={refresh}
        maxLength={255}
        emptyValue={t('备注')}
        renderValue={renderRemark}
      />
    </Space>
  );
};

const EditableUserContact = ({ record, t, refresh }) => {
  const originalValue = getUserContactValue(record);
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(originalValue);
  const [saving, setSaving] = useState(false);
  const savingRef = useRef(false);
  const skipBlurSaveRef = useRef(false);

  useEffect(() => {
    if (!editing) setValue(originalValue);
  }, [editing, originalValue]);

  const cancel = () => {
    skipBlurSaveRef.current = true;
    setValue(originalValue);
    setEditing(false);
  };

  const save = async () => {
    const nextValue = value.trim();
    if (savingRef.current) return;
    if (nextValue === originalValue) {
      setEditing(false);
      return;
    }

    const { qqContact, wechatContact } = parseUserContact(nextValue);
    savingRef.current = true;
    setSaving(true);
    try {
      const res = await API.put('/api/user/', {
        ...record,
        password: '',
        original_password: '',
        qq_contact: qqContact,
        wechat_contact: wechatContact,
      });
      if (res.data.success) {
        showSuccess(t('操作成功完成！'));
        setEditing(false);
        refresh();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(t('操作失败'));
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };

  if (editing) {
    return (
      <Input
        autoFocus
        value={value}
        maxLength={130}
        aria-label={`${t('微信')} / QQ`}
        placeholder={t('输入微信号或QQ号，多个用逗号分隔')}
        disabled={saving}
        onChange={setValue}
        onClick={(event) => event.stopPropagation()}
        onBlur={() => {
          if (skipBlurSaveRef.current) {
            skipBlurSaveRef.current = false;
            return;
          }
          void save();
        }}
        onEnterPress={() => void save()}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            event.preventDefault();
            cancel();
          }
        }}
      />
    );
  }

  const contacts = getUserContactItems(record);
  const copyContact = async (event) => {
    event.stopPropagation();
    if (!originalValue) return;
    if (await copy(originalValue)) {
      showSuccess(t('已复制到剪贴板'));
    } else {
      showError(t('复制失败'));
    }
  };

  return (
    <div className='flex min-w-0 items-center gap-1'>
      <Space
        wrap
        spacing={4}
        className='min-w-0 flex-1 cursor-text'
        onClick={(event) => {
          event.stopPropagation();
          setEditing(true);
        }}
      >
        {contacts.length === 0
          ? '-'
          : contacts.map((contact) => (
              <Tooltip
                key={`${contact.type}-${contact.value}`}
                content={
                  contact.inferred ? `${t('用户名')} → QQ` : contact.value
                }
              >
                <Tag
                  color={contact.type === 'QQ' ? 'blue' : 'green'}
                  shape='circle'
                  size='small'
                >
                  {contact.type === 'QQ' ? 'QQ' : t('微信')}：{contact.value}
                </Tag>
              </Tooltip>
            ))}
      </Space>
      <Tooltip content={t('复制')}>
        <Button
          aria-label={t('复制')}
          icon={<IconCopy />}
          size='small'
          theme='borderless'
          type='tertiary'
          disabled={!originalValue}
          onClick={copyContact}
        />
      </Tooltip>
      <Tooltip content={t('编辑')}>
        <Button
          aria-label={t('编辑')}
          icon={<IconEdit />}
          size='small'
          theme='borderless'
          type='tertiary'
          onClick={(event) => {
            event.stopPropagation();
            setEditing(true);
          }}
        />
      </Tooltip>
    </div>
  );
};

const formatGroupRatio = (ratio) => {
  const numericRatio = Number(ratio);
  return Number.isFinite(numericRatio) ? numericRatio.toString() : '1';
};

const renderGroupsWithRatios = (value, effectiveRatios, groupRatios) => {
  const effectiveGroups = Object.keys(effectiveRatios || {});
  const groups = (
    effectiveGroups.length > 0
      ? effectiveGroups
      : (value || 'default')
          .split(',')
          .map((group) => group.trim())
          .filter(Boolean)
  ).sort();

  return (
    <Space wrap spacing={4}>
      {groups.map((group) => (
        <span key={group} className='inline-flex items-center gap-1'>
          {renderGroup(group)}
          <Typography.Text type='tertiary' size='small'>
            ×{' '}
            {formatGroupRatio(
              effectiveRatios?.[group] ?? groupRatios[group] ?? 1,
            )}
          </Typography.Text>
        </span>
      ))}
    </Space>
  );
};

/**
 * Render user statistics
 */
const renderStatistics = (text, record, showEnableDisableModal, t) => {
  const isDeleted = record.DeletedAt !== null;

  // Determine tag text & color like original status column
  let tagColor = 'grey';
  let tagText = t('未知状态');
  if (isDeleted) {
    tagColor = 'red';
    tagText = t('已注销');
  } else if (record.status === 1) {
    tagColor = 'green';
    tagText = t('已启用');
  } else if (record.status === 2) {
    tagColor = 'red';
    tagText = t('已禁用');
  }

  const content = (
    <Space spacing={4}>
      <Tag color={tagColor} shape='circle' size='small'>
        {tagText}
      </Tag>
      {record.is_hidden && (
        <Tag color='grey' shape='circle' size='small'>
          {t('已隐藏')}
        </Tag>
      )}
    </Space>
  );

  const tooltipContent = (
    <div className='text-xs'>
      <div>
        {t('调用次数')}: {renderNumber(record.request_count)}
      </div>
    </div>
  );

  return (
    <Tooltip content={tooltipContent} position='top'>
      {content}
    </Tooltip>
  );
};

// Render separate quota usage column
const renderQuotaUsage = (text, record, t) => {
  const { Paragraph } = Typography;
  const used = parseInt(record.used_quota) || 0;
  const remain = parseInt(record.quota) || 0;
  const total = used + remain;
  const percent = total > 0 ? (remain / total) * 100 : 0;
  const popoverContent = (
    <div className='text-xs p-2'>
      <Paragraph copyable={{ content: renderQuota(used) }}>
        {t('已用额度')}: {renderQuota(used)}
      </Paragraph>
      <Paragraph copyable={{ content: renderQuota(remain) }}>
        {t('剩余额度')}: {renderQuota(remain)} ({percent.toFixed(0)}%)
      </Paragraph>
      <Paragraph copyable={{ content: renderQuota(total) }}>
        {t('总额度')}: {renderQuota(total)}
      </Paragraph>
    </div>
  );
  return (
    <Popover content={popoverContent} position='top'>
      <Tag color='white' shape='circle'>
        <div className='flex flex-col items-end'>
          <span className='text-xs leading-none'>{`${renderQuota(remain)} / ${renderQuota(total)}`}</span>
          <Progress
            percent={percent}
            aria-label='quota usage'
            format={() => `${percent.toFixed(0)}%`}
            style={{ width: '100%', marginTop: '1px', marginBottom: 0 }}
          />
        </div>
      </Tag>
    </Popover>
  );
};

/**
 * Render invite information
 */
const renderInviteInfo = (text, record, t) => {
  return (
    <div>
      <Space spacing={1}>
        <Tag color='white' shape='circle' className='!text-xs'>
          {t('邀请')}: {renderNumber(record.aff_count)}
        </Tag>
        <Tag color='white' shape='circle' className='!text-xs'>
          {t('收益')}: {renderQuota(record.aff_history_quota)}
        </Tag>
        <Tag color='white' shape='circle' className='!text-xs'>
          {record.inviter_id === 0
            ? t('无邀请人')
            : `${t('邀请人')}: ${record.inviter_id}`}
        </Tag>
      </Space>
    </div>
  );
};

/**
 * Render operations column
 */
const renderOperations = (
  text,
  record,
  {
    setEditingUser,
    setShowEditUser,
    showPromoteModal,
    showDemoteModal,
    showEnableDisableModal,
    showDeleteModal,
    showDirectDeleteModal,
    showResetPasskeyModal,
    showResetTwoFAModal,
    showUserSubscriptionsModal,
    manageUser,
    showHiddenUserAction,
    t,
  },
) => {
  if (record.DeletedAt !== null) {
    return <></>;
  }

  const moreMenu = [
    {
      node: 'item',
      name: t('订阅管理'),
      onClick: () => showUserSubscriptionsModal(record),
    },
    {
      node: 'divider',
    },
    {
      node: 'item',
      name: t('重置 Passkey'),
      onClick: () => showResetPasskeyModal(record),
    },
    {
      node: 'item',
      name: t('重置 2FA'),
      onClick: () => showResetTwoFAModal(record),
    },
    {
      node: 'divider',
    },
    {
      node: 'item',
      name: t('注销'),
      type: 'danger',
      onClick: () => showDeleteModal(record),
    },
    {
      node: 'item',
      name: t('直接删除'),
      type: 'danger',
      onClick: () => showDirectDeleteModal(record),
    },
  ];

  return (
    <Space>
      {record.status === 1 ? (
        <Button
          type='danger'
          size='small'
          onClick={() => showEnableDisableModal(record, 'disable')}
        >
          {t('禁用')}
        </Button>
      ) : (
        <Button
          size='small'
          onClick={() => showEnableDisableModal(record, 'enable')}
        >
          {t('启用')}
        </Button>
      )}
      {showHiddenUserAction && (
        <Button
          type='tertiary'
          size='small'
          onClick={() =>
            manageUser(record.id, record.is_hidden ? 'unhide' : 'hide', record)
          }
        >
          {record.is_hidden ? t('显示') : t('隐藏')}
        </Button>
      )}
      <Button
        type='tertiary'
        size='small'
        onClick={() => {
          setEditingUser(record);
          setShowEditUser(true);
        }}
      >
        {t('编辑')}
      </Button>
      <Button
        type='warning'
        size='small'
        onClick={() => showPromoteModal(record)}
      >
        {t('提升')}
      </Button>
      <Button
        type='secondary'
        size='small'
        onClick={() => showDemoteModal(record)}
      >
        {t('降级')}
      </Button>
      <Dropdown menu={moreMenu} trigger='click' position='bottomRight'>
        <Button type='tertiary' size='small' icon={<IconMore />} />
      </Dropdown>
    </Space>
  );
};

/**
 * Get users table column definitions
 */
export const getUsersColumns = ({
  t,
  setEditingUser,
  setShowEditUser,
  showPromoteModal,
  showDemoteModal,
  showEnableDisableModal,
  showDeleteModal,
  showDirectDeleteModal,
  showResetPasskeyModal,
  showResetTwoFAModal,
  showUserSubscriptionsModal,
  manageUser,
  showWeChatContact,
  groupRatios,
  sortBy,
  sortOrder,
  refresh,
}) => {
  const getSortOrder = (field) => {
    if (sortBy !== field) return false;
    return sortOrder === 'desc' ? 'descend' : 'ascend';
  };

  return [
    {
      title: 'ID',
      dataIndex: 'id',
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      render: (text, record) => (
        <EditableUsernameRemark
          text={text}
          record={record}
          t={t}
          refresh={refresh}
        />
      ),
    },
    ...(showWeChatContact
      ? [
          {
            title: `${t('微信')} / QQ`,
            dataIndex: 'contact',
            render: (text, record) => (
              <EditableUserContact record={record} t={t} refresh={refresh} />
            ),
          },
        ]
      : []),
    {
      title: t('状态'),
      dataIndex: 'info',
      render: (text, record, index) =>
        renderStatistics(text, record, showEnableDisableModal, t),
    },
    {
      title: t('今日消耗金额'),
      dataIndex: 'today_consumed_quota',
      sorter: true,
      sortOrder: getSortOrder('today_consumed_quota'),
      render: (text) => (
        <Tag color='white' shape='circle'>
          {renderQuota(text || 0)}
        </Tag>
      ),
    },
    {
      title: t('总消耗金额'),
      dataIndex: 'total_consumed_quota',
      sorter: true,
      sortOrder: getSortOrder('total_consumed_quota'),
      render: (text) => (
        <Tag color='white' shape='circle'>
          {renderQuota(text || 0)}
        </Tag>
      ),
    },
    {
      title: t('剩余额度/总额度'),
      dataIndex: 'quota',
      key: 'quota_usage',
      sorter: true,
      sortOrder: getSortOrder('quota'),
      render: (text, record) => renderQuotaUsage(text, record, t),
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      render: (text, record) =>
        renderGroupsWithRatios(
          text,
          record.effective_group_ratios,
          groupRatios,
        ),
    },
    {
      title: t('角色'),
      dataIndex: 'role',
      render: (text, record, index) => {
        return <div>{renderRole(text, t)}</div>;
      },
    },
    {
      title: t('邀请信息'),
      dataIndex: 'invite',
      render: (text, record, index) => renderInviteInfo(text, record, t),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      render: renderTimestamp,
    },
    {
      title: t('最后登录'),
      dataIndex: 'last_login_at',
      render: renderTimestamp,
    },
    {
      title: '',
      dataIndex: 'operate',
      fixed: 'right',
      width: 200,
      render: (text, record, index) =>
        renderOperations(text, record, {
          setEditingUser,
          setShowEditUser,
          showPromoteModal,
          showDemoteModal,
          showEnableDisableModal,
          showDeleteModal,
          showDirectDeleteModal,
          showResetPasskeyModal,
          showResetTwoFAModal,
          showUserSubscriptionsModal,
          manageUser,
          showHiddenUserAction: showWeChatContact,
          t,
        }),
    },
  ];
};
