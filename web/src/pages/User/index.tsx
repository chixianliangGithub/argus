import React, { useEffect, useRef, useState } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Button, message, Popconfirm, Switch, Tag, Typography } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { getUsers, addUser, updateUser, deleteUser } from '../../services/user';
import { ModalForm, ProFormText, ProFormSelect } from '@ant-design/pro-components';
import { getGlobalTwoFASetting, setGlobalTwoFASetting } from '../../services/systemSettings';

const { Text } = Typography;

const UserList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [global2FAEnabled, setGlobal2FAEnabled] = useState(false);
  const [global2FALoading, setGlobal2FALoading] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const r: any = await getGlobalTwoFASetting();
        setGlobal2FAEnabled(!!r?.enabled);
      } catch (e) {}
    })();
  }, []);

  const columns: ProColumns[] = [
    {
      title: '用户名',
      dataIndex: 'username',
      copyable: true,
      ellipsis: true,
      formItemProps: {
        rules: [{ required: true, message: '此项为必填项' }],
      },
    },
    {
      title: '邮箱',
      dataIndex: 'email',
      hideInSearch: true,
    },
    {
      title: '手机号',
      dataIndex: 'phone',
      hideInSearch: true,
    },
    {
      title: '部门',
      dataIndex: 'department',
      hideInSearch: true,
    },
    {
      title: '角色',
      dataIndex: 'role',
      valueType: 'select',
      valueEnum: {
        admin: { text: '管理员', status: 'Success' },
        user: { text: '普通用户', status: 'Default' },
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      valueType: 'select',
      valueEnum: {
        active: { text: '正常', status: 'Success' },
        disabled: { text: '禁用', status: 'Error' },
      },
      render: (_, record) => (
        <Tag color={record.status === 'active' ? 'green' : 'red'}>
          {record.status === 'active' ? '正常' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '2FA',
      dataIndex: 'two_fa_enabled',
      hideInSearch: true,
      width: 90,
      render: (_, record: any) => (
        <Tag color={record.two_fa_enabled ? 'green' : 'default'}>{record.two_fa_enabled ? '已启用' : '未启用'}</Tag>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_text, record, _, _action) => [
        <ModalForm
          key="edit"
          title="编辑用户"
          trigger={<a>编辑</a>}
          onFinish={async (values) => {
            try {
              await updateUser(record.id, values);
              message.success('更新成功');
              actionRef.current?.reload();
              return true;
            } catch (error) {
              message.error('更新失败');
              return false;
            }
          }}
          initialValues={record}
        >
          <ProFormText name="email" label="邮箱" />
          <ProFormText name="phone" label="手机号" />
          <ProFormText name="department" label="部门" />
          <ProFormSelect
            name="role"
            label="角色"
            options={[
              { label: '管理员', value: 'admin' },
              { label: '普通用户', value: 'user' },
            ]}
            rules={[{ required: true }]}
          />
          <ProFormSelect
            name="status"
            label="状态"
            options={[
              { label: '正常', value: 'active' },
              { label: '禁用', value: 'disabled' },
            ]}
            rules={[{ required: true }]}
          />
          <ProFormText.Password name="password" label="重置密码 (留空不修改)" />
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteUser(record.id);
              message.success('删除成功');
              actionRef.current?.reload();
            } catch (error) {
              message.error('删除失败');
            }
          }}
        >
          <a>删除</a>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <ProTable
      columns={columns}
      actionRef={actionRef}
      cardBordered
      request={async (params) => {
        try {
          const res = await getUsers(params);
          // Backend returns array directly for now
          const data = Array.isArray(res) ? res : (res.data || []);
          return {
            data: data,
            success: true,
            total: data.length,
          };
        } catch (error) {
          return {
            data: [],
            success: false,
          };
        }
      }}
      rowKey="id"
      search={{
        labelWidth: 'auto',
      }}
      pagination={{
        pageSize: 10,
      }}
      dateFormatter="string"
      headerTitle="用户列表"
      toolBarRender={() => [
        <div key="global-2fa" style={{ display: 'flex', alignItems: 'center', gap: 8, marginRight: 12 }}>
          <Text>全局2FA</Text>
          <Switch
            checked={global2FAEnabled}
            loading={global2FALoading}
            onChange={async (checked) => {
              setGlobal2FALoading(true);
              try {
                await setGlobalTwoFASetting(checked);
                setGlobal2FAEnabled(checked);
                message.success(checked ? '已开启全局2FA，将重新登录' : '已关闭全局2FA，将重新登录');
                localStorage.removeItem('token');
                window.location.href = '/login';
              } catch (e: any) {
                const err = e?.response?.data?.error || e?.message || '设置失败';
                message.error(String(err));
              } finally {
                setGlobal2FALoading(false);
              }
            }}
          />
        </div>,
        <ModalForm
          key="create"
          title="新建用户"
          trigger={
            <Button type="primary">
              <PlusOutlined />
              新建
            </Button>
          }
          onFinish={async (values) => {
            try {
              await addUser(values);
              message.success('添加成功');
              actionRef.current?.reload();
              return true;
            } catch (error) {
              message.error('添加失败');
              return false;
            }
          }}
        >
          <ProFormText name="username" label="用户名" rules={[{ required: true }]} />
          <ProFormText.Password name="password" label="密码" rules={[{ required: true }]} />
          <ProFormText name="email" label="邮箱" />
          <ProFormText name="phone" label="手机号" />
          <ProFormText name="department" label="部门" />
          <ProFormSelect
            name="role"
            label="角色"
            options={[
              { label: '管理员', value: 'admin' },
              { label: '普通用户', value: 'user' },
            ]}
            rules={[{ required: true }]}
            initialValue="user"
          />
          <ProFormSelect
            name="status"
            label="状态"
            options={[
              { label: '正常', value: 'active' },
              { label: '禁用', value: 'disabled' },
            ]}
            rules={[{ required: true }]}
            initialValue="active"
          />
        </ModalForm>,
      ]}
    />
  );
};

export default UserList;
