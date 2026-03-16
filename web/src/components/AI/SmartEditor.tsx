import React, { useRef, useState } from 'react';
import Editor, { type OnMount } from '@monaco-editor/react';
import { Card, Select, Button, Space, Tooltip } from 'antd';
import { BulbOutlined, CodeOutlined } from '@ant-design/icons';

const { Option } = Select;

const SmartEditor: React.FC = () => {
  const editorRef = useRef<any>(null);
  const [language, setLanguage] = useState('javascript');
  const [aiSuggestionsEnabled, setAiSuggestionsEnabled] = useState(true);

  const handleEditorDidMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;

    // Register a completion item provider for JavaScript
    monaco.languages.registerCompletionItemProvider('javascript', {
      provideCompletionItems: (model: any, position: any) => {
        if (!aiSuggestionsEnabled) return { suggestions: [] };
        
        const word = model.getWordUntilPosition(position);
        const range = {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        };

        return {
          suggestions: [
            {
              label: 'ai-log-error',
              kind: monaco.languages.CompletionItemKind.Snippet,
              documentation: 'AI Suggested: Standard Error Logging Pattern',
              insertText: 'console.error("[Error] " + ${1:error}.message);',
              insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
              range: range,
              detail: 'AI Suggestion',
            },
            {
              label: 'ai-fetch-data',
              kind: monaco.languages.CompletionItemKind.Snippet,
              documentation: 'AI Suggested: Fetch Data with Error Handling',
              insertText: [
                'async function fetchData(url) {',
                '\ttry {',
                '\t\tconst response = await fetch(url);',
                '\t\tif (!response.ok) throw new Error(response.statusText);',
                '\t\treturn await response.json();',
                '\t} catch (error) {',
                '\t\tconsole.error("Fetch error:", error);',
                '\t\treturn null;',
                '\t}',
                '}'
              ].join('\n'),
              insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
              range: range,
              detail: 'AI Suggestion',
            }
          ],
        };
      },
    });
  };

  return (
    <Card 
      title={<Space><CodeOutlined /> Smart Monaco Editor</Space>} 
      extra={
        <Space>
          <Tooltip title="Toggle AI Suggestions">
            <Button 
              type={aiSuggestionsEnabled ? 'primary' : 'default'} 
              icon={<BulbOutlined />} 
              onClick={() => setAiSuggestionsEnabled(!aiSuggestionsEnabled)}
            >
              AI Suggestions: {aiSuggestionsEnabled ? 'ON' : 'OFF'}
            </Button>
          </Tooltip>
          <Select defaultValue="javascript" style={{ width: 120 }} onChange={setLanguage}>
            <Option value="javascript">JavaScript</Option>
            <Option value="typescript">TypeScript</Option>
            <Option value="python">Python</Option>
          </Select>
        </Space>
      }
      style={{ width: '100%', height: 500 }}
      bodyStyle={{ padding: 0, height: 'calc(100% - 57px)' }}
    >
      <Editor
        height="100%"
        defaultLanguage="javascript"
        language={language}
        defaultValue="// Type 'ai-' to see AI suggestions..."
        onMount={handleEditorDidMount}
        theme="vs-dark"
        options={{
          minimap: { enabled: false },
          fontSize: 14,
          automaticLayout: true,
        }}
      />
    </Card>
  );
};

export default SmartEditor;
