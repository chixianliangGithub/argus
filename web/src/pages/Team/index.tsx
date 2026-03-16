import React, { useRef, useState } from 'react';
import { ProTable, type ProColumns, type ActionType } from '@ant-design/pro-components';
import { Button, message, Popconfirm, Tag, Space, Drawer, List, Avatar } from 'antd';
import { PlusOutlined, UserOutlined, DeleteOutlined } from '@ant-design/icons';
import { getTeams, addTeam, updateTeam, deleteTeam, addTeamMember, removeTeamMember } from '../../services/team';
import { getUsers } from '../../services/user';
import { ModalForm, ProFormText, ProFormTextArea, ProFormSelect } from '@ant-design/pro-components';

const TeamList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [memberDrawerVisible, setMemberDrawerVisible] = useState(false);
  const [currentTeam, setCurrentTeam] = useState<any>(null);
  const [users, setUsers] = useState<any[]>([]);

  const loadUsers = async () => {
    try {
      const res = await getUsers();
      setUsers(Array.isArray(res) ? res : (res.data || []));
    } catch (error) {
      console.error(error);
    }
  };

  const handleManageMembers = (record: any) => {
    setCurrentTeam(record);
    loadUsers();
    setMemberDrawerVisible(true);
  };

  const handleAddMember = async (userId: number) => {
    try {
      await addTeamMember(currentTeam.id, userId);
      message.success('添加成功');
      actionRef.current?.reload();
      // Update local state for immediate feedback
      setCurrentTeam({
        ...currentTeam,
        users: [...(currentTeam.users || []), users.find(u => u.id === userId)]
      });
    } catch (error) {
      message.error('添加失败');
    }
  };

  const handleRemoveMember = async (userId: number) => {
    try {
      await removeTeamMember(currentTeam.id, userId);
      message.success('移除成功');
      actionRef.current?.reload();
      // Update local state
      setCurrentTeam({
        ...currentTeam,
        users: currentTeam.users.filter((u: any) => u.id !== userId)
      });
    } catch (error) {
      message.error('移除失败');
    }
  };

  const columns: ProColumns[] = [
    {
      title: '团队名称',
      dataIndex: 'name',
      copyable: true,
      formItemProps: {
        rules: [{ required: true, message: '此项为必填项' }],
      },
    },
    {
      title: '描述',
      dataIndex: 'description',
      hideInSearch: true,
    },
    {
      title: '成员数量',
      dataIndex: 'users',
      hideInSearch: true,
      render: (_, record) => <Tag color="blue">{record.users?.length || 0} 人</Tag>,
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_text, record, _, _action) => [
        <a key="members" onClick={() => handleManageMembers(record)}>
          成员管理
        </a>,
        <ModalForm
          key="edit"
          title="编辑团队"
          trigger={<a>编辑</a>}
          onFinish={async (values) => {
            try {
              await updateTeam(record.id, values);
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
          <ProFormText name="name" label="团队名称" rules={[{ required: true }]} />
          <ProFormTextArea name="description" label="描述" />
        </ModalForm>,
        <Popconfirm
          key="delete"
          title="确定要删除吗？"
          onConfirm={async () => {
            try {
              await deleteTeam(record.id);
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
    <>
      <ProTable
        columns={columns}
        actionRef={actionRef}
        cardBordered
        request={async (params) => {
          try {
            const res = await getTeams(params);
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
        headerTitle="团队列表"
        toolBarRender={() => [
          <ModalForm
            key="create"
            title="新建团队"
            trigger={
              <Button type="primary">
                <PlusOutlined />
                新建
              </Button>
            }
            onFinish={async (values) => {
              try {
                await addTeam(values);
                message.success('添加成功');
                actionRef.current?.reload();
                return true;
              } catch (error) {
                message.error('添加失败');
                return false;
              }
            }}
          >
            <ProFormText name="name" label="团队名称" rules={[{ required: true }]} />
            <ProFormTextArea name="description" label="描述" />
          </ModalForm>,
        ]}
      />

      <Drawer
        title={`成员管理 - ${currentTeam?.name}`}
        width={600}
        open={memberDrawerVisible}
        onClose={() => setMemberDrawerVisible(false)}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <ModalForm
             title="添加成员"
             trigger={<Button type="dashed" block icon={<PlusOutlined />}>添加成员</Button>}
             onFinish={async (values) => {
               await handleAddMember(values.user_id);
               return true;
             }}
          >
             <ProFormSelect
                name="user_id"
                label="选择用户"
                options={users.map(u => ({ label: `${u.username} (${u.email || '-'})`, value: u.id }))}
                rules={[{ required: true }]}
             />
          </ModalForm>

          <List
            itemLayout="horizontal"
            dataSource={currentTeam?.users || []}
            renderItem={(user: any) => (
              <List.Item
                actions={[
                  <Button 
                    type="text" 
                    danger 
                    icon={<DeleteOutlined />} 
                    onClick={() => handleRemoveMember(user.id)}
                  >
                    移除
                  </Button>
                ]}
              >
                <List.Item.Meta
                  avatar={<Avatar icon={<UserOutlined />} />}
                  title={user.username}
                  description={user.email}
                />
              </List.Item>
            )}
          />
        </Space>
      </Drawer>
    </>
  );
};

export default TeamList;
