import React, { useState, useRef, useEffect } from 'react';
import { Button, Card, Input, List, Avatar, Space, Typography, FloatButton, Drawer } from 'antd';
import { RobotOutlined, UserOutlined, SendOutlined, CloseOutlined } from '@ant-design/icons';

const { Text } = Typography;

interface Message {
  id: string;
  sender: 'user' | 'ai';
  content: string;
  timestamp: Date;
}

const AIAssistant: React.FC = () => {
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const [isTyping, setIsTyping] = useState(false);
  const [messages, setMessages] = useState<Message[]>([
    {
      id: '1',
      sender: 'ai',
      content: 'Hello! I am Argus AI. How can I assist you with system diagnostics today?',
      timestamp: new Date(),
    },
  ]);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(scrollToBottom, [messages, isOpen]);

  const handleSendMessage = () => {
    if (!inputValue.trim()) return;

    const newMessage: Message = {
      id: Date.now().toString(),
      sender: 'user',
      content: inputValue,
      timestamp: new Date(),
    };

    setMessages([...messages, newMessage]);
    setInputValue('');
    setIsTyping(true);

    // Mock AI response
    setTimeout(() => {
      const aiResponse: Message = {
        id: (Date.now() + 1).toString(),
        sender: 'ai',
        content: generateAIResponse(newMessage.content),
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, aiResponse]);
      setIsTyping(false);
    }, 1500);
  };

  const generateAIResponse = (userMessage: string): string => {
    const lowerMsg = userMessage.toLowerCase();
    if (lowerMsg.includes('error') || lowerMsg.includes('fail')) {
      return "I've detected some error patterns. Checking the logs for related anomalies...";
    }
    if (lowerMsg.includes('cpu') || lowerMsg.includes('memory')) {
      return "Analyzing resource utilization. There seems to be a spike around 10:00 AM.";
    }
    if (lowerMsg.includes('database') || lowerMsg.includes('sql')) {
      return "Database latency is slightly elevated. I recommend checking slow query logs.";
    }
    if (lowerMsg.includes('help')) {
      return "I can help you diagnose system issues, analyze logs, and suggest code fixes.";
    }
    return "I'm processing your request. Could you provide more details?";
  };

  return (
    <>
      <FloatButton 
        icon={<RobotOutlined />} 
        type="primary" 
        style={{ right: 24, bottom: 24, width: 60, height: 60 }}
        onClick={() => setIsOpen(true)}
        tooltip="Argus AI Assistant"
      />
      
      <Drawer
        title={
          <Space>
            <RobotOutlined style={{ color: '#1890ff' }} />
            <Text strong>Argus AI Assistant</Text>
          </Space>
        }
        placement="right"
        onClose={() => setIsOpen(false)}
        open={isOpen}
        width={400}
        styles={{ body: { padding: 0, display: 'flex', flexDirection: 'column' } }}
        extra={
          <Button type="text" icon={<CloseOutlined />} onClick={() => setIsOpen(false)} />
        }
      >
        <div style={{ flex: 1, overflowY: 'auto', padding: '16px', background: '#f0f2f5' }}>
          <List
            dataSource={messages}
            split={false}
            renderItem={(item) => (
              <List.Item style={{ padding: '8px 0', justifyContent: item.sender === 'user' ? 'flex-end' : 'flex-start' }}>
                <div 
                  style={{ 
                    display: 'flex', 
                    flexDirection: item.sender === 'user' ? 'row-reverse' : 'row',
                    maxWidth: '85%' 
                  }}
                >
                  <Avatar 
                    icon={item.sender === 'user' ? <UserOutlined /> : <RobotOutlined />} 
                    style={{ 
                      backgroundColor: item.sender === 'user' ? '#87d068' : '#1890ff',
                      marginLeft: item.sender === 'user' ? 8 : 0,
                      marginRight: item.sender === 'ai' ? 8 : 0,
                    }} 
                  />
                  <Card 
                    size="small" 
                    bodyStyle={{ padding: '8px 12px' }}
                    style={{ 
                      borderRadius: 16, 
                      borderTopRightRadius: item.sender === 'user' ? 4 : 16,
                      borderTopLeftRadius: item.sender === 'ai' ? 4 : 16,
                      background: item.sender === 'user' ? '#e6f7ff' : '#fff'
                    }}
                  >
                    <Text>{item.content}</Text>
                    <div style={{ fontSize: 10, color: '#999', marginTop: 4, textAlign: 'right' }}>
                      {item.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </div>
                  </Card>
                </div>
              </List.Item>
            )}
          />
          {isTyping && (
            <div style={{ padding: '8px 0', display: 'flex', alignItems: 'center' }}>
              <Avatar icon={<RobotOutlined />} size="small" style={{ backgroundColor: '#1890ff', marginRight: 8 }} />
              <Text type="secondary" style={{ fontStyle: 'italic', fontSize: 12 }}>Argus AI is typing...</Text>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>
        
        <div style={{ padding: '16px', borderTop: '1px solid #f0f0f0', background: '#fff' }}>
          <Space.Compact style={{ width: '100%' }}>
            <Input 
              placeholder="Type a message..." 
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              onPressEnter={handleSendMessage}
              disabled={isTyping}
            />
            <Button type="primary" icon={<SendOutlined />} onClick={handleSendMessage} disabled={isTyping || !inputValue.trim()} />
          </Space.Compact>
        </div>
      </Drawer>
    </>
  );
};

export default AIAssistant;
