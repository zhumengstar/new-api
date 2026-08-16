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

import React, { useEffect, useMemo, useState } from 'react';
import { Button, Dropdown, Empty, Modal, Tag } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getUsersColumns } from './UsersColumnDefs';
import PromoteUserModal from './modals/PromoteUserModal';
import DemoteUserModal from './modals/DemoteUserModal';
import EnableDisableUserModal from './modals/EnableDisableUserModal';
import DeleteUserModal from './modals/DeleteUserModal';
import ResetPasskeyModal from './modals/ResetPasskeyModal';
import ResetTwoFAModal from './modals/ResetTwoFAModal';
import UserSubscriptionsModal from './modals/UserSubscriptionsModal';
import { API, isRoot, showError, showSuccess } from '../../../helpers';

const BATCH_CONCURRENCY = 5;

const runBatchWithLimit = async (items, worker) => {
  const results = new Array(items.length);
  let nextIndex = 0;

  const runWorker = async () => {
    while (nextIndex < items.length) {
      const index = nextIndex++;
      try {
        results[index] = await worker(items[index]);
      } catch (error) {
        results[index] = {
          success: false,
          message: error?.response?.data?.message || error?.message,
        };
      }
    }
  };

  await Promise.all(
    Array.from(
      { length: Math.min(BATCH_CONCURRENCY, items.length) },
      runWorker,
    ),
  );
  return results;
};

const UsersTable = (usersData) => {
  const {
    users,
    loading,
    activePage,
    pageSize,
    userCount,
    compactMode,
    handlePageChange,
    handlePageSizeChange,
    handleRow,
    setEditingUser,
    setShowEditUser,
    manageUser,
    handleSortChange,
    groupRatios,
    sortBy,
    sortOrder,
    refresh,
    resetUserPasskey,
    resetUserTwoFA,
    t,
  } = usersData;

  // Modal states
  const [showPromoteModal, setShowPromoteModal] = useState(false);
  const [showDemoteModal, setShowDemoteModal] = useState(false);
  const [showEnableDisableModal, setShowEnableDisableModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [showDirectDeleteModal, setShowDirectDeleteModal] = useState(false);
  const [modalUser, setModalUser] = useState(null);
  const [enableDisableAction, setEnableDisableAction] = useState('');
  const [showResetPasskeyModal, setShowResetPasskeyModal] = useState(false);
  const [showResetTwoFAModal, setShowResetTwoFAModal] = useState(false);
  const [showUserSubscriptionsModal, setShowUserSubscriptionsModal] =
    useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [batchLoading, setBatchLoading] = useState(false);

  const rootUser = isRoot();
  const selectedUsers = useMemo(() => {
    const selectedKeys = new Set(selectedRowKeys.map(String));
    return users.filter((user) => selectedKeys.has(String(user.id)));
  }, [selectedRowKeys, users]);

  useEffect(() => {
    const visibleKeys = new Set(users.map((user) => String(user.id)));
    setSelectedRowKeys((keys) =>
      keys.filter((key) => visibleKeys.has(String(key))),
    );
  }, [users]);

  const executeBatchAction = async (action) => {
    if (selectedUsers.length === 0 || batchLoading) return;

    setBatchLoading(true);
    try {
      const results = await runBatchWithLimit(selectedUsers, async (user) => {
        const response = await API.post('/api/user/manage', {
          id: user.id,
          action,
        });
        return response.data;
      });

      const successCount = results.filter((result) => result?.success).length;
      const failedResults = results.filter((result) => !result?.success);
      if (successCount > 0) {
        showSuccess(
          t('批量操作完成: {{success}}个成功, {{failed}}个失败', {
            success: successCount,
            failed: failedResults.length,
          }),
        );
      }
      if (failedResults.length > 0) {
        const firstMessage = failedResults.find(
          (result) => result?.message,
        )?.message;
        showError(firstMessage || t('批量操作失败'));
      }

      setSelectedRowKeys([]);
      await refresh?.();
    } catch (error) {
      showError(
        error?.response?.data?.message || error?.message || t('批量操作失败'),
      );
    } finally {
      setBatchLoading(false);
    }
  };

  const requestBatchAction = (action, label, destructive = false) => {
    if (!destructive) {
      executeBatchAction(action);
      return;
    }
    Modal.confirm({
      title: label,
      content: t('确定要对选中的 {{count}} 个用户执行此操作吗？', {
        count: selectedUsers.length,
      }),
      okType: 'danger',
      onOk: () => executeBatchAction(action),
    });
  };

  // Modal handlers
  const showPromoteUserModal = (user) => {
    setModalUser(user);
    setShowPromoteModal(true);
  };

  const showDemoteUserModal = (user) => {
    setModalUser(user);
    setShowDemoteModal(true);
  };

  const showEnableDisableUserModal = (user, action) => {
    setModalUser(user);
    setEnableDisableAction(action);
    setShowEnableDisableModal(true);
  };

  const showDeleteUserModal = (user) => {
    setModalUser(user);
    setShowDeleteModal(true);
  };

  const showDirectDeleteUserModal = (user) => {
    setModalUser(user);
    setShowDirectDeleteModal(true);
  };

  const showResetPasskeyUserModal = (user) => {
    setModalUser(user);
    setShowResetPasskeyModal(true);
  };

  const showResetTwoFAUserModal = (user) => {
    setModalUser(user);
    setShowResetTwoFAModal(true);
  };

  const showUserSubscriptionsUserModal = (user) => {
    setModalUser(user);
    setShowUserSubscriptionsModal(true);
  };

  // Modal confirm handlers
  const handlePromoteConfirm = () => {
    manageUser(modalUser.id, 'promote', modalUser);
    setShowPromoteModal(false);
  };

  const handleDemoteConfirm = () => {
    manageUser(modalUser.id, 'demote', modalUser);
    setShowDemoteModal(false);
  };

  const handleEnableDisableConfirm = () => {
    manageUser(modalUser.id, enableDisableAction, modalUser);
    setShowEnableDisableModal(false);
  };

  const handleResetPasskeyConfirm = async () => {
    await resetUserPasskey(modalUser);
    setShowResetPasskeyModal(false);
  };

  const handleResetTwoFAConfirm = async () => {
    await resetUserTwoFA(modalUser);
    setShowResetTwoFAModal(false);
  };

  // Get all columns
  const columns = useMemo(() => {
    return getUsersColumns({
      t,
      setEditingUser,
      setShowEditUser,
      showPromoteModal: showPromoteUserModal,
      showDemoteModal: showDemoteUserModal,
      showEnableDisableModal: showEnableDisableUserModal,
      showDeleteModal: showDeleteUserModal,
      showDirectDeleteModal: showDirectDeleteUserModal,
      showResetPasskeyModal: showResetPasskeyUserModal,
      showResetTwoFAModal: showResetTwoFAUserModal,
      showUserSubscriptionsModal: showUserSubscriptionsUserModal,
      manageUser,
      showWeChatContact: rootUser,
      groupRatios,
      sortBy,
      sortOrder,
      refresh,
    });
  }, [
    t,
    setEditingUser,
    setShowEditUser,
    showPromoteUserModal,
    showDemoteUserModal,
    showEnableDisableUserModal,
    showDeleteUserModal,
    showDirectDeleteUserModal,
    showResetPasskeyUserModal,
    showResetTwoFAUserModal,
    showUserSubscriptionsUserModal,
    manageUser,
    groupRatios,
    sortBy,
    sortOrder,
    rootUser,
    refresh,
  ]);

  // Handle compact mode by removing fixed positioning
  const tableColumns = useMemo(() => {
    return compactMode
      ? columns.map((col) => {
          if (col.dataIndex === 'operate') {
            const { fixed, ...rest } = col;
            return rest;
          }
          return col;
        })
      : columns;
  }, [compactMode, columns]);

  return (
    <>
      {selectedUsers.length > 0 && (
        <div className='mb-2 flex flex-wrap items-center gap-2'>
          <Tag color='blue' shape='circle'>
            {t('已选择 {{count}} 个用户', { count: selectedUsers.length })}
          </Tag>
          <Dropdown
            trigger='click'
            render={
              <Dropdown.Menu>
                <Dropdown.Item
                  onClick={() => requestBatchAction('enable', t('批量启用'))}
                >
                  {t('批量启用')}
                </Dropdown.Item>
                <Dropdown.Item
                  onClick={() => requestBatchAction('disable', t('批量禁用'))}
                >
                  {t('批量禁用')}
                </Dropdown.Item>
                {rootUser && (
                  <>
                    <Dropdown.Divider />
                    <Dropdown.Item
                      onClick={() => requestBatchAction('hide', t('批量隐藏'))}
                    >
                      {t('批量隐藏')}
                    </Dropdown.Item>
                    <Dropdown.Item
                      onClick={() =>
                        requestBatchAction('unhide', t('批量取消隐藏'))
                      }
                    >
                      {t('批量取消隐藏')}
                    </Dropdown.Item>
                  </>
                )}
                <Dropdown.Divider />
                <Dropdown.Item
                  type='danger'
                  onClick={() =>
                    requestBatchAction('delete', t('批量逻辑删除'), true)
                  }
                >
                  {t('批量逻辑删除')}
                </Dropdown.Item>
              </Dropdown.Menu>
            }
          >
            <Button size='small' loading={batchLoading}>
              {t('批量操作')}
            </Button>
          </Dropdown>
          <Button
            size='small'
            type='tertiary'
            disabled={batchLoading}
            onClick={() => setSelectedRowKeys([])}
          >
            {t('取消选择')}
          </Button>
        </div>
      )}
      <CardTable
        columns={tableColumns}
        dataSource={users}
        scroll={compactMode ? undefined : { x: 'max-content' }}
        pagination={{
          currentPage: activePage,
          pageSize: pageSize,
          total: userCount,
          pageSizeOpts: [10, 20, 50, 100],
          showSizeChanger: true,
          onPageSizeChange: handlePageSizeChange,
          onPageChange: handlePageChange,
        }}
        hidePagination={true}
        loading={loading}
        onRow={handleRow}
        onChange={handleSortChange}
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
          getCheckboxProps: (record) => ({
            disabled: record.role >= 100 || Boolean(record.DeletedAt),
            name: String(record.id),
          }),
        }}
        empty={
          <Empty
            image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('搜索无结果')}
            style={{ padding: 30 }}
          />
        }
        className='overflow-hidden'
        size='middle'
      />

      {/* Modal components */}
      <PromoteUserModal
        visible={showPromoteModal}
        onCancel={() => setShowPromoteModal(false)}
        onConfirm={handlePromoteConfirm}
        user={modalUser}
        t={t}
      />

      <DemoteUserModal
        visible={showDemoteModal}
        onCancel={() => setShowDemoteModal(false)}
        onConfirm={handleDemoteConfirm}
        user={modalUser}
        t={t}
      />

      <EnableDisableUserModal
        visible={showEnableDisableModal}
        onCancel={() => setShowEnableDisableModal(false)}
        onConfirm={handleEnableDisableConfirm}
        user={modalUser}
        action={enableDisableAction}
        t={t}
      />

      <DeleteUserModal
        visible={showDeleteModal}
        onCancel={() => setShowDeleteModal(false)}
        user={modalUser}
        users={users}
        activePage={activePage}
        refresh={refresh}
        manageUser={manageUser}
        t={t}
      />

      <DeleteUserModal
        visible={showDirectDeleteModal}
        onCancel={() => setShowDirectDeleteModal(false)}
        user={modalUser}
        users={users}
        activePage={activePage}
        refresh={refresh}
        manageUser={manageUser}
        directDelete
        t={t}
      />

      <ResetPasskeyModal
        visible={showResetPasskeyModal}
        onCancel={() => setShowResetPasskeyModal(false)}
        onConfirm={handleResetPasskeyConfirm}
        user={modalUser}
        t={t}
      />

      <ResetTwoFAModal
        visible={showResetTwoFAModal}
        onCancel={() => setShowResetTwoFAModal(false)}
        onConfirm={handleResetTwoFAConfirm}
        user={modalUser}
        t={t}
      />

      <UserSubscriptionsModal
        visible={showUserSubscriptionsModal}
        onCancel={() => setShowUserSubscriptionsModal(false)}
        user={modalUser}
        t={t}
        onSuccess={() => refresh?.()}
      />
    </>
  );
};

export default UsersTable;
